package device

import (
	"routerprobe/internal/probetemplate"
	"testing"
	"time"
)

func TestCollectionSnapshotIsolation(t *testing.T) {
	s, _ := New(2)
	s.Publish(Registration{DeviceID: "id"}, "one", time.Now())
	template := &probetemplate.Template{ID: "t", Name: "T", Properties: map[string]probetemplate.Property{"x": {Name: "X"}, "y": {Name: "Y"}}}
	s.ApplyConfiguration("id", "one", 1, template, "")
	values := map[string]Metric{"x": {Name: "X", Value: "first", Status: "ok"}, "y": {Name: "Y", Status: "error", Reason: "empty"}}
	s.Observe("id", "one", "template", values, time.Now())
	template.Name = "mutated"
	values["x"] = Metric{}
	v, _ := s.Get("id")
	if v.LatestSession.ConfigTemplate.Name != "T" || v.LatestSession.Telemetry.Template["x"].Value != "first" || v.LatestSession.Telemetry.Template["y"].Reason != "empty" {
		t.Fatal(v)
	}
	v.LatestSession.ConfigTemplate.Name = "again"
	v.LatestSession.Telemetry.Template["x"] = Metric{}
	s.Publish(Registration{DeviceID: "id"}, "two", time.Now())
	history, _ := s.Sessions("id")
	if history.Ended[0].ConfigTemplate.Name != "T" || history.Ended[0].Telemetry.Template["x"].Value != "first" || history.Current.ConfigTemplate != nil {
		t.Fatal(history)
	}
}
