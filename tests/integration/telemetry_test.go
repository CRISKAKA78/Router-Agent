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
	"testing"
	"time"
)

func TestTelemetryRealProbe(t *testing.T) {
	binary := probeBinary(t)
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
	path := filepath.Join(t.TempDir(), "cpu")
	os.WriteFile(path, []byte("42"), 0600)
	tpl, e := app.ProbeTemplates().Put("", 0, probetemplate.Input{Name: "cycles", Monitoring: &probetemplate.Monitoring{CPU: 1, Memory: 2, Disk: 0, Network: 1}, Properties: map[string]probetemplate.Property{
		"cpu_usage": {Name: "Template CPU", Command: "cat '" + path + "'", Interval: 1},
		"slow":      {Name: "Slow", Command: "sleep 2; printf slow", Timeout: 3, Interval: 1},
	}})
	if e != nil {
		t.Fatal(e)
	}
	startProbe(t, binary, l.Addr().String(), "telemetry", io.Discard)
	adoptProbe(t, app, "telemetry", tpl.ID, nil)
	wait := func(check func(device.Snapshot, map[string]device.Metric) bool) device.Snapshot {
		t.Helper()
		deadline := time.Now().Add(12 * time.Second)
		for time.Now().Before(deadline) {
			v, err := app.Devices().Get("telemetry")
			if err == nil {
				m := device.EffectiveMetrics(v.LatestSession, time.Now())
				if check(v, m) {
					return v
				}
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatal("telemetry deadline")
		return device.Snapshot{}
	}
	first := wait(func(v device.Snapshot, m map[string]device.Metric) bool {
		return m["cpu_usage"].Value == "42" && m["memory_total_bytes"].Status == "ok" && m["cpu_arch"].Status == "ok" && len(v.LatestSession.Telemetry.Groups["cpu"]) > 0
	})
	if first.Registration.SourceIP != "127.0.0.1" {
		t.Fatal("source IP", first.Registration.SourceIP)
	}
	m := device.EffectiveMetrics(first.LatestSession, time.Now())
	if m["cpu_usage"].Source != "template" || m["memory_total_bytes"].Interval != 2 || len(first.LatestSession.Telemetry.Groups["disk"]) != 0 {
		t.Fatal(m)
	}
	cpuAt := first.LatestSession.Telemetry.Groups["cpu"]["cpu_usage"].SampledAt
	wait(func(v device.Snapshot, m map[string]device.Metric) bool {
		return v.LatestSession.Telemetry.Groups["cpu"]["cpu_usage"].SampledAt.Sub(cpuAt) >= 2*time.Second
	})
	os.WriteFile(path, []byte("63"), 0600)
	changed := wait(func(v device.Snapshot, m map[string]device.Metric) bool { return m["cpu_usage"].Value == "63" })
	if changed.Registration.DeviceID != first.Registration.DeviceID || changed.Registration.Hostname != first.Registration.Hostname {
		t.Fatal("registration snapshot rewritten")
	}
	wire := p5request(t, http.URL, "GET", "/devices/telemetry", "", nil, 200)
	if wire["source_ip"] != "127.0.0.1" || wire["effective_metrics"].(map[string]any)["cpu_usage"].(map[string]any)["value"] != "63" {
		t.Fatal(wire)
	}
	os.WriteFile(path, []byte("invalid"), 0600)
	wait(func(v device.Snapshot, m map[string]device.Metric) bool {
		return m["cpu_usage"].Status == "error" && m["cpu_usage"].Value == ""
	})
	old := changed.LatestSession.ID
	app.Disconnect("telemetry")
	wait(func(v device.Snapshot, m map[string]device.Metric) bool {
		return v.LatestSession.ID != old && m["memory_total_bytes"].Status == "ok"
	})
}
