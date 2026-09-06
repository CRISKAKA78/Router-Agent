package gateway

import (
	"context"
	"routerprobe/internal/device"
	"routerprobe/internal/tunnel"
	"testing"
	"time"
)

func TestTunnelBindingReplacementAndWriterRevalidation(t *testing.T) {
	s, address := startTestServer(t)
	payload := `{"device_id":"bound","probe_version":"v1","arch":"arm","boot_id":"boot","capabilities":["tunnel"]}`
	deviceConnect(t, s, address, payload)
	binding, e := s.BindTunnel("bound")
	if e != nil {
		t.Fatal(e)
	}
	s.mu.Lock()
	old := s.sessions["bound"]
	s.mu.Unlock()
	old.transport.priority.lock(false)
	sending := make(chan error, 1)
	go func() { sending <- binding.Send(context.Background(), false, tunnel.Command{MaintenanceID: "old"}) }()
	deviceConnect(t, s, address, payload)
	select {
	case <-binding.Done:
	default:
		t.Fatal("replacement did not synchronously revoke binding")
	}
	old.transport.priority.unlock()
	select {
	case e := <-sending:
		if e == nil {
			t.Fatal("stale send admitted")
		}
	case <-time.After(time.Second):
		t.Fatal("send stuck")
	}
	current, e := s.BindTunnel("bound")
	if e != nil || current.ID == binding.ID {
		t.Fatal(current, e)
	}
	s.endSession(old, device.Disconnected)
	select {
	case <-current.Done:
		t.Fatal("late old cleanup revoked new session")
	default:
	}
	s.Disconnect("bound")
	select {
	case <-current.Done:
	default:
		t.Fatal("disconnect did not revoke")
	}
}
