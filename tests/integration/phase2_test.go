package integration

import (
	"context"
	"io"
	"log"
	"net"
	"testing"
	"time"

	"routerprobe/internal/device"
	"routerprobe/internal/gateway"
	"routerprobe/internal/task"
)

func phase2Server(t *testing.T, limit int) (*gateway.Server, string) {
	t.Helper()
	s, err := gateway.New(gateway.Config{HeartbeatInterval: 10 * time.Second, DeviceHistoryLimit: limit, Logger: log.New(io.Discard, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(l) }()
	t.Cleanup(func() {
		s.Close()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	return s, l.Addr().String()
}

func phase2Get(t *testing.T, q device.Query, id string) device.Snapshot {
	t.Helper()
	v, err := q.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestDeviceInventoryRealProbeReconnectHistory(t *testing.T) {
	binary := probeBinary(t)
	s, address := phase2Server(t, 2)
	q := s.Devices()
	const id = "phase2-real"
	startProbe(t, binary, address, id, io.Discard)
	firstEvent := waitOnline(t, s.Events(), id, 5*time.Second)
	first := phase2Get(t, q, id)
	if first.Status != device.Online || first.CurrentSession.ID != firstEvent.SessionID || first.Registration.Hostname != "phase1b-host" || first.Registration.BootID != "phase1b-boot" || first.Registration.Arch == "" || first.Registration.ProbeVersion == "" || len(first.Registration.Capabilities) == 0 {
		t.Fatalf("real REGISTER not retained: %#v", first)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	taskID, err := s.CreateExec(ctx, id, task.ExecRequest{Command: "printf phase2", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.WaitTaskResult(ctx, taskID)
	if err != nil || result.Stdout != "phase2" {
		t.Fatalf("exec: %#v %v", result, err)
	}
	latest := first
	for n := 0; n < 4; n++ {
		if !s.Disconnect(id) {
			t.Fatal("disconnect")
		}
		offline := phase2Get(t, q, id)
		if offline.Status != device.Offline || offline.CurrentSession != nil || offline.LatestSession.ID != latest.CurrentSession.ID || offline.LastOfflineAt.IsZero() {
			t.Fatalf("offline: %#v", offline)
		}
		event := waitOnline(t, s.Events(), id, 7*time.Second)
		latest = phase2Get(t, q, id)
		if latest.CurrentSession.ID != event.SessionID || latest.CurrentSession.ID == offline.LatestSession.ID || !latest.FirstSeenAt.Equal(first.FirstSeenAt) || !latest.LastOfflineAt.Equal(offline.LastOfflineAt) || !latest.LastOnlineAt.After(offline.LastOnlineAt) {
			t.Fatalf("reconnect: %#v", latest)
		}
	}
	h, err := q.Sessions(id)
	if err != nil || h.TotalSessions != 5 || h.EvictedSessions != 2 || len(h.Ended) != 2 || h.Current.ID != latest.CurrentSession.ID || len(q.List()) != 1 {
		t.Fatalf("bounded history: %#v %v", h, err)
	}
	for _, ended := range h.Ended {
		if ended.EndReason != device.RequestedDisconnect || ended.EndedAt.IsZero() || ended.Registration.Hostname != first.Registration.Hostname {
			t.Fatalf("ended session: %#v", ended)
		}
	}
	// Device history eviction must not discard task identities/results/dispatches.
	if err := s.ResendTask(ctx, taskID); err != nil {
		t.Fatal(err)
	}
	replayed, err := s.WaitTaskResult(ctx, taskID)
	if err != nil || replayed.Stdout != result.Stdout {
		t.Fatalf("task after history eviction: %#v %v", replayed, err)
	}
	snap, err := s.TaskSnapshot(taskID)
	if err != nil || len(snap.Dispatches) != 2 || snap.Dispatches[0].SessionID != first.CurrentSession.ID {
		t.Fatalf("task dispatch history lost: %#v %v", snap, err)
	}
	q.List()[0].Registration.Capabilities[0] = "mutated"
	if phase2Get(t, q, id).Registration.Capabilities[0] == "mutated" {
		t.Fatal("query returned mutable inventory")
	}
	s.Close()
	closed := phase2Get(t, q, id)
	if closed.Status != device.Offline || closed.LatestSession.EndReason != device.ServerClosed {
		t.Fatalf("server shutdown: %#v", closed)
	}
}

func TestDeviceRealProbeReplacement(t *testing.T) {
	binary := probeBinary(t)
	s, address := phase2Server(t, 0)
	const id = "phase2-replaced"
	firstProbe := startProbe(t, binary, address, id, io.Discard)
	firstEvent := waitOnline(t, s.Events(), id, 5*time.Second)
	startProbe(t, binary, address, id, io.Discard)
	secondEvent := waitOnline(t, s.Events(), id, 5*time.Second)
	// Stop the displaced process before its reconnect backoff expires.
	firstProbe.Process.Kill()
	v := phase2Get(t, s.Devices(), id)
	h, _ := s.Devices().Sessions(id)
	if firstEvent.SessionID == secondEvent.SessionID || v.Status != device.Online || v.CurrentSession.ID != secondEvent.SessionID || !v.LastOfflineAt.IsZero() || len(h.Ended) != 1 || h.Ended[0].ID != firstEvent.SessionID || h.Ended[0].EndReason != device.Replaced || !h.Ended[0].EndedAt.Equal(v.LastOnlineAt) {
		t.Fatalf("real replacement: %#v %#v", v, h)
	}
	if r := runExec(t, s, id, task.ExecRequest{Command: "printf current", Timeout: time.Second}); r.Stdout != "current" {
		t.Fatalf("replacement dispatch: %#v", r)
	}
}
