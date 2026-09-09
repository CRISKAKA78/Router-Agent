package management

import (
	"routerprobe/internal/device"
	"routerprobe/internal/enrollment"
	"routerprobe/internal/probetemplate"
	"testing"
)

func TestOfflineConfigurationAndPinnedTemplateSurviveRestart(t *testing.T) {
	config := Config{RepositoryDirectory: t.TempDir()}
	app, e := New(config)
	if e != nil {
		t.Fatal(e)
	}
	template, e := app.ProbeTemplates().Put("", 0, probetemplate.Input{Name: "布局模板", Monitoring: &probetemplate.Monitoring{CPU: 5}, Presentation: &probetemplate.Presentation{Groups: []probetemplate.DisplayGroup{{ID: "basic", Name: "基本信息", Order: 10}}}})
	if e != nil {
		t.Fatal(e)
	}
	if e = app.Enrollment().Discover(device.Registration{DeviceID: "r", Hostname: "raw"}); e != nil {
		t.Fatal(e)
	}
	if _, e = app.PutModel(enrollment.Model{ID: "m", Name: "Router", Aliases: []string{"board"}, TemplateID: template.ID}); e != nil {
		t.Fatal(e)
	}
	seconds := uint32(3)
	p, e := app.UpdateDevice("r", DeviceUpdate{Version: 1, Admission: "managed", Name: "管理员名称", ModelID: "m", TemplateID: template.ID, InterfaceSampling: &enrollment.InterfaceSampling{NetworkSeconds: &seconds}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = app.ProbeTemplates().Put(template.ID, 1, probetemplate.Input{Name: template.Name, Monitoring: &probetemplate.Monitoring{CPU: 10}}); e != nil {
		t.Fatal(e)
	}
	app.Close()
	app, e = New(config)
	if e != nil {
		t.Fatal(e)
	}
	defer app.Close()
	restored, e := app.Enrollment().Get("r")
	if e != nil {
		t.Fatal(e)
	}
	if restored.Configuration.Revision != p.Configuration.Revision || restored.Configuration.Template.Monitoring.Network != 3 || restored.Configuration.Template.Monitoring.CPU != 5 || restored.BoundTemplate.Version != 1 || restored.BoundTemplate.Monitoring.CPU != 5 || restored.Name != "管理员名称" || restored.Reported.Hostname != "raw" {
		t.Fatalf("lost durable profile or pinned snapshot: %+v", restored)
	}
	inventory := app.Inventory()
	if len(inventory) != 1 || inventory[0].Status != device.Offline || inventory[0].CurrentSession != nil {
		t.Fatal("restart fabricated live session", inventory)
	}
	if m := app.Enrollment().Match("BOARD"); m == nil || m.TemplateID != template.ID {
		t.Fatal("model mapping lost")
	}
	if e = app.DeleteTemplate(template.ID, 2); e == nil {
		t.Fatal("durable template references lost")
	}
}
