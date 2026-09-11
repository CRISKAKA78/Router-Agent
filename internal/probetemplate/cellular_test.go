package probetemplate

import (
	"encoding/json"
	"testing"
)

func TestCellularConfigRoundTrip(t *testing.T) {
	var c CellularProbe
	if e := json.Unmarshal([]byte(`{}`), &c); e != nil || c.Interval != 30 {
		t.Fatal(c, e)
	}
	for _, s := range []string{`null`, `{"interval_seconds":null}`, `{"interval_seconds":0}`, `{"interval_seconds":9}`, `{"interval_seconds":86401}`, `{"interval_seconds":1.5}`, `{"command":"ATZ"}`, `{"port":"/dev/ttyS0"}`} {
		if json.Unmarshal([]byte(s), &c) == nil {
			t.Fatal("accepted", s)
		}
	}
	path := t.TempDir() + "/templates.json"
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	in := Input{Name: "AT only", Properties: map[string]Property{}, CellularProbe: &CellularProbe{Interval: 30}}
	v, e := s.Put("", 0, in)
	if e != nil {
		t.Fatal(e)
	}
	in.CellularProbe.Interval = 90
	if v.CellularProbe.Interval != 30 {
		t.Fatal("input alias")
	}
	id := v.ID
	v.CellularProbe.Interval = 70
	loaded, e := s.Resolve(id, "")
	if e != nil || loaded.CellularProbe.Interval != 30 {
		t.Fatal("result alias", e)
	}
	s.Close()
	s, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	loaded, e = s.Resolve(id, "")
	if e != nil || loaded.CellularProbe.Interval != 30 {
		t.Fatal("persistence", e)
	}
}
