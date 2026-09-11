package api

import (
	"testing"
)

func TestCellularAPIAndTemplate(t *testing.T) {
	_, _, base, control := fixture(t, Config{})
	register(t, control, "plain-probe")
	v := request(t, base, "GET", "/api/v1/devices/plain-probe/cellular", "", "", 200)
	if v["data"].(map[string]any)["snapshot"] != nil {
		t.Fatal("invented identity")
	}
	request(t, base, "GET", "/api/v1/devices/missing/cellular", "", "", 404)
	c := request(t, base, "GET", "/api/v1/capabilities", "", "", 200)
	found := false
	for _, v := range c["data"].(map[string]any)["capabilities"].([]any) {
		found = found || v == "cellular_identity_v1"
	}
	if !found {
		t.Fatal("missing capability")
	}
	v = request(t, base, "POST", "/api/v1/probe-templates", "at-template", `{"name":"AT","properties":{},"cellular_probe":{"interval_seconds":30}}`, 201)
	if v["data"].(map[string]any)["cellular_probe"].(map[string]any)["interval_seconds"] != float64(30) {
		t.Fatal(v)
	}
	request(t, base, "POST", "/api/v1/probe-templates", "at-invalid", `{"name":"AT invalid","properties":{},"cellular_probe":{"command":"ATZ"}}`, 400)
}
