package device

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestLifecycleAndTimeSemantics(t *testing.T) {
	s, _ := New(0)
	t0 := time.Unix(1000, 0)
	info := Registration{DeviceID: "router", Serial: "serial", Model: "model", Firmware: "firmware", ProbeVersion: "v1", Hostname: "host", Arch: "arm", Kernel: "kernel", Libc: "uclibc", BootID: "boot", Capabilities: []string{"exec", "future"}}
	if _, err := s.Get("router"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Sessions("router"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if s.Seen("router", "a", t0) || s.End("router", "a", Disconnected, t0) {
		t.Fatal("unknown device was created")
	}
	s.Publish(info, "a", t0)
	first, _ := s.Get("router")
	if !reflect.DeepEqual(first.Registration, info) || first.Status != Online || first.CurrentSession.ID != "a" || !first.FirstSeenAt.Equal(t0) || !first.LastOnlineAt.Equal(t0) || !first.LastOfflineAt.IsZero() {
		t.Fatalf("first: %#v", first)
	}
	s.Seen("router", "a", t0.Add(time.Second))
	s.Seen("router", "a", t0) // stale observed time cannot move activity backwards
	s.Publish(Registration{DeviceID: "router", ProbeVersion: "v2", Arch: "arm", BootID: "boot", Capabilities: []string{}}, "b", t0.Add(2*time.Second))
	s.Seen("router", "a", t0.Add(20*time.Second))
	s.End("router", "a", Disconnected, t0.Add(20*time.Second))
	second, _ := s.Get("router")
	if second.Status != Online || second.CurrentSession.ID != "b" || second.Registration.Hostname != "" || len(second.Registration.Capabilities) != 0 || !second.FirstSeenAt.Equal(t0) || !second.LastOnlineAt.Equal(t0.Add(2*time.Second)) || !second.LastOfflineAt.IsZero() || !second.LastSeenAt.Equal(second.LastOnlineAt) {
		t.Fatalf("replacement: %#v", second)
	}
	h, _ := s.Sessions("router")
	if h.Limit != 64 || len(h.Ended) != 1 || h.Ended[0].EndReason != Replaced || !h.Ended[0].EndedAt.Equal(second.LastOnlineAt) || !h.Ended[0].LastSeenAt.Equal(t0.Add(time.Second)) || !reflect.DeepEqual(h.Ended[0].Registration, info) {
		t.Fatalf("history: %#v", h)
	}
	offlineAt := t0.Add(3 * time.Second)
	if !s.End("router", "b", Disconnected, offlineAt) || s.End("router", "b", WriteError, t0.Add(4*time.Second)) {
		t.Fatal("end was not idempotent")
	}
	offline, _ := s.Get("router")
	if offline.Status != Offline || offline.CurrentSession != nil || offline.LatestSession.ID != "b" || !offline.LastOfflineAt.Equal(offlineAt) || !offline.LastOnlineAt.Equal(second.LastOnlineAt) || !offline.LastSeenAt.Equal(second.LastSeenAt) {
		t.Fatalf("offline: %#v", offline)
	}
	s.Publish(info, "c", t0.Add(5*time.Second))
	s.Publish(info, "d", t0.Add(6*time.Second))
	latest, _ := s.Get("router")
	if !latest.LastOfflineAt.Equal(offlineAt) || !latest.LastOnlineAt.Equal(t0.Add(6*time.Second)) || latest.TotalSessions != 4 {
		t.Fatalf("reconnect/replacement times: %#v", latest)
	}
}

func TestHistoryBoundAndInventoryLifetime(t *testing.T) {
	if _, err := New(-1); err == nil {
		t.Fatal("negative capacity")
	}
	for _, limit := range []int{1, 3, 64} {
		s, _ := New(limit)
		for i := 0; i < 100; i++ {
			s.Publish(Registration{DeviceID: "z", Firmware: fmt.Sprint(i)}, fmt.Sprint(i), time.Unix(int64(i+1), 0))
		}
		h, _ := s.Sessions("z")
		if len(h.Ended) != limit || h.TotalSessions != 100 || h.EvictedSessions != uint64(99-limit) || h.Current.ID != "99" || h.Ended[0].ID != fmt.Sprint(99-limit) {
			t.Fatalf("limit %d: %#v", limit, h)
		}
		s.End("z", "99", ServerClosed, time.Unix(200, 0))
		h, _ = s.Sessions("z")
		if h.Current != nil || len(h.Ended) != limit || h.EvictedSessions != uint64(100-limit) || h.Ended[limit-1].ID != "99" {
			t.Fatalf("final history: %#v", h)
		}
		s.Publish(Registration{DeviceID: "a"}, "a1", time.Unix(201, 0))
		list := s.List()
		if len(list) != 2 || list[0].Registration.DeviceID != "a" || list[1].Status != Offline || !list[1].FirstSeenAt.Equal(time.Unix(1, 0)) {
			t.Fatalf("inventory: %#v", list)
		}
		fresh, _ := New(limit)
		if len(fresh.List()) != 0 {
			t.Fatal("new Service recovered old state")
		}
	}
}

func TestQueryAndInputCopies(t *testing.T) {
	s, _ := New(1)
	info := Registration{DeviceID: "d", Capabilities: []string{"exec"}}
	s.Publish(info, "a", time.Now())
	info.Capabilities[0] = "changed"
	s.Publish(Registration{DeviceID: "d", Capabilities: []string{"file"}}, "b", time.Now())
	v, _ := s.Get("d")
	v.Registration.Capabilities[0] = "bad"
	v.CurrentSession.Registration.Capabilities[0] = "bad"
	v.LatestSession.Registration.Capabilities[0] = "bad"
	h, _ := s.Sessions("d")
	if h.Ended[0].Registration.Capabilities[0] != "exec" {
		t.Fatal("input alias")
	}
	h.Current.Registration.Capabilities[0] = "bad"
	h.Ended[0].Registration.Capabilities[0] = "bad"
	list := s.List()
	list[0].Registration.Capabilities[0] = "bad"
	list[0].CurrentSession.Registration.Capabilities[0] = "bad"
	list[0].LatestSession.Registration.Capabilities[0] = "bad"
	v, _ = s.Get("d")
	h, _ = s.Sessions("d")
	if v.Registration.Capabilities[0] != "file" || v.LatestSession.Registration.Capabilities[0] != "file" || v.CurrentSession.Registration.Capabilities[0] != "file" || h.Ended[0].Registration.Capabilities[0] != "exec" {
		t.Fatal("query aliases store")
	}
	// Every nested value in a single result is independent as well.
	v.CurrentSession.Registration.Capabilities[0] = "bad"
	if v.LatestSession.Registration.Capabilities[0] != "file" || v.Registration.Capabilities[0] != "file" {
		t.Fatal("nested snapshot alias")
	}
}

func TestConcurrentQueriesAndLifecycle(t *testing.T) {
	s, _ := New(3)
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for n := 0; n < 100; n++ {
				id := fmt.Sprintf("%d-%d", worker, n)
				s.Publish(Registration{DeviceID: "shared", Capabilities: []string{"exec"}}, id, time.Now())
				s.Seen("shared", id, time.Now())
				v, _ := s.Get("shared")
				v.Registration.Capabilities[0] = "local"
				h, _ := s.Sessions("shared")
				current := uint64(0)
				if h.Current != nil {
					current = 1
				}
				if h.TotalSessions != uint64(len(h.Ended))+h.EvictedSessions+current {
					t.Error("inconsistent history snapshot")
				}
				s.List()
				s.End("shared", id, Disconnected, time.Now())
			}
		}(worker)
	}
	wg.Wait()
	v, _ := s.Get("shared")
	if v.TotalSessions != 800 {
		t.Fatalf("lost sessions: %d", v.TotalSessions)
	}
}
