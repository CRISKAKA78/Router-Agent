package device

import (
	"fmt"
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

func TestRecentNeighborsFreshnessCapacityAndIsolation(t *testing.T) {
	s, _ := New(2)
	at := time.Now()
	s.Publish(Registration{DeviceID: "d"}, "s", at)
	p := probetemplate.Template{NeighborProbe: &probetemplate.NeighborProbe{Interval: 30, Domains: []probetemplate.NeighborDomain{{ID: "local", Scope: "broadcast", Interface: "br0"}, {ID: "lan", Scope: "lan", Interface: "br0", Ports: []string{"lan1"}}}}}
	s.ApplyConfiguration("d", "s", 1, &p, "")
	age := uint64(0)
	row := NeighborRow{IP: "192.0.2.2", MAC: "02:00:00:00:00:02", Port: "lan1", Source: "active_arp", State: "responded", ActiveAgeMS: &age}
	n := Neighbors{Revision: 1, Interval: 30, Unclassified: []NeighborRow{}, Domains: []NeighborDomain{{ID: "local", Scope: "broadcast", Interface: "br0", Rows: []NeighborRow{row}}, {ID: "lan", Scope: "lan", Interface: "br0", Rows: []NeighborRow{row}}}}
	if !s.ObserveNeighbors("d", "s", n, at, 0) {
		t.Fatal("sample rejected")
	}
	d, _ := s.Get("d")
	if len(RecentNeighborSnapshot(d, at)) != 2 {
		t.Fatal("overlap lost")
	}
	age = 59000
	if !s.ObserveNeighbors("d", "s", n, at.Add(59*time.Second), 0) {
		t.Fatal("repeat rejected")
	}
	d, _ = s.Get("d")
	for _, r := range RecentNeighborSnapshot(d, at.Add(61*time.Second)) {
		if r.State != "recent" || r.Source == "active_arp" {
			t.Fatalf("false active freshness: %+v", r)
		}
	}
	n.Domains[0].Rows = nil
	n.Domains[1].Rows = nil
	s.ObserveNeighbors("d", "s", n, at.Add(62*time.Second), 0)
	d, _ = s.Get("d")
	if len(RecentNeighborSnapshot(d, at.Add(time.Minute*2))) != 2 {
		t.Fatal("history disappeared with snapshot")
	}
	if len(RecentNeighborSnapshot(d, at.Add(24*time.Hour))) != 0 {
		t.Fatal("TTL")
	}
	for i := 0; i < 1100; i++ {
		n.Domains[0].Rows = []NeighborRow{{MAC: fmt.Sprintf("02:00:00:%02x:%02x:01", i/256, i%256), Source: "fdb", State: "mac_only"}}
		s.ObserveNeighbors("d", "s", n, at.Add(time.Duration(i)*time.Second), 0)
	}
	d, _ = s.Get("d")
	if len(d.RecentNeighbors) != 1024 || d.RecentNeighbors[0].State != "mac_only" || d.RecentNeighbors[0].IP != "" {
		t.Fatal("capacity/MAC-only")
	}
	copy := d.RecentNeighbors
	copy[0].MAC = "mutation"
	d, _ = s.Get("d")
	if d.RecentNeighbors[0].MAC == "mutation" {
		t.Fatal("alias")
	}
	s.ApplyConfiguration("d", "s", 2, &p, "")
	d, _ = s.Get("d")
	if len(d.RecentNeighbors) != 0 {
		t.Fatal("revision leaked")
	}
	n.Revision = 2
	s.ObserveNeighbors("d", "s", n, at, 0)
	s.Publish(Registration{DeviceID: "d"}, "new", at)
	d, _ = s.Get("d")
	if len(d.RecentNeighbors) != 0 || s.ObserveNeighbors("d", "s", n, at, 0) {
		t.Fatal("session leaked")
	}
	fresh, _ := New(2)
	if _, e := fresh.Get("d"); e == nil {
		t.Fatal("restart inherited memory")
	}
}
func TestNetworkDiscoverySubrangesAndBridgeMembers(t *testing.T) {
	raw := `{"networks":[{"interface":"br0","bridge":true,"eligible":true,"ipv4":["192.168.5.222/24","198.51.100.1/25"],"ports":["eth0","vlan3"]},{"interface":"eth0","master":"br0","eligible":true,"ipv4":["192.168.5.223/24"],"ports":[]},{"interface":"eth1.10","vlan":true,"eligible":true,"ipv4":[],"ports":[]}],"preset_status":"not_tested"}`
	n, e := ParseNeighborDiscovery(raw)
	if e != nil || n.Networks[0].Networks[0] != "192.168.5.0/24" || n.Networks[1].Eligible || !n.Networks[2].VLAN {
		t.Fatalf("inventory: %+v %v", n, e)
	}
	s, _ := New(1)
	at := time.Now()
	s.Publish(Registration{DeviceID: "d"}, "s", at)
	p := probetemplate.Template{NeighborProbe: &probetemplate.NeighborProbe{Interval: 30, Domains: []probetemplate.NeighborDomain{{ID: "local", Scope: "broadcast", Interface: "br0"}}}}
	s.ApplyConfiguration("d", "s", 1, &p, "")
	if s.ObserveNeighborDiscovery("d", "old", 1, n, at) || s.ObserveNeighborDiscovery("d", "s", 2, n, at) || !s.ObserveNeighborDiscovery("d", "s", 1, n, at) {
		t.Fatal("inspection isolation")
	}
	if !s.ObserveNeighborDiscovery("d", "s", 1, n, at, 20) || s.ObserveNeighborDiscovery("d", "s", 1, n, at.Add(time.Second), 19) {
		t.Fatal("late earlier inspection replaced the newer dispatch result")
	}
	d, _ := s.Get("d")
	for _, cidr := range []string{"192.168.5.0/24", "192.168.5.128/25", "198.51.100.4/32"} {
		if !NeighborRangeOnLink(d, "local", cidr, at) {
			t.Fatal(cidr)
		}
	}
	for _, cidr := range []string{"192.168.6.0/24", "192.168.5.222/24", "192.168.4.0/23", "198.51.100.128/25"} {
		if NeighborRangeOnLink(d, "local", cidr, at) {
			t.Fatal(cidr)
		}
	}
	if NeighborRangeOnLink(d, "local", "192.168.5.0/24", at.Add(91*time.Second)) {
		t.Fatal("stale inspection accepted")
	}
}
