package overlay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{APIURL: "http://127.0.0.1:11211", Username: "test", Password: "not-a-real-secret", ConfigServerURL: "tcp://127.0.0.1:22020/test"}
}
func TestDefaultsAndValidation(t *testing.T) {
	c := testConfig()
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
	n := Network{ID: UUID(), Spec: Spec{Name: "default"}}
	if e := n.Spec.Validate(); e != nil {
		t.Fatal(e)
	}
	m := Member{InstanceID: UUID()}
	v := EngineConfig(n, m, "secret")
	if v["dhcp"] != true || v["enable_manual_routes"] != true || !reflect.DeepEqual(v["routes"], []string{}) || !reflect.DeepEqual(v["listener_urls"], []string{"tcp://0.0.0.0:11010", "udp://0.0.0.0:11010"}) || v["bind_device"] != true || v["multi_thread"] != true {
		t.Fatalf("defaults: %#v", v)
	}
	m.VirtualIP = "10.144.144.2"
	if EngineConfig(n, m, "s")["dhcp"] != false {
		t.Fatal("explicit static")
	}
	for _, url := range []string{"http://localhost:11211", "http://192.168.1.2:11211", "http://127.0.0.1:11211@evil.com", "http://127.0.0.1:11211/?x=y"} {
		bad := testConfig()
		bad.APIURL = url
		if bad.Validate() == nil {
			t.Fatal(url)
		}
	}
	for _, dir := range []string{"/tmp/root", "/tmp/../etc", "/tmp/a b", "/tmp/a;b", "/tmp/中文"} {
		if ValidDirectory(dir) {
			t.Fatal(dir)
		}
	}
	for _, action := range []string{"stop", "exec", "shell"} {
		if (AgentRequest{Action: action, Directory: c.InstallDirectory, MachineID: UUID()}).Validate() == nil {
			t.Fatal(action)
		}
	}
	file := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(file, []byte(`{} {}`), 0600)
	if _, e := LoadConfig(file); e == nil {
		t.Fatal("trailing json")
	}
}
func TestOfficialWebCookiePathsAndNoAmbiguousReplay(t *testing.T) {
	machine, instance := UUID(), UUID()
	var mu sync.Mutex
	var paths []string
	ambiguous := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/api/v1/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "id", Value: "test", Path: "/"})
			w.WriteHeader(200)
			return
		}
		if _, e := r.Cookie("id"); e != nil {
			t.Error("missing session cookie")
			w.WriteHeader(401)
			return
		}
		if r.Method == "PUT" {
			if ambiguous {
				w.WriteHeader(500)
				w.Write([]byte("secret internal error"))
				return
			}
			var value map[string]json.RawMessage
			if json.NewDecoder(r.Body).Decode(&value) != nil {
				t.Error("invalid json")
			}
			if strings.Contains(r.URL.Path, "/config/") && value["config"] == nil {
				t.Error("config envelope")
			}
			w.WriteHeader(200)
			return
		}
		if r.Method == "GET" {
			w.Write([]byte(`{"info":{"map":{"` + instance + `":{"running":true,"my_node_info":{"peer_id":1,"virtual_ipv4":{"address":{"addr":176197633}}},"peers":[],"routes":[]}}}}`))
			return
		}
	}))
	defer server.Close()
	c := testConfig()
	c.APIURL = server.URL
	web, e := NewWebClient(c)
	if e != nil {
		t.Fatal(e)
	}
	defer web.Close()
	if e = web.Apply(context.Background(), machine, instance, map[string]any{"instance_id": instance}); e != nil {
		t.Fatal(e)
	}
	expected := []string{"POST /api/v1/auth/login", "PUT " + machinePath(machine) + "/config/" + instance, "PUT " + machinePath(machine) + "/" + instance}
	if !reflect.DeepEqual(paths, expected) {
		t.Fatal(paths)
	}
	got, e := web.Collect(context.Background(), machine, instance)
	if e != nil || !got.Running || got.MyNode.IP.Address != "10.128.144.1" {
		t.Fatalf("fixture decode %#v %v", got, e)
	}
	mu.Lock()
	ambiguous = true
	before := len(paths)
	mu.Unlock()
	if e = web.Apply(context.Background(), machine, instance, map[string]any{}); !errors.Is(e, ErrUncertain) || strings.Contains(e.Error(), "secret") {
		t.Fatal(e)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != before+1 {
		t.Fatal("replayed ambiguous mutation", paths)
	}
}

type fakeDriver struct {
	online  bool
	settled bool
	err     error
	boot    int
	mu      sync.Mutex
}

func (f *fakeDriver) ValidateDevice(string) error {
	if !f.online {
		return ErrConflict
	}
	return nil
}
func (f *fakeDriver) Online(string) bool         { return f.online }
func (f *fakeDriver) TasksSettled([]string) bool { return f.settled }
func (f *fakeDriver) Bootstrap(ctx context.Context, m Member, report Reporter) error {
	f.mu.Lock()
	f.boot++
	f.mu.Unlock()
	if e := report("bootstrap", "original-task"); e != nil {
		return e
	}
	return f.err
}

type fakeController struct {
	mu             sync.Mutex
	running        bool
	applies, stops int
	err            error
	confirmed      bool
}

func (f *fakeController) Apply(context.Context, string, string, map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applies++
	f.running = true
	return f.err
}
func (f *fakeController) Stop(context.Context, string, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	f.running = false
	return f.err
}
func (f *fakeController) Collect(context.Context, string, string) (Running, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return Running{Running: f.running}, nil
}
func (f *fakeController) Confirm(context.Context, string, string, map[string]any) error {
	if !f.confirmed {
		return ErrUncertain
	}
	return nil
}
func awaitOp(t *testing.T, s *Service, id string) Operation {
	t.Helper()
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end); {
		for _, op := range s.Operations("") {
			if op.ID == id && op.State != "running" && op.State != "queued" {
				return op
			}
		}
		time.Sleep(time.Millisecond * 5)
	}
	t.Fatal("operation timed out")
	return Operation{}
}
func TestPersistedLifecycleDisconnectAndIdentity(t *testing.T) {
	file := filepath.Join(t.TempDir(), "networks.json")
	d := &fakeDriver{online: true, settled: true}
	up := &fakeController{confirmed: true}
	s, e := Open(file, testConfig(), d, up)
	if e != nil {
		t.Fatal(e)
	}
	n, e := s.Create(Spec{Name: "branch network"})
	if e != nil {
		t.Fatal(e)
	}
	op, e := s.Start(n.ID, JoinRequest{DeviceID: "router-a"}, "start")
	if e != nil {
		t.Fatal(e)
	}
	if got := awaitOp(t, s, op.ID); got.State != "succeeded" {
		t.Fatal(got)
	}
	n, _ = s.Get(n.ID)
	machine := n.Members[0].MachineID
	b, _ := json.Marshal(s.List())
	if strings.Contains(string(b), "secret") {
		t.Fatal("public secret leak")
	}
	s.Close()
	up.mu.Lock()
	if up.stops != 0 || !up.running {
		t.Fatal("management disconnect tore down network")
	}
	up.mu.Unlock()
	s, e = Open(file, testConfig(), d, up)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if up.applies != 1 {
		t.Fatal("restart auto applied")
	}
	n, _ = s.Get(n.ID)
	if n.Members[0].MachineID != machine {
		t.Fatal("identity changed")
	}
	op, e = s.Start(n.ID, JoinRequest{DeviceID: "router-a"}, "stop")
	if e != nil {
		t.Fatal(e)
	}
	if awaitOp(t, s, op.ID).State != "succeeded" {
		t.Fatal("stop")
	}
	if e = s.RemoveMember(n.ID, "router-a"); e != nil {
		t.Fatal(e)
	}
	op, e = s.Start(n.ID, JoinRequest{DeviceID: "router-a"}, "start")
	if e != nil {
		t.Fatal(e)
	}
	awaitOp(t, s, op.ID)
	n, _ = s.Get(n.ID)
	if n.Members[0].MachineID != machine {
		t.Fatal("rejoin changed bootstrap machine")
	}
}
func TestUncertainCannotReplayOrReconcilePendingTask(t *testing.T) {
	d := &fakeDriver{online: true, settled: false}
	up := &fakeController{err: ErrUncertain, confirmed: true}
	s, e := Open(filepath.Join(t.TempDir(), "n.json"), testConfig(), d, up)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	n, _ := s.Create(Spec{Name: "test"})
	op, _ := s.Start(n.ID, JoinRequest{DeviceID: "a"}, "start")
	got := awaitOp(t, s, op.ID)
	if got.State != "uncertain" || !reflect.DeepEqual(got.TaskIDs, []string{"original-task"}) {
		t.Fatal(got)
	}
	if _, e = s.Start(n.ID, JoinRequest{DeviceID: "a"}, "start"); !errors.Is(e, ErrConflict) {
		t.Fatal("ambiguous retry", e)
	}
	if _, e = s.Reconcile(context.Background(), op.ID); !errors.Is(e, ErrUncertain) {
		t.Fatal("pending task released", e)
	}
	d.settled = true
	if got, e = s.Reconcile(context.Background(), op.ID); e != nil || got.State != "reconciled" {
		t.Fatal(got, e)
	}
	if up.applies != 1 {
		t.Fatal("reconcile wrote")
	}
}
func TestTopologyUnknownStaleAndActualLinks(t *testing.T) {
	now := time.Now()
	n := Network{Members: []Member{{DeviceID: "a"}, {DeviceID: "b"}}}
	a := Observation{DeviceID: "a", PeerID: 1, State: "running", SampledAt: now, Links: []Link{{PeerID: 2, Transport: "udp"}}}
	b := Observation{DeviceID: "b", PeerID: 2, State: "running", SampledAt: now, Links: []Link{{PeerID: 1, Transport: "udp"}}}
	v := topology(n, map[string]Observation{"a": a}, func(string) bool { return false }, now)
	if len(v.Edges) != 1 || v.Edges[0].ConfirmedBoth {
		t.Fatal(v)
	}
	v = topology(n, map[string]Observation{"a": a, "b": b}, func(string) bool { return true }, now)
	if len(v.Edges) != 2 || !v.Edges[0].ConfirmedBoth {
		t.Fatal(v)
	}
	v = topology(n, map[string]Observation{"a": a, "b": b}, func(string) bool { return true }, now.Add(time.Minute))
	for _, edge := range v.Edges {
		if !edge.Stale || edge.ConfirmedBoth {
			t.Fatal(edge)
		}
	}
	var c Counter
	if json.Unmarshal([]byte(`"18446744073709551615"`), &c) != nil || uint64(c) != ^uint64(0) {
		t.Fatal("uint64 precision")
	}
	if json.Unmarshal([]byte(`-1`), &c) == nil {
		t.Fatal("negative counter")
	}
}
