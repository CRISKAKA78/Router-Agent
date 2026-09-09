package device

import (
	"testing"
	"time"
)

func TestEgressIndependentResultsAndHeartbeat(t *testing.T) {
	s, _ := New(2)
	at := time.Now()
	s.Publish(Registration{DeviceID: "d"}, "a", at)
	s.Observe("d", "a", "egress", map[string]Metric{"egress_ipv4": {Value: "8.8.8.8", Unit: "text", Status: "ok"}}, at)
	s.Observe("d", "a", "egress", map[string]Metric{"egress_ipv6": {Status: "error", Reason: "timeout"}}, at.Add(time.Second))
	v, _ := s.Get("d")
	m := EffectiveMetrics(v.LatestSession, at)
	if m["egress_ipv4"].Value != "8.8.8.8" || m["egress_ipv6"].Reason != "timeout" || v.LatestSession.Runtime != nil {
		t.Fatal(m, v.LatestSession.Runtime)
	}
	s.Observe("d", "a", "egress", map[string]Metric{"egress_ipv4": {Status: "error", Value: "8.8.8.8", Reason: "egress_request_failed"}}, at)
	v, _ = s.Get("d")
	if EffectiveMetrics(v.LatestSession, at)["egress_ipv4"].Value != "" {
		t.Fatal("failed lookup retained old IP")
	}
	s.Publish(Registration{DeviceID: "d"}, "b", at.Add(2*time.Second))
	v, _ = s.Get("d")
	if len(EffectiveMetrics(v.LatestSession, at)) != 0 {
		t.Fatal("new session retained egress")
	}
}
