package management

import (
	"errors"
	"routerprobe/internal/device"
	"routerprobe/internal/enrollment"
	"routerprobe/internal/probetemplate"
	"testing"
)

func TestInterfaceOverridesAndExplicitTemplateGenerations(t *testing.T) {
	app, err := New(Config{RepositoryDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	template, err := app.ProbeTemplates().Put("", 0, probetemplate.Input{Name: "采样", Monitoring: &probetemplate.Monitoring{CPU: 7, Memory: 9, Disk: 60, Network: 5, Egress: 600}})
	if err != nil {
		t.Fatal(err)
	}
	if err = app.Enrollment().Discover(device.Registration{DeviceID: "r"}); err != nil {
		t.Fatal(err)
	}
	q := DeviceUpdate{Version: 1, Admission: "managed", Name: "路由器", TemplateID: template.ID}
	p, err := app.UpdateDevice("r", q)
	if err != nil {
		t.Fatal(err)
	}
	firstGeneration, firstRevision := p.Configuration.TemplateGeneration, p.Configuration.Revision
	seconds, names := uint32(2), "eth0,br0"
	q.Version = p.Version
	q.InterfaceSampling = &enrollment.InterfaceSampling{NetworkSeconds: &seconds, NetworkInterfaces: &names}
	p, err = app.UpdateDevice("r", q)
	if err != nil {
		t.Fatal(err)
	}
	m := p.Configuration.Template.Monitoring
	if m.CPU != 7 || m.Memory != 9 || m.Disk != 60 || m.Egress != 600 || m.Network != 2 || *m.NetworkInterfaces != names || p.Configuration.TemplateGeneration != firstGeneration || p.Configuration.Revision != firstRevision+1 {
		t.Fatal("interface update changed another plan", p)
	}
	q.Version, q.Name = p.Version, "新名称"
	renamed, err := app.UpdateDevice("r", q)
	if err != nil || renamed.Configuration.Revision != p.Configuration.Revision {
		t.Fatal("rename changed collection", renamed, err)
	}
	q.Version, q.ApplyTemplate = renamed.Version, true
	reapplied, err := app.UpdateDevice("r", q)
	if err != nil || reapplied.BoundTemplate.Version != template.Version || reapplied.Configuration.TemplateGeneration != firstGeneration+1 || reapplied.Configuration.Revision != p.Configuration.Revision+1 || reapplied.Configuration.Template.Monitoring.Network != 2 {
		t.Fatal("same-version reapply did not advance generation or retain interfaces", reapplied, err)
	}
	q.Version, q.ApplyTemplate, q.Monitoring = reapplied.Version, false, &probetemplate.Monitoring{CPU: 1}
	if _, err = app.UpdateDevice("r", q); !errors.Is(err, probetemplate.ErrInvalid) {
		t.Fatal("non-interface override accepted", err)
	}
	q.Monitoring = nil
	q.PropertyIntervals = map[string]uint32{"signal": 1}
	if _, err = app.UpdateDevice("r", q); !errors.Is(err, probetemplate.ErrInvalid) {
		t.Fatal("property interval override accepted", err)
	}
	q.PropertyIntervals = nil
	q.InterfaceSampling = nil
	reset, err := app.UpdateDevice("r", q)
	if err != nil || reset.Configuration.Template.Monitoring.Network != 5 || reset.Configuration.TemplateGeneration != reapplied.Configuration.TemplateGeneration {
		t.Fatal("interface defaults changed template generation", reset, err)
	}
}

func TestLegacyNonInterfaceOverridesRemainReadOnlyUntilApply(t *testing.T) {
	app, err := New(Config{RepositoryDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.Enrollment().Discover(device.Registration{DeviceID: "r"})
	p, err := app.UpdateDevice("r", DeviceUpdate{Version: 1, Admission: "managed", Name: "r"})
	if err != nil {
		t.Fatal(err)
	}
	p.Monitoring = &probetemplate.Monitoring{CPU: 17, Memory: 19, Disk: 61, Network: 5, Egress: 0}
	p.Configuration.Template.Monitoring = p.Monitoring
	p, err = app.Enrollment().Put(p, p.Version)
	if err != nil {
		t.Fatal(err)
	}
	q := DeviceUpdate{Version: p.Version, Admission: "managed", Name: "renamed"}
	p, err = app.UpdateDevice("r", q)
	if err != nil || p.Configuration.Template.Monitoring.CPU != 17 {
		t.Fatal("rename silently cleared existing settings", p, err)
	}
	q.Version, q.ApplyTemplate = p.Version, true
	p, err = app.UpdateDevice("r", q)
	if err != nil || p.Monitoring != nil || p.Configuration.Template.Monitoring != nil {
		t.Fatal("explicit apply did not restore builtin defaults", p, err)
	}
}
