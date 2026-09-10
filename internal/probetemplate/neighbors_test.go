package probetemplate

import (
	"encoding/json"
	"testing"
)

func TestNeighborPlanRoundTripAndIsolation(t *testing.T) {
	input := Input{Name: "neighbors", Properties: map[string]Property{}, NeighborProbe: &NeighborProbe{Interval: 30, Domains: []NeighborDomain{{ID: "lan", Scope: "lan", Interface: "br0", Ports: []string{"LAN1"}}, {ID: "local", Scope: "broadcast", Interface: "br0"}}}}
	s, e := Open(t.TempDir() + "/templates.json")
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Put("", 0, input)
	if e != nil {
		t.Fatal(e)
	}
	input.NeighborProbe.Domains[0].Ports[0] = "changed"
	if v.NeighborProbe.Domains[0].Ports[0] != "LAN1" {
		t.Fatal("aliased input")
	}
	s.Close()
	s, e = Open(s.path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	loaded, e := s.Resolve(v.ID, "")
	if e != nil || loaded.NeighborProbe.Domains[1].Scope != "broadcast" {
		t.Fatal(e)
	}
	for _, raw := range []string{`{"domains":[{"id":"x","scope":"broadcast","interface":"br0","lease_file":null}]}`, `{"domains":[{"id":"x","scope":"uplink","interface":"br0"}]}`, `{"domains":[{"id":"x","scope":"lan","interface":"br0"}]}`, `{"interval_seconds":1,"domains":[{"id":"x","scope":"broadcast","interface":"br0"}]}`} {
		var n NeighborProbe
		if json.Unmarshal([]byte(raw), &n) == nil {
			t.Fatal(raw)
		}
	}
}
