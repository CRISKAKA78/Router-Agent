package enrollment

import (
	"errors"

	"path/filepath"
	"routerprobe/internal/device"
	"routerprobe/internal/probetemplate"
	"testing"
)

func TestDurableAdmissionAndModelMapping(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Discover(device.Registration{DeviceID: "router", Hostname: "reported", Model: "neutral"}); e != nil {
		t.Fatal(e)
	}
	if !errors.Is(s.RequireManaged("router"), ErrNotManaged) {
		t.Fatal("discovery bypassed admission")
	}
	p, _ := s.Get("router")
	p.Name = "管理员名称"
	p.Admission = "managed"
	if _, e = s.Put(p, p.Version); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Put(p, p.Version); !errors.Is(e, probetemplate.ErrConflict) {
		t.Fatal("lost update allowed", e)
	}
	if e = s.Discover(device.Registration{DeviceID: "router", Hostname: "changed"}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.PutModel(Model{ID: "m", Name: "Router A", Aliases: []string{"a"}, TemplateID: "t"}); e != nil {
		t.Fatal(e)
	}
	if m := s.Match(" A "); m == nil || m.ID != "m" {
		t.Fatal(m)
	}
	if _, e = s.PutModel(Model{ID: "other", Name: "A"}); !errors.Is(e, probetemplate.ErrConflict) {
		t.Fatal("ambiguous model accepted", e)
	}
	if !s.Referenced("t") {
		t.Fatal("model template reference lost")
	}
	s.Close()
	s, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	p, _ = s.Get("router")
	if p.Name != "管理员名称" || p.Admission != "managed" || p.Reported.Hostname != "changed" {
		t.Fatal(p)
	}
	p.Name = "mutated copy"
	copy, _ := s.Get("router")
	if copy.Name == p.Name {
		t.Fatal("query leaked mutable state")
	}
}
