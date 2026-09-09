package device

import (
	"testing"
	"time"
)

func TestConnectionPeriodsFollowDeviceTransitions(t *testing.T) {
	s, _ := New(2)
	base := time.Unix(1000, 0)
	info := Registration{DeviceID: "router"}
	s.Publish(info, "a", base)
	s.Publish(info, "b", base.Add(10*time.Second))
	if s.End("router", "a", Disconnected, base.Add(11*time.Second)) {
		t.Fatal("old session changed device")
	}
	h, _ := s.Connections("router")
	if len(h.Periods) != 1 || !h.Periods[0].OnlineAt.Equal(base) || !h.Periods[0].OfflineAt.IsZero() {
		t.Fatal("replacement interrupted online period", h)
	}
	s.End("router", "b", HeartbeatTimeout, base.Add(30*time.Second))
	h, _ = s.Connections("router")
	if h.Periods[0].OfflineAt.Sub(h.Periods[0].OnlineAt) != 30*time.Second || !h.Periods[0].ReconnectedAt.IsZero() {
		t.Fatal("outage did not start", h)
	}
	s.Publish(info, "c", base.Add(50*time.Second))
	h, _ = s.Connections("router")
	if len(h.Periods) != 2 || h.Periods[1].ReconnectedAt.Sub(h.Periods[1].OfflineAt) != 20*time.Second || h.Periods[0].ID == h.Periods[1].ID {
		t.Fatal("reconnect failed to freeze outage and start online interval", h)
	}
	h.Periods[0].OnlineAt = time.Time{}
	again, _ := s.Connections("router")
	if again.Periods[0].OnlineAt.IsZero() {
		t.Fatal("history is not a snapshot")
	}
	for i, id := range []string{"d", "e", "f"} {
		current := []string{"c", "d", "e"}[i]
		s.End("router", current, Disconnected, base.Add(time.Duration(60+i*20)*time.Second))
		s.Publish(info, id, base.Add(time.Duration(70+i*20)*time.Second))
	}
	h, _ = s.Connections("router")
	if len(h.Periods) != 3 || h.Total != 5 || h.Evicted != 2 {
		t.Fatal("connection history is not bounded", h)
	}
}
