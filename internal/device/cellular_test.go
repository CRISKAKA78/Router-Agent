package device

import (
	"routerprobe/internal/probetemplate"
	"testing"
	"time"
)

func TestCellularSnapshotLifecycle(t *testing.T) {
	s, _ := New(2)
	at := time.Now()
	s.Publish(Registration{DeviceID: "d"}, "one", at)
	plan := probetemplate.Template{CellularProbe: &probetemplate.CellularProbe{Interval: 30}}
	s.ApplyConfiguration("d", "one", 1, &plan, "")
	n := Cellular{Revision: 1, Interval: 30, Status: "ok", Ports: []CellularPort{{Path: "/dev/ttyUSB2", AgeMS: 1500, IMEI: AtIdentity{Value: "867123456789012"}}}}
	if !s.ObserveCellular("d", "one", n, at, time.Second) {
		t.Fatal("observation")
	}
	n.Ports[0].IMEI.Value = "changed"
	d, _ := s.Get("d")
	out := CellularSnapshot(d, at)
	if out.Ports[0].IMEI.Value != "867123456789012" || !out.SampledAt.Equal(at.Add(-time.Second)) || !out.Ports[0].SampledAt.Equal(at.Add(-1500*time.Millisecond)) {
		t.Fatal("value/age", out)
	}
	out.Ports[0].IMEI.Value = "alias"
	d, _ = s.Get("d")
	if d.LatestSession.Cellular.Ports[0].IMEI.Value == "alias" {
		t.Fatal("snapshot alias")
	}
	if !CellularSnapshot(d, at.Add(91*time.Second)).Stale {
		t.Fatal("stale")
	}
	if s.ObserveCellular("d", "one", n, at.Add(-time.Second), time.Second) {
		t.Fatal("older sample")
	}
	n.Interval = 60
	if s.ObserveCellular("d", "one", n, at, 0) {
		t.Fatal("wrong interval")
	}
	n.Interval = 30
	s.End("d", "one", Disconnected, at.Add(time.Second))
	d, _ = s.Get("d")
	if !CellularSnapshot(d, at.Add(time.Second)).Stale {
		t.Fatal("offline")
	}
	s.Publish(Registration{DeviceID: "d"}, "two", at.Add(2*time.Second))
	if s.ObserveCellular("d", "one", n, at, 0) {
		t.Fatal("old session")
	}
	s.ApplyConfiguration("d", "two", 2, &plan, "")
	if s.ObserveCellular("d", "two", n, at, 0) {
		t.Fatal("old revision")
	}
	d, _ = s.Get("d")
	if CellularSnapshot(d, at) != nil {
		t.Fatal("previous sample survived")
	}
}
