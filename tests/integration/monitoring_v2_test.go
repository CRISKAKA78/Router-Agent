package integration

import (
	"io"
	"log"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"routerprobe/internal/api"
	"routerprobe/internal/device"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/probetemplate"
	"strconv"
	"testing"
	"time"
)

func TestMonitoringV2RealProbe(t *testing.T) {
	binary := probeBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nvram"), []byte("#!/bin/sh\nprintf 'FNR100 v1.1 (Jan  7 2026 11:51:01) std'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"curl", "wget"} {
		if err := os.WriteFile(filepath.Join(dir, tool), []byte("#!/bin/sh\nprintf invoked >> "+filepath.Join(dir, "external-http-called")+"\nprintf '8.8.8.8'\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	app, e := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{HeartbeatInterval: 10 * time.Second, Logger: log.New(io.Discard, "", 0)}})
	if e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(l) }()
	adapter, e := api.New(app, api.Config{})
	if e != nil {
		t.Fatal(e)
	}
	http := httptest.NewServer(adapter)
	defer func() { adapter.Close(); http.Close(); app.Close(); <-done }()
	names := "missing0"
	tpl, e := app.ProbeTemplates().Put("", 0, probetemplate.Input{Name: "filtered", Monitoring: &probetemplate.Monitoring{Network: 1, Egress: 1, NetworkInterfaces: &names}, Properties: map[string]probetemplate.Property{"x": {Name: "X", Command: "printf x"}}})
	if e != nil {
		t.Fatal(e)
	}
	startProbe(t, binary, l.Addr().String(), "v2", io.Discard)
	adoptProbe(t, app, "v2", tpl.ID, nil)
	wait := func(check func(device.Snapshot, map[string]device.Metric) bool) device.Snapshot {
		t.Helper()
		end := time.Now().Add(18 * time.Second)
		for time.Now().Before(end) {
			v, e := app.Devices().Get("v2")
			if e == nil && check(v, device.EffectiveMetrics(v.LatestSession, time.Now())) {
				return v
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatal("v2 telemetry deadline")
		return device.Snapshot{}
	}
	first := wait(func(v device.Snapshot, m map[string]device.Metric) bool {
		return m["egress_ipv4"].Status == "error" && m["egress_ipv4"].Reason != "" && m["egress_ipv6"].Status == "error" && m["egress_ipv6"].Reason != "" && len(v.LatestSession.Telemetry.Groups["network"]) == 7
	})
	if first.Registration.Model != "FNR100" || first.Registration.Firmware != "FNR100 v1.1 (Jan  7 2026 11:51:01) std" {
		t.Fatal(first.Registration)
	}
	for _, m := range first.LatestSession.Telemetry.Groups["network"] {
		if m.Entity != "missing0" || m.Reason != "interface_missing" {
			t.Fatal(m)
		}
	}
	wire := p5request(t, http.URL, "GET", "/devices/v2", "", nil, 200)
	metrics := wire["effective_metrics"].(map[string]any)
	if wire["source_ip"] != "127.0.0.1" || metrics["egress_ipv4"].(map[string]any)["status"] != "error" {
		t.Fatal(wire)
	}
	if _, err := os.Stat(filepath.Join(dir, "external-http-called")); !os.IsNotExist(err) {
		t.Fatal("native egress executed an external HTTP tool", err)
	}
	// A template application, not an arbitrary device override, disables egress.
	tpl, e = app.ProbeTemplates().Put(tpl.ID, tpl.Version, probetemplate.Input{Name: "filtered", Monitoring: &probetemplate.Monitoring{Network: 1, Egress: 0, NetworkInterfaces: &names}, Properties: tpl.Properties})
	if e != nil {
		t.Fatal(e)
	}
	adoptProbe(t, app, "v2", tpl.ID, nil)
	wait(func(v device.Snapshot, m map[string]device.Metric) bool {
		return m["egress_ipv4"].Reason == "collection_disabled" && m["egress_ipv6"].Reason == "collection_disabled"
	})
	old := first.LatestSession.ID
	app.Disconnect("v2")
	wait(func(v device.Snapshot, m map[string]device.Metric) bool {
		return v.LatestSession.ID != old && m["egress_ipv4"].Reason == "collection_disabled"
	})
	startProbe(t, binary, l.Addr().String(), "v2-counter", io.Discard, "--network-interfaces", "lo", "--egress-interval", "0")
	loopback := "lo"
	adoptProbe(t, app, "v2-counter", tpl.ID, &probetemplate.Monitoring{Network: 1, NetworkInterfaces: &loopback})
	waitCounter := func(check func(device.Snapshot, map[string]device.Metric) bool) device.Snapshot {
		t.Helper()
		end := time.Now().Add(12 * time.Second)
		for time.Now().Before(end) {
			v, e := app.Devices().Get("v2-counter")
			if e == nil && check(v, device.EffectiveMetrics(v.LatestSession, time.Now())) {
				return v
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatal("counter deadline")
		return device.Snapshot{}
	}
	counter := waitCounter(func(v device.Snapshot, m map[string]device.Metric) bool {
		seconds, _ := strconv.Atoi(m["net_6c6f_elapsed_seconds"].Value)
		return seconds >= 3 && m["net_6c6f_rx_bytes"].Value != "0"
	})
	for _, m := range counter.LatestSession.Telemetry.Groups["network"] {
		if m.Entity != "lo" {
			t.Fatal("device override did not override template", m)
		}
	}
	before := device.EffectiveMetrics(counter.LatestSession, time.Now())
	previousSeconds, _ := strconv.Atoi(before["net_6c6f_elapsed_seconds"].Value)
	app.Disconnect("v2-counter")
	waitCounter(func(v device.Snapshot, m map[string]device.Metric) bool {
		seconds, _ := strconv.Atoi(m["net_6c6f_elapsed_seconds"].Value)
		return v.LatestSession.ID != counter.LatestSession.ID && seconds > previousSeconds && m["egress_ipv4"].Reason == "collection_disabled"
	})
}
