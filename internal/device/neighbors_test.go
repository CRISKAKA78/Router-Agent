package device

import (
	"routerprobe/internal/probetemplate"
	"testing"
	"time"
)

func TestNeighborsAreScopedToSessionConfigurationAndPort(t *testing.T) {
	s, err := New(2)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	s.Publish(Registration{DeviceID: "d"}, "one", at)
	p := probetemplate.Template{NeighborProbe: &probetemplate.NeighborProbe{Interval: 30, Domains: []probetemplate.NeighborDomain{{ID: "lan", Scope: "lan", Interface: "br0", Ports: []string{"LAN1"}}}}}
	s.ApplyConfiguration("d", "one", 1, &p, "")
	n := Neighbors{Revision: 1, Interval: 30, Unclassified: []NeighborRow{}, Domains: []NeighborDomain{{ID: "lan", Scope: "lan", Interface: "br0", Status: "ok", Rows: []NeighborRow{{IP: "192.0.2.2", MAC: "02:00:00:00:00:02", Port: "LAN1"}}}}}
	if !s.ObserveNeighbors("d", "one", n, at, time.Second) {
		t.Fatal("valid observation")
	}
	n.Domains[0].Rows[0].Port = "other"
	if s.ObserveNeighbors("d", "one", n, at, 0) {
		t.Fatal("wrong port accepted")
	}
	d, _ := s.Get("d")
	out := NeighborSnapshot(d, at)
	if out.Domains[0].Rows[0].Port != "LAN1" || !out.SampledAt.Equal(at.Add(-time.Second)) {
		t.Fatal(out)
	}
	out.Domains[0].Rows[0].IP = "changed"
	d, _ = s.Get("d")
	if d.LatestSession.Neighbors.Domains[0].Rows[0].IP == "changed" {
		t.Fatal("alias")
	}
	if !NeighborSnapshot(d, at.Add(100*time.Second)).Stale {
		t.Fatal("age")
	}
	s.ApplyConfiguration("d", "one", 2, &p, "")
	if s.ObserveNeighbors("d", "one", n, at, 0) {
		t.Fatal("old config")
	}
	d, _ = s.Get("d")
	if d.LatestSession.Neighbors != nil {
		t.Fatal("old snapshot retained")
	}
	s.Publish(Registration{DeviceID: "d"}, "two", at)
	if s.ObserveNeighbors("d", "one", n, at, 0) {
		t.Fatal("old session")
	}
}
