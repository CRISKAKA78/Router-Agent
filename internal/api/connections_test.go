package api

import (
	"routerprobe/internal/device"
	"testing"
	"time"
)

func TestConnectionDurationStates(t *testing.T) {
	start := time.Unix(1000, 0)
	p := device.ConnectionPeriod{ID: 1, OnlineAt: start}
	online := connectionDTO(p, start.Add(25*time.Second))
	if online["state"] != "online" || online["online_seconds"] != int64(25) || online["offline_seconds"] != nil {
		t.Fatal(online)
	}
	p.OfflineAt = start.Add(30 * time.Second)
	offline := connectionDTO(p, start.Add(40*time.Second))
	if offline["state"] != "offline" || offline["online_seconds"] != int64(30) || offline["offline_seconds"] != int64(10) {
		t.Fatal(offline)
	}
	p.ReconnectedAt = start.Add(50 * time.Second)
	completed := connectionDTO(p, start.Add(200*time.Second))
	if completed["state"] != "completed" || completed["online_seconds"] != int64(30) || completed["offline_seconds"] != int64(20) {
		t.Fatal(completed)
	}
}
