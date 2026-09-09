package device

import (
	"testing"
	"time"
)

func TestHeartbeatRuntimeLifecycle(t *testing.T) {
	s, _ := New(2)
	at := time.Now()
	s.Publish(Registration{DeviceID: "router", Arch: "arm"}, "first", at)
	initial, _ := s.Get("router")
	if initial.LatestSession.Runtime != nil {
		t.Fatal("runtime exists before heartbeat")
	}
	seconds := uint64(1234)
	revision := s.Revision()
	if !s.Heartbeat("router", "first", &seconds, at.Add(time.Second)) {
		t.Fatal("heartbeat rejected")
	}
	seconds = 9999
	first, _ := s.Get("router")
	if *first.CurrentSession.Runtime.UptimeSeconds != 1234 || s.Revision() != revision {
		t.Fatal("input alias or heartbeat notification")
	}
	*first.CurrentSession.Runtime.UptimeSeconds = 7
	*first.LatestSession.Runtime.UptimeSeconds = 8
	s.Seen("router", "first", at.Add(2*time.Second))
	v, _ := s.Get("router")
	if *v.LatestSession.Runtime.UptimeSeconds != 1234 || !v.LatestSession.Runtime.ReportedAt.Equal(at.Add(time.Second)) {
		t.Fatal("query alias or other activity changed sample")
	}
	s.Publish(Registration{DeviceID: "router"}, "second", at.Add(3*time.Second))
	if s.Heartbeat("router", "first", &seconds, at.Add(4*time.Second)) {
		t.Fatal("stale session accepted")
	}
	v, _ = s.Get("router")
	if v.LatestSession.Runtime != nil {
		t.Fatal("new session inherited old runtime")
	}
	h, _ := s.Sessions("router")
	if *h.Ended[0].Runtime.UptimeSeconds != 1234 {
		t.Fatal("history lost final sample")
	}
	*h.Ended[0].Runtime.UptimeSeconds = 9
	h, _ = s.Sessions("router")
	if *h.Ended[0].Runtime.UptimeSeconds != 1234 {
		t.Fatal("history query alias")
	}
	zero := uint64(0)
	s.Heartbeat("router", "second", &zero, at.Add(5*time.Second))
	v, _ = s.Get("router")
	if v.LatestSession.Runtime.UptimeSeconds == nil || *v.LatestSession.Runtime.UptimeSeconds != 0 {
		t.Fatal("valid zero lost")
	}
	s.Heartbeat("router", "second", nil, at.Add(6*time.Second))
	if s.Heartbeat("router", "second", &seconds, at.Add(5*time.Second)) {
		t.Fatal("old sample accepted")
	}
	s.End("router", "second", Disconnected, at.Add(7*time.Second))
	if s.Heartbeat("router", "second", &seconds, at.Add(8*time.Second)) {
		t.Fatal("ended session accepted")
	}
	v, _ = s.Get("router")
	if v.Status != Offline || v.LatestSession.Runtime.UptimeSeconds != nil || !v.LatestSession.Runtime.ReportedAt.Equal(at.Add(6*time.Second)) {
		t.Fatal("unknown/offline sample lost")
	}
}
