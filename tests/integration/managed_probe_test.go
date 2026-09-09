package integration

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http/httptest"
	"routerprobe/internal/api"
	"routerprobe/internal/device"
	"routerprobe/internal/enrollment"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/probetemplate"
	"testing"
	"time"
)

func TestManagedProbeOptionalPhysicalPort(t *testing.T) {
	binary := probeBinary(t)
	app, e := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{Logger: log.New(io.Discard, "", 0)}})
	if e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(listener) }()
	defer func() { app.Close(); <-done }()
	var input probetemplate.Input
	e = json.Unmarshal([]byte(`{"name":"端口","properties":{},"monitoring":{"network_seconds":1},"switch_probe":{"backend":"command","command":"printf 'p1\\t-\\t-\\teth0\\t-\\tup\\tunknown\\t1000\\tfull\\n'","ports":[{"id":"p1","system_name":"eth0","display_name":"LAN1"}]}}`), &input)
	if e != nil {
		t.Fatal(e)
	}
	tpl, e := app.ProbeTemplates().Put("", 0, input)
	if e != nil {
		t.Fatal(e)
	}
	if tpl.SwitchProbe.Ports[0].Port != nil {
		t.Fatal("absent port became zero")
	}
	startProbe(t, binary, listener.Addr().String(), "optional-port", io.Discard)
	adoptProbe(t, app, "optional-port", tpl.ID, nil)
	waitManagedMetric(t, app, "optional-port", func(d device.Snapshot, m map[string]device.Metric) bool {
		return d.LatestSession.ConfigRevision == 1 && m["switch_p1_label"].Value == "LAN1" && m["switch_p1_state"].Value == "up" && m["switch_p1_port"].Status == "unknown"
	})
}

func interfaceSampling(m *probetemplate.Monitoring) *enrollment.InterfaceSampling {
	if m == nil {
		return nil
	}
	return &enrollment.InterfaceSampling{NetworkSeconds: &m.Network, NetworkInterfaces: m.NetworkInterfaces}
}
func adoptProbe(t *testing.T, app *management.Server, id, template string, monitoring *probetemplate.Monitoring) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		p, e := app.Enrollment().Get(id)
		if e == nil {
			_, e = app.UpdateDevice(id, management.DeviceUpdate{Version: p.Version, Admission: "managed", Name: p.Name, TemplateID: template, ApplyTemplate: true, InterfaceSampling: interfaceSampling(monitoring)})
			if e != nil {
				t.Fatal(e)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("discovery deadline", id)
}
func waitManagedMetric(t *testing.T, app *management.Server, id string, check func(device.Snapshot, map[string]device.Metric) bool) device.Snapshot {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		d, e := app.Devices().Get(id)
		if e == nil && check(d, device.EffectiveMetrics(d.LatestSession, time.Now())) {
			return d
		}
		time.Sleep(20 * time.Millisecond)
	}
	d, _ := app.Devices().Get(id)
	t.Fatalf("managed observation deadline: %+v", d.LatestSession)
	return d
}
func TestManagedProbeAdmissionHotConfigurationAndReconnect(t *testing.T) {
	binary := probeBinary(t)
	app, e := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{Logger: log.New(io.Discard, "", 0)}})
	if e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(listener) }()
	adapter, e := api.New(app, api.Config{})
	if e != nil {
		t.Fatal(e)
	}
	http := httptest.NewServer(adapter)
	defer func() { http.Close(); adapter.Close(); app.Close(); <-done }()
	tpl, e := app.ProbeTemplates().Put("", 0, probetemplate.Input{Name: "Managed", Properties: map[string]probetemplate.Property{"signal": {Name: "Signal", Command: "printf 90", Interval: 1}}, Monitoring: &probetemplate.Monitoring{CPU: 1, Memory: 1, Network: 1}, Presentation: &probetemplate.Presentation{Groups: []probetemplate.DisplayGroup{{ID: "basic", Name: "基本信息", Order: 10}}, Fields: map[string]probetemplate.DisplayField{"signal": {GroupID: "basic", Order: 20}}}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = app.PutModel(enrollment.Model{ID: "router", Name: "Router", TemplateID: tpl.ID})
	if e != nil {
		t.Fatal(e)
	}
	startProbe(t, binary, listener.Addr().String(), "managed", io.Discard)
	waitManagedMetric(t, app, "managed", func(d device.Snapshot, m map[string]device.Metric) bool { return d.CurrentSession != nil })
	list := p5request(t, http.URL, "GET", "/devices", "", nil, 200)
	if list["total"] != float64(0) {
		t.Fatal("unadmitted device visible", list)
	}
	p5request(t, http.URL, "POST", "/devices/managed/config-tasks", "denied", p5object{"backend": "nvram", "operation": "get", "key": "SN"}, 409)
	adoptProbe(t, app, "managed", tpl.ID, nil)
	first := waitManagedMetric(t, app, "managed", func(d device.Snapshot, m map[string]device.Metric) bool {
		return d.LatestSession.ConfigRevision == 1 && m["signal"].Value == "90"
	})
	if first.LatestSession.ConfigTemplate.ID != tpl.ID {
		t.Fatal("server template overwrote raw registration")
	}
	if e = app.DeleteTemplate(tpl.ID, tpl.Version); e == nil {
		t.Fatal("referenced template deleted")
	}
	p, _ := app.Enrollment().Get("managed")
	_, e = app.UpdateDevice("managed", management.DeviceUpdate{Version: p.Version, Admission: "managed", Name: "管理员名称", ModelID: "router", TemplateID: tpl.ID, InterfaceSampling: interfaceSampling(&probetemplate.Monitoring{Network: 2})})
	if e != nil {
		t.Fatal(e)
	}
	changed := waitManagedMetric(t, app, "managed", func(d device.Snapshot, m map[string]device.Metric) bool {
		return d.LatestSession.ConfigRevision == 2 && m["signal"].Interval == 1 && m["signal"].Value == "90"
	})
	if changed.LatestSession.ID != first.LatestSession.ID {
		t.Fatal("hot configuration reconnected probe")
	}
	p, _ = app.Enrollment().Get("managed")
	if p.BoundTemplate.Properties["signal"].Interval != 1 {
		t.Fatal("device override mutated bound defaults")
	}
	app.Disconnect("managed")
	waitManagedMetric(t, app, "managed", func(d device.Snapshot, m map[string]device.Metric) bool {
		return d.LatestSession.ID != first.LatestSession.ID && d.LatestSession.ConfigRevision == 2 && m["signal"].Value == "90"
	})
	p, _ = app.Enrollment().Get("managed")
	if p.Name != "管理员名称" || p.Admission != "managed" {
		t.Fatal("reconnect lost admin profile")
	}
}
