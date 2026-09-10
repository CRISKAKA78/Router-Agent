package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"routerprobe/internal/api"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/probetemplate"
	"routerprobe/internal/task"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNativeNeighborDiscovery(t *testing.T) {
	binary := probeBinary(t)
	suffix := fmt.Sprintf("%x", time.Now().UnixNano()&0xffffff)
	bridge, port, peer := "nb"+suffix, "np"+suffix, "nv"+suffix
	ip := func(args ...string) {
		t.Helper()
		if out, e := exec.Command("ip", args...).CombinedOutput(); e != nil {
			t.Fatalf("ip %v: %v %s", args, e, out)
		}
	}
	ip("link", "add", bridge, "type", "bridge")
	defer exec.Command("ip", "link", "del", bridge).Run()
	before, _ := net.Interfaces()
	existing := map[int]bool{}
	for _, i := range before {
		existing[i.Index] = true
	}
	ip("link", "add", port, "type", "veth")
	after, _ := net.Interfaces()
	actualPeer := ""
	for _, i := range after {
		if !existing[i.Index] && i.Name != port {
			actualPeer = i.Name
		}
	}
	if actualPeer == "" {
		t.Fatal("veth peer not found")
	}
	ip("link", "set", actualPeer, "name", peer)
	defer exec.Command("ip", "link", "del", port).Run()
	ip("link", "set", port, "master", bridge)
	ip("addr", "add", "192.0.2.1/24", "dev", bridge)
	peerProcess := exec.Command("unshare", "-n", "sleep", "120")
	if e := peerProcess.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { peerProcess.Process.Kill(); peerProcess.Wait() }()
	pid := strconv.Itoa(peerProcess.Process.Pid)
	parentNS, _ := os.Readlink("/proc/self/ns/net")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		childNS, _ := os.Readlink("/proc/" + pid + "/ns/net")
		if childNS != "" && childNS != parentNS {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	peerInterface, e := net.InterfaceByName(peer)
	if e != nil {
		t.Fatal(e)
	}
	ip("link", "set", peer, "netns", pid)
	for _, args := range [][]string{{"addr", "add", "192.0.2.2/24", "dev", peer}, {"link", "set", peer, "up"}} {
		command := append([]string{"-t", pid, "-n", "ip"}, args...)
		if out, e := exec.Command("nsenter", command...).CombinedOutput(); e != nil {
			t.Fatalf("peer setup: %v %s", e, out)
		}
	}
	for _, name := range []string{bridge, port} {
		ip("link", "set", name, "up")
	}
	app, e := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{Logger: log.New(io.Discard, "", 0)}})
	if e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(listener) }()
	defer func() { app.Close(); <-done }()
	lease := filepath.Join(t.TempDir(), "leases")
	if e := os.WriteFile(lease, []byte("0 02:00:00:00:00:09 192.0.2.9 lease-only *\n0 02:00:00:00:00:08 198.51.100.9 wrong-subnet *\n"), 0600); e != nil {
		t.Fatal(e)
	}
	tpl, e := app.ProbeTemplates().Put("", 0, probetemplate.Input{Name: "native neighbors", Properties: map[string]probetemplate.Property{}, Monitoring: &probetemplate.Monitoring{}, NeighborProbe: &probetemplate.NeighborProbe{Interval: 10, Domains: []probetemplate.NeighborDomain{{ID: "lan", Scope: "lan", Interface: bridge, Ports: []string{port}, LeaseFile: lease}, {ID: "local", Scope: "broadcast", Interface: bridge, LeaseFile: lease}}}})
	if e != nil {
		t.Fatal(e)
	}
	startProbe(t, binary, listener.Addr().String(), "neighbors", io.Discard)
	adoptProbe(t, app, "neighbors", tpl.ID, nil)
	var revision uint64
	until := time.Now().Add(10 * time.Second)
	for time.Now().Before(until) {
		d, _ := app.Devices().Get("neighbors")
		revision = d.LatestSession.ConfigRevision
		if revision > 0 {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if revision == 0 {
		t.Fatal("configuration not applied")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	adapter, e := api.New(app, api.Config{})
	if e != nil {
		t.Fatal(e)
	}
	defer adapter.Close()
	httpServer := httptest.NewServer(adapter)
	defer httpServer.Close()
	scanHTTP := func() string {
		t.Helper()
		body := fmt.Sprintf(`{"domain_id":"local","cidr":"192.0.2.0/30","config_revision":%d}`, revision)
		request, _ := http.NewRequestWithContext(ctx, "POST", httpServer.URL+"/api/v1/devices/neighbors/neighbor-scans", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "native-neighbor-scan")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var envelope struct {
			Data struct {
				ID string `json:"task_id"`
			}
		}
		if response.StatusCode != 202 || json.NewDecoder(response.Body).Decode(&envelope) != nil || envelope.Data.ID == "" {
			t.Fatalf("invalid scan response %d", response.StatusCode)
		}
		return envelope.Data.ID
	}
	id := scanHTTP()
	if scanHTTP() != id {
		t.Fatal("retry created a replacement scan")
	}
	result, e := app.WaitTaskResult(ctx, id)
	if e != nil || result.Status != "success" {
		t.Fatalf("scan: %+v %v", result, e)
	}
	until = time.Now().Add(10 * time.Second)
	found := false
	for time.Now().Before(until) {
		n, e := app.Neighbors("neighbors")
		if e == nil && n != nil && len(n.Domains) == 2 {
			counts := 0
			for _, d := range n.Domains {
				for _, r := range d.Rows {
					if r.IP == "192.0.2.2" && r.Port == port {
						counts++
						break
					}
				}
			}
			if counts == 2 {
				found = true
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
	if found {
		n, _ := app.Neighbors("neighbors")
		leaseFound := false
		for _, r := range n.Domains[1].Rows {
			if r.IP == "192.0.2.9" && r.Hostname == "lease-only" && r.State == "lease" {
				leaseFound = true
			}
			if r.IP == "198.51.100.9" {
				t.Fatal("foreign lease leaked")
			}
		}
		if !leaseFound {
			t.Fatal("local lease missing")
		}
	}
	if !found {
		n, _ := app.Neighbors("neighbors")
		t.Fatalf("real ARP/FDB views did not overlap: result=%+v snapshot=%+v", result, n)
	}
	id, e = app.CreateNeighbor(ctx, "neighbors", task.NeighborRequest{DomainID: "local", CIDR: "192.0.2.0/24", Revision: revision}, false)
	if e != nil {
		t.Fatal(e)
	}
	stop, e := app.CreateNeighbor(ctx, "neighbors", task.NeighborRequest{TargetTaskID: id}, true)
	if e != nil {
		t.Fatal(e)
	}
	stopped, e := app.WaitTaskResult(ctx, stop)
	if e != nil || stopped.Status != "success" {
		t.Fatalf("stop: %+v %v", stopped, e)
	}
	result, e = app.WaitTaskResult(ctx, id)
	if e != nil || result.Status != "failed" || !strings.Contains(result.Stderr, "cancelled") {
		t.Fatalf("cancelled scan: %+v %v", result, e)
	}
	id, e = app.CreateNeighbor(ctx, "neighbors", task.NeighborRequest{DomainID: "local", CIDR: "198.51.100.0/24", Revision: revision}, false)
	if e != nil {
		t.Fatal(e)
	}
	result, e = app.WaitTaskResult(ctx, id)
	if e != nil || result.Status != "failed" || result.Stderr != "range_not_on_link" {
		t.Fatalf("off-link scan: %+v %v", result, e)
	}
	if _, e = app.CreateNeighbor(ctx, "neighbors", task.NeighborRequest{DomainID: "local", CIDR: "192.0.0.0/16", Revision: revision}, false); e == nil {
		t.Fatal("unbounded scan accepted")
	}
	// Conflicting vendor port evidence must never choose the last row as LAN.
	plan := tpl.NeighborProbe
	for _, domain := range plan.Domains {
		plan.FDBCommand += fmt.Sprintf("printf '%s\\t%s\\t%s\\n%s\\t%s\\tother\\n';", domain.ID, peerInterface.HardwareAddr.String(), port, domain.ID, peerInterface.HardwareAddr.String())
	}
	tpl, e = app.ProbeTemplates().Put(tpl.ID, tpl.Version, probetemplate.Input{Name: tpl.Name, Properties: tpl.Properties, Monitoring: tpl.Monitoring, NeighborProbe: plan})
	if e != nil {
		t.Fatal(e)
	}
	adoptProbe(t, app, "neighbors", tpl.ID, nil)
	until = time.Now().Add(10 * time.Second)
	for time.Now().Before(until) {
		n, _ := app.Neighbors("neighbors")
		if n != nil && n.Revision > revision {
			for _, r := range n.Domains[0].Rows {
				if r.MAC == peerInterface.HardwareAddr.String() {
					t.Fatal("conflicting port classified as LAN")
				}
			}
			for _, r := range n.Unclassified {
				if r.MAC == peerInterface.HardwareAddr.String() && r.Port == "" {
					return
				}
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("conflicting port evidence was not kept unclassified")

}
