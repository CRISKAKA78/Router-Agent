package api

import (
	"encoding/json"
	"fmt"
	"routerprobe/internal/device"
	"testing"
)

func TestDeviceWorkspaceProfileContract(t *testing.T) {
	_, app, base, _ := fixture(t, Config{})
	if err := app.Enrollment().Discover(device.Registration{DeviceID: "workspace"}); err != nil {
		t.Fatal(err)
	}
	template := data(request(t, base, "POST", "/api/v1/probe-templates", "workspace-template", `{"name":"T","monitoring":{"cpu_seconds":7},"presentation":{"fields":{"cpu_usage":{"group_id":"builtin_resources","order":1},"memory_usage":{"group_id":"builtin_system","order":2}}},"properties":{}}`, 201))
	id := template["template_id"].(string)
	body := fmt.Sprintf(`{"version":1,"admission":"managed","name":"R","template_id":%q,"apply_template":true,"interface_sampling":{"network_seconds":3,"network_interfaces":"lo"}}`, id)
	first := data(request(t, base, "PUT", "/api/v1/devices/workspace/profile", "workspace-apply", body, 200))
	same := data(request(t, base, "PUT", "/api/v1/devices/workspace/profile", "workspace-apply", body, 200))
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(same)
	if string(a) != string(b) {
		t.Fatal("idempotent application changed its result")
	}
	p, _ := app.Enrollment().Get("workspace")
	if p.Configuration.TemplateGeneration != 1 || p.Configuration.Revision != 1 || p.Configuration.Template.Monitoring.CPU != 7 {
		t.Fatal(p)
	}
	request(t, base, "PUT", "/api/v1/devices/workspace/profile", "workspace-forbidden", fmt.Sprintf(`{"version":2,"admission":"managed","name":"R","template_id":%q,"monitoring":{"cpu_seconds":1}}`, id), 400)
	body = fmt.Sprintf(`{"version":2,"admission":"managed","name":"R","template_id":%q,"apply_template":true,"interface_sampling":{"network_seconds":3}}`, id)
	request(t, base, "PUT", "/api/v1/devices/workspace/profile", "workspace-reapply", body, 200)
	p, _ = app.Enrollment().Get("workspace")
	if p.Configuration.TemplateGeneration != 2 || p.Configuration.Revision != 2 {
		t.Fatal("same-version apply lost", p)
	}
	d := data(request(t, base, "GET", "/api/v1/devices/workspace", "", "", 200))
	profile := d["profile"].(map[string]any)
	if profile["template_generation"] != float64(2) || profile["configuration_state"] != "waiting_dispatch" || profile["latest_template"].(map[string]any)["version"] != float64(1) || d["active_template"] != nil {
		t.Fatal("desired config was reported as acknowledged", d)
	}
	request(t, base, "POST", "/api/v1/probe-templates", "reserved-group", `{"name":"Bad","presentation":{"groups":[{"id":"builtin_system","name":"X","order":1}]},"properties":{}}`, 400)
	empty := data(request(t, base, "GET", "/api/v1/devices/workspace/connections", "", "", 200))
	if empty["total"] != float64(0) {
		t.Fatal("invented connection history", empty)
	}
	request(t, base, "GET", "/api/v1/devices/missing/connections", "", "", 404)
}
