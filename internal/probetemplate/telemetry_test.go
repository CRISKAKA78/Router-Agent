package probetemplate

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestMonitoringDefaultsAndPersistence(t *testing.T) {
	var in Input
	if e := json.Unmarshal([]byte(`{"name":"m","monitoring":{"cpu_seconds":1},"properties":{"cpu_usage":{"name":"CPU","command":"printf 42","interval_seconds":7}}}`), &in); e != nil {
		t.Fatal(e)
	}
	if in.Monitoring.Memory != 5 || in.Monitoring.Disk != 60 || in.Monitoring.Network != 5 {
		t.Fatal(in.Monitoring)
	}
	path := filepath.Join(t.TempDir(), "templates.json")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Put("", 0, in)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	v, e = s.Resolve(v.ID, "")
	if e != nil || v.Monitoring.CPU != 1 || v.Properties["cpu_usage"].Interval != 7 {
		t.Fatal(v, e)
	}
	v.Monitoring.CPU = 999
	again, _ := s.Resolve(v.ID, "")
	if again.Monitoring.CPU != 1 {
		t.Fatal("monitoring aliased")
	}
	for _, bad := range []string{`{"name":"m","monitoring":{"cpu_seconds":null}}`, `{"name":"m","properties":{"cpu_usage":{"name":"CPU","command":"x","interval_seconds":null}}}`} {
		if json.Unmarshal([]byte(bad), &in) == nil {
			t.Fatal("accepted null", bad)
		}
	}
}
