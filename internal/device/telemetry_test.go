package device

import (
	"routerprobe/internal/probetemplate"
	"testing"
	"time"
)

func TestTelemetryPriorityAndSessionIsolation(t *testing.T) {
	s, _ := New(2)
	at := time.Now()
	r := Registration{DeviceID: "d"}
	s.Publish(r, "a", at)
	s.ApplyConfiguration("d", "a", 1, &probetemplate.Template{Properties: map[string]probetemplate.Property{"cpu_usage": {Name: "Custom", Interval: 2}}}, "")
	s.Observe("d", "a", "template", map[string]Metric{"cpu_usage": {Name: "Custom", Value: "42", Status: "ok"}}, at)
	builtin := map[string]Metric{"cpu_usage": {Name: "CPU", Value: "10", Unit: "percent", Status: "ok", Interval: 1}}
	if !s.Observe("d", "a", "cpu", builtin, at.Add(time.Second)) {
		t.Fatal("observe")
	}
	v, _ := s.Get("d")
	m := EffectiveMetrics(v.LatestSession, at.Add(time.Second))["cpu_usage"]
	if m.Value != "42" || m.Source != "template" {
		t.Fatal(m)
	}
	s.Observe("d", "a", "template", map[string]Metric{"cpu_usage": {Value: "not a number", Status: "ok"}}, at.Add(2*time.Second))
	v, _ = s.Get("d")
	if m = EffectiveMetrics(v.LatestSession, at.Add(20*time.Second))["cpu_usage"]; m.Status != "error" || m.Value != "" || !m.Stale {
		t.Fatal(m)
	}
	if s.Observe("d", "a", "template", map[string]Metric{"unselected": {}}, at) {
		t.Fatal("unselected template property accepted")
	}
	v.LatestSession.Telemetry.Template["cpu_usage"] = Metric{Value: "mutated"}
	v, _ = s.Get("d")
	if v.LatestSession.Telemetry.Template["cpu_usage"].Value == "mutated" {
		t.Fatal("snapshot alias")
	}
	s.Publish(Registration{DeviceID: "d"}, "b", at.Add(3*time.Second))
	if s.Observe("d", "a", "cpu", builtin, at.Add(4*time.Second)) {
		t.Fatal("old session")
	}
	v, _ = s.Get("d")
	if len(EffectiveMetrics(v.LatestSession, at)) != 0 {
		t.Fatal("inherited sample")
	}
	s.Observe("d", "b", "network", map[string]Metric{"net_x_state": {Value: "up", Status: "ok", Unit: "text"}}, at.Add(4*time.Second))
	s.Observe("d", "b", "network", map[string]Metric{}, at.Add(5*time.Second))
	v, _ = s.Get("d")
	if len(EffectiveMetrics(v.LatestSession, at)) != 0 {
		t.Fatal("removed interface retained")
	}
}
