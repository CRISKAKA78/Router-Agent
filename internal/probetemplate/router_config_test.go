package probetemplate

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestConfigSourcesPersistAndRejectWrites(t *testing.T) {
	for _, raw := range []string{
		`{"name":"x","source":"nvram","key":"SN","command":""}`,
		`{"name":"x","command":"true","key":""}`,
		`{"name":"x","source":"uci","key":"system.main.hostname","operation":"set"}`,
	} {
		var property Property
		if json.Unmarshal([]byte(raw), &property) == nil {
			t.Fatal("mixed/unknown property accepted", raw)
		}
	}
	path := filepath.Join(t.TempDir(), "templates.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	in := Input{Name: "config", Properties: map[string]Property{
		"serial":   {Name: "序列号", Source: "nvram", Key: "SN"},
		"hostname": {Name: "主机名", Source: "uci", Key: "system.@system[0].hostname"},
		"kernel":   {Name: "内核", Command: "uname -r"},
	}}
	v, err := s.Put("", 0, in)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Resolve(v.ID, "")
	if err != nil || got.Properties["serial"].Source != "nvram" || got.Properties["hostname"].Key != in.Properties["hostname"].Key || got.Properties["kernel"].Command != "uname -r" {
		t.Fatal(got, err)
	}
	for _, p := range []Property{
		{Name: "x", Source: "uci", Key: "network.lan"},
		{Name: "x", Source: "nvram", Key: "SN", Command: "nvram set SN=bad"},
		{Name: "x", Source: "uci", Key: "system.@system[0].hostname", Command: "true"},
		{Name: "x", Source: "command", Key: "SN", Command: "true"},
		{Name: "x", Source: "nvram", Key: "SN;reboot"},
		{Name: "x", Source: "set", Key: "SN"},
	} {
		if _, err = s.Put("", 0, Input{Name: "invalid", Properties: map[string]Property{"custom": p}}); err == nil {
			t.Fatal("invalid source accepted", p)
		}
	}
}
