package overlay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

func TestMachineIDOfficialRustVectors(t *testing.T) {
	cases := map[string]string{"FE7140555489": "42a73850-8c63-39ad-a92a-63eec54941f6", "FJB130161591": "e5b55f38-0434-bee1-2e58-cc8adf7fd053", "router-a": "3f1f1e54-4cf1-b04c-6e39-c5b88acf18f6", " abc ": "b1eab65d-1ed0-05ad-d279-c6e62f21d01c", "中文设备": "21603d0f-7d68-42ed-c7a2-e55905f07b21", "0123456789abcdef0123456789abcdef0123456789": "cdfa27cd-c461-7637-dd15-b736ee9ce0bd"}
	for input, want := range cases {
		if got := MachineID(input); got != want {
			t.Fatalf("%q got %s want %s", input, got, want)
		}
	}
	id := UUID()
	if MachineID(" "+strings.ToUpper(id)+" ") != id {
		t.Fatal("UUID not preserved")
	}
}
func TestConfiguredDefaultsPasswordAndLegacyPreservation(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "networks.json"), testConfig(), &fakeDriver{online: true, settled: true}, &fakeController{confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	n, e := s.CreateConfigured(Spec{Name: "new"}, "chosen-password")
	if e != nil {
		t.Fatal(e)
	}
	if n.Profile != 2 || n.MTU != 1380 || len(n.PeerURLs) != 2 || n.PeerURLs[0] != "tcp://47.119.168.150:11010" {
		t.Fatal(n)
	}
	if p, e := s.Password(n.ID); e != nil || p != "chosen-password" {
		t.Fatal(p, e)
	}
	raw, _ := json.Marshal(s.List())
	if strings.Contains(string(raw), "chosen-password") || strings.Contains(string(raw), "secret") {
		t.Fatal("password leaked")
	}
	m := Member{DeviceID: "a", InstanceID: UUID()}
	c := EngineConfig(n, m, "secret")
	for key, want := range map[string]any{"multi_thread": false, "dev_name": "et0", "enable_private_mode": true, "bind_device": true, "proxy_forward_by_system": true, "disable_sym_hole_punching": true, "disable_upnp": true, "mtu": 1380, "dhcp": true, "enable_manual_routes": true} {
		if c[key] != want {
			t.Errorf("%s: %v", key, c[key])
		}
	}
	legacy, _ := s.Create(Spec{Name: "legacy", Routes: []string{"192.168.20.0/24"}})
	old := EngineConfig(legacy, m, "s")
	if old["multi_thread"] != true || old["mtu"] != nil || old["enable_private_mode"] != nil {
		t.Fatal("legacy default rewritten", old)
	}
	q := Spec{Name: "prefix", PeerURLs: []string{"tcp://et.criskaka.com"}}
	if q.Validate() != nil || q.PeerURLs[0] != "tcp://et.criskaka.com:11010" {
		t.Fatal(q)
	}
	for _, url := range []string{"et.criskaka.com:11010", "tcp://user:pass@host:11010", "tcp://host:11010?password=a"} {
		q.PeerURLs = []string{url}
		if q.Validate() == nil {
			t.Fatal("invalid URL", url)
		}
	}
}

type keyedController struct {
	mu             sync.Mutex
	configs        map[string]map[string]any
	running        map[string]bool
	applies, stops map[string]int
	badConfirm     map[string]bool
}

func newKeyedController() *keyedController {
	return &keyedController{configs: map[string]map[string]any{}, running: map[string]bool{}, applies: map[string]int{}, stops: map[string]int{}, badConfirm: map[string]bool{}}
}
func (c *keyedController) Apply(_ context.Context, _, id string, cfg map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applies[id]++
	c.configs[id] = clone(cfg)
	c.running[id] = true
	return nil
}
func (c *keyedController) Stop(_ context.Context, _, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stops[id]++
	c.running[id] = false
	return nil
}
func (c *keyedController) Collect(_ context.Context, _, id string) (Running, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := Running{Running: c.running[id]}
	if cfg, ok := c.configs[id]; ok {
		ip, _ := cfg["virtual_ipv4"].(string)
		if ip == "" {
			ip = "10.144.144.22"
		}
		r.MyNode.IP.Address = ip
		return r, nil
	}
	return r, ErrNotFound
}
func (c *keyedController) Confirm(_ context.Context, _, id string, cfg map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.badConfirm[id] || !valuesEqual(c.configs[id], cfg) {
		return ErrUncertain
	}
	return nil
}
func TestBatchAnchorMemberRevisionStopAndRecovery(t *testing.T) {
	up := newKeyedController()
	d := &fakeDriver{online: true, settled: true}
	s, e := Open(filepath.Join(t.TempDir(), "networks.json"), testConfig(), d, up)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	n, _ := s.CreateConfigured(Spec{Name: "batch"}, "password")
	if _, e = s.JoinBatch(n.ID, []JoinRequest{{DeviceID: "dhcp"}}); !errors.Is(e, ErrConflict) {
		t.Fatal("DHCP without anchor", e)
	}
	results, e := s.JoinBatch(n.ID, []JoinRequest{{DeviceID: "dhcp"}, {DeviceID: "static", VirtualIP: "10.144.144.1"}})
	if e != nil || len(results) != 2 {
		t.Fatal(results, e)
	}
	for _, r := range results {
		if r.Operation == nil {
			t.Fatal(r)
		}
		if op := awaitOp(t, s, r.Operation.ID); op.State != "succeeded" {
			t.Fatal(op)
		}
	}
	n, _ = s.Get(n.ID)
	anchor := n.Members[0]
	dhcp := n.Members[1]
	if anchor.DeviceID != "static" || anchor.MachineID != MachineID("static") {
		t.Fatal(n)
	}
	cfg := *dhcp.Config
	cfg.SystemForward = false
	cfg.LazyP2P = true
	cfg.NeedP2P = true
	cfg.Routes = []string{"192.168.50.0/24"}
	cfg.ProxyCIDRs = []string{"192.168.60.0/24"}
	updated, e := s.ConfigureMember(n.ID, dhcp.DeviceID, cfg, dhcp.ConfigRevision)
	if e != nil {
		t.Fatal(e)
	}
	if op := awaitOp(t, s, updated.OperationID); op.State != "succeeded" {
		t.Fatal(op)
	}
	up.mu.Lock()
	if up.applies[anchor.InstanceID] != 1 || up.applies[dhcp.InstanceID] != 2 || up.stops[dhcp.InstanceID] != 1 {
		t.Fatal("not instance scoped", up.applies, up.stops)
	}
	up.mu.Unlock()
	if _, e = s.ConfigureMember(n.ID, dhcp.DeviceID, cfg, dhcp.ConfigRevision); !errors.Is(e, ErrConflict) {
		t.Fatal("stale revision", e)
	}
	op, e := s.Start(n.ID, JoinRequest{DeviceID: dhcp.DeviceID}, "stop")
	if e != nil {
		t.Fatal(e)
	}
	awaitOp(t, s, op.ID)
	cfg.ManualRoutes = false
	cfg.Routes = nil
	updated, e = s.ConfigureMember(n.ID, dhcp.DeviceID, cfg, updated.ConfigRevision)
	if e != nil {
		t.Fatal(e)
	}
	up.mu.Lock()
	if up.running[dhcp.InstanceID] || up.applies[dhcp.InstanceID] != 2 {
		t.Fatal("saved stopped member restarted")
	}
	up.running[anchor.InstanceID] = false
	delete(up.configs, anchor.InstanceID)
	up.mu.Unlock()
	s.maintain()
	ops := s.Operations(n.ID)
	var recovery Operation
	for _, o := range ops {
		if o.DeviceID == anchor.DeviceID && o.Recovery {
			recovery = o
			break
		}
	}
	if recovery.ID == "" {
		t.Fatal("missing recovery")
	}
	if op := awaitOp(t, s, recovery.ID); op.State != "succeeded" {
		t.Fatal(op)
	}
	current, _ := s.Get(n.ID)
	if current.Members[0].InstanceID != anchor.InstanceID || current.Members[0].MachineID != anchor.MachineID {
		t.Fatal("recovered as new identity")
	}
	up.mu.Lock()
	if up.running[dhcp.InstanceID] {
		t.Fatal("stopped peer restored")
	}
	count := up.applies[anchor.InstanceID]
	up.mu.Unlock()
	s.maintain()
	up.mu.Lock()
	if up.applies[anchor.InstanceID] != count {
		t.Fatal("healthy member reapplied")
	}
	up.mu.Unlock()
	op, e = s.Start(n.ID, JoinRequest{DeviceID: anchor.DeviceID}, "stop")
	if e != nil {
		t.Fatal(e)
	}
	awaitOp(t, s, op.ID)
	if e = s.RemoveMember(n.ID, anchor.DeviceID); !errors.Is(e, ErrConflict) {
		t.Fatal("last anchor removed while DHCP remains", e)
	}
	if e = s.RemoveMember(n.ID, dhcp.DeviceID); e != nil {
		t.Fatal(e)
	}
	if e = s.RemoveMember(n.ID, anchor.DeviceID); e != nil {
		t.Fatal(e)
	}
}

type historicalDriver struct {
	fakeDriver
	evidence string
}

func (d *historicalDriver) TaskEvidence([]string) string { return d.evidence }
func TestHistoricalTasksAndMemberIsolation(t *testing.T) {
	up := &fakeController{confirmed: true}
	d := &historicalDriver{fakeDriver: fakeDriver{online: true, settled: true}, evidence: "missing"}
	s, e := Open(filepath.Join(t.TempDir(), "networks.json"), testConfig(), d, up)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	n, _ := s.Create(Spec{Name: "legacy"})
	a, _ := s.Start(n.ID, JoinRequest{DeviceID: "a"}, "start")
	awaitOp(t, s, a.ID)
	b, _ := s.Start(n.ID, JoinRequest{DeviceID: "b"}, "start")
	awaitOp(t, s, b.ID)
	s.mu.Lock()
	data := clone(s.data)
	old := data.Operations[a.ID]
	old.State = "uncertain"
	old.Step = "verify_runtime"
	data.Operations[a.ID] = old
	if e = s.commit(data); e != nil {
		t.Fatal(e)
	}
	s.mu.Unlock()
	d.settled = false
	got, e := s.Reconcile(context.Background(), a.ID)
	if e != nil || got.State != "reconciled" || len(got.TaskIDs) != 1 || got.TaskIDs[0] != "original-task" {
		t.Fatal(got, e)
	}
	// Pending work still blocks reconciliation, but does not block another member's removal.
	s.mu.Lock()
	data = clone(s.data)
	old = data.Operations[a.ID]
	old.State = "uncertain"
	old.Step = "verify_runtime"
	data.Operations[a.ID] = old
	_ = s.commit(data)
	s.mu.Unlock()
	d.evidence = "pending"
	if _, e = s.Reconcile(context.Background(), a.ID); !errors.Is(e, ErrUncertain) {
		t.Fatal(e)
	}
	stopped, e := s.Start(n.ID, JoinRequest{DeviceID: "b"}, "stop")
	if e != nil {
		t.Fatal(e)
	}
	awaitOp(t, s, stopped.ID)
	if e = s.RemoveMember(n.ID, "b"); e != nil {
		t.Fatal("other member blocked removal", e)
	}
	d.evidence = "missing"
	stopped, e = s.Start(n.ID, JoinRequest{DeviceID: "a"}, "stop")
	if e != nil {
		t.Fatal("stop cannot supersede uncertain start", e)
	}
	awaitOp(t, s, stopped.ID)
	s.mu.Lock()
	old = s.data.Operations[a.ID]
	s.mu.Unlock()
	if old.State != "uncertain" || old.SupersededBy != stopped.ID {
		t.Fatal("history overwritten", old)
	}
	if e = s.report(a.ID, "late", ""); !errors.Is(e, ErrConflict) {
		t.Fatal("late report accepted", e)
	}
	if e = s.RemoveMember(n.ID, "a"); e != nil {
		t.Fatal(e)
	}
}
func TestAuthoritativeRawConfigAndRPC(t *testing.T) {
	id, machine := UUID(), UUID()
	raw := fmt.Sprintf("instance_id = %q\ndhcp = true\nroutes = []\nlisteners = [\"tcp://0.0.0.0:11010\",\"udp://0.0.0.0:11010\"]\n[network_identity]\nnetwork_name = \"network\"\nnetwork_secret = \"password\"\n[flags]\nbind_device = true\nmulti_thread = false\nprivate_mode = true\n", id)
	expected := map[string]any{"instance_id": id, "dhcp": true, "virtual_ipv4": "", "network_length": 24, "enable_manual_routes": true, "routes": []string{}, "network_name": "network", "network_secret": "password", "listener_urls": []string{"tcp://0.0.0.0:11010", "udp://0.0.0.0:11010"}, "bind_device": true, "multi_thread": false, "enable_private_mode": true}
	var config map[string]any
	if e := toml.Unmarshal([]byte(raw), &config); e != nil {
		t.Fatal(e)
	}
	if !rawConfigMatches(config, expected) {
		t.Fatal("raw config mismatch")
	}
	delete(config, "routes")
	if rawConfigMatches(config, expected) {
		t.Fatal("missing routes accepted as manual")
	}
	calls := 0
	wrongInstance := false
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/login" {
			w.Write([]byte(`{}`))
			return
		}
		if r.Method == "GET" {
			w.Write([]byte(`{"enable_manual_routes":null,"routes":[]}`))
			return
		}
		if r.URL.Path != "/api/v1/machines/"+machine+"/proxy-rpc" || r.Method != "POST" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(400)
			return
		}
		calls++
		var request map[string]any
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			t.Fatal("invalid RPC")
		}
		if request["method_name"] != "ShowNodeInfo" || request["service_name"] != "api.instance.PeerManageRpcService" {
			t.Error(request)
		}
		instance := request["payload"].(map[string]any)["instance"].(map[string]any)
		if len(instance["selector"].(map[string]any)["Id"].(map[string]any)) != 4 {
			t.Error("wrong UUID RPC")
		}
		actualID := id
		if wrongInstance {
			actualID = UUID()
		}
		json.NewEncoder(w).Encode(map[string]any{"node_info": map[string]any{"inst_id": actualID, "config": raw}})
	}))
	defer h.Close()
	cfg := testConfig()
	cfg.APIURL = h.URL
	web, _ := NewWebClient(cfg)
	defer web.Close()
	if e := web.Confirm(context.Background(), machine, id, expected); e != nil || calls != 1 {
		t.Fatal(e, calls)
	}
	wrongInstance = true
	if e := web.Confirm(context.Background(), machine, id, expected); !errors.Is(e, ErrUncertain) {
		t.Fatal("different runtime instance accepted", e)
	}
}
func TestAutomaticReadOnlyReconcile(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "networks.json"), testConfig(), &fakeDriver{online: true, settled: true}, &fakeController{confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	n, _ := s.Create(Spec{Name: "auto"})
	op, _ := s.Start(n.ID, JoinRequest{DeviceID: "a"}, "start")
	awaitOp(t, s, op.ID)
	s.mu.Lock()
	data := clone(s.data)
	op = data.Operations[op.ID]
	op.State = "uncertain"
	op.Step = "verify_configuration"
	op.UpdatedAt = time.Now()
	data.Operations[op.ID] = op
	_ = s.commit(data)
	s.mu.Unlock()
	s.maintain()
	s.mu.Lock()
	got := s.data.Operations[op.ID]
	s.mu.Unlock()
	if got.State != "reconciled" {
		t.Fatal(got)
	}
}
func TestRawDefaultsDoNotPretendSingleThread(t *testing.T) {
	raw := map[string]any{"flags": map[string]any{}}
	if !rawConfigMatches(raw, map[string]any{"mtu": 1380, "bind_device": true, "multi_thread": true}) {
		t.Fatal("official omitted defaults not restored")
	}
	if rawConfigMatches(raw, map[string]any{"multi_thread": false}) {
		t.Fatal("absent multi_thread incorrectly accepted as false")
	}
}
func TestRecoveryCannotOverrideNewStopIntent(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "networks.json"), testConfig(), &fakeDriver{online: true, settled: true}, &fakeController{confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	n, _ := s.Create(Spec{Name: "stop-wins"})
	op, _ := s.Start(n.ID, JoinRequest{DeviceID: "a"}, "start")
	awaitOp(t, s, op.ID)
	op, _ = s.Start(n.ID, JoinRequest{DeviceID: "a"}, "stop")
	awaitOp(t, s, op.ID)
	if _, e = s.start(n.ID, JoinRequest{DeviceID: "a"}, "start", true); !errors.Is(e, ErrConflict) {
		t.Fatal("stale recovery snapshot restarted stopped member", e)
	}
}
