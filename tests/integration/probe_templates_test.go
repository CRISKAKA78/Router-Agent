package integration

import (
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"routerprobe/internal/device"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/probetemplate"
	"testing"
	"time"
)

func TestProbeTemplateControlLimit(t *testing.T) {
	if _, err := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{MaxControlPayload: 1024}}); err == nil {
		t.Fatal("current configuration requires a 64KiB control limit")
	}
}

func TestProbeTemplateStartupSnapshot(t *testing.T) {
	binary := probeBinary(t)
	app, e := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{Logger: log.New(io.Discard, "", 0)}})
	if e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(l) }()
	defer func() { app.Close(); <-done }()
	tmp := t.TempDir()
	counter := filepath.Join(tmp, "calls")
	tpl, e := app.ProbeTemplates().Put("", 0, probetemplate.Input{Name: "Startup", Properties: map[string]probetemplate.Property{"model": {Name: "Model", Command: "printf model-v1"}, "custom": {Name: "Custom", Command: "printf x >> '" + counter + "'; printf custom-value"}, "bad": {Name: "Bad", Command: "exit 2"}, "hostname": {Name: "Hostname", Command: "printf collected-host"}}})
	if e != nil {
		t.Fatal(e)
	}
	startProbe(t, binary, l.Addr().String(), "template-device", io.Discard, "--hostname", "manual-host")
	adoptProbe(t, app, "template-device", tpl.ID, nil)
	first := waitManagedMetric(t, app, "template-device", func(d device.Snapshot, m map[string]device.Metric) bool {
		return m["custom"].Value == "custom-value" && m["bad"].Reason == "command_failed" && m["model"].Value == "model-v1"
	})
	if first.Registration.Hostname != "manual-host" {
		t.Fatal("raw startup snapshot changed", first.Registration)
	}
	tpl, e = app.ProbeTemplates().Put(tpl.ID, tpl.Version, probetemplate.Input{Name: tpl.Name, Properties: map[string]probetemplate.Property{"model": {Name: "Model", Command: "printf model-v2"}, "custom": tpl.Properties["custom"]}})
	if e != nil {
		t.Fatal(e)
	}
	app.Disconnect("template-device")
	second := waitManagedMetric(t, app, "template-device", func(d device.Snapshot, m map[string]device.Metric) bool {
		return d.LatestSession.ID != first.LatestSession.ID && m["custom"].Value == "custom-value"
	})
	if second.LatestSession.ConfigTemplate.Version != 1 || device.EffectiveMetrics(second.LatestSession, time.Now())["model"].Value != "model-v1" {
		t.Fatal("publication auto-applied")
	}
	calls, _ := os.ReadFile(counter)
	if string(calls) != "x" {
		t.Fatal("reconnect recollected startup property", string(calls))
	}
	history, e := app.Devices().Sessions("template-device")
	if e != nil || len(history.Ended) != 1 || history.Ended[0].ConfigTemplate.Version != 1 {
		t.Fatal("historical configuration snapshot lost", history, e)
	}
	adoptProbe(t, app, "template-device", tpl.ID, nil)
	waitManagedMetric(t, app, "template-device", func(d device.Snapshot, m map[string]device.Metric) bool {
		return d.LatestSession.ConfigTemplate != nil && d.LatestSession.ConfigTemplate.Version == 2 && m["model"].Value == "model-v2"
	})
	calls, _ = os.ReadFile(counter)
	if string(calls) != "xx" {
		t.Fatal("explicit new version must collect once", string(calls))
	}
}
