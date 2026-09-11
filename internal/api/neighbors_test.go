package api

import (
	"fmt"
	"testing"
)

func TestNeighborAPIUnsupportedAndInvalidRanges(t *testing.T) {
	_, _, base, control := fixture(t, Config{})
	register(t, control, "old-neighbor-probe")
	v := request(t, base, "GET", "/api/v1/devices/old-neighbor-probe/neighbors", "", "", 200)
	if v["data"].(map[string]any)["snapshot"] != nil {
		t.Fatal("old probe invented neighbors")
	}
	path := "/api/v1/devices/old-neighbor-probe/neighbor-scans"
	request(t, base, "POST", path, "unsupported", `{"domain_id":"local","cidr":"192.0.2.0/24","config_revision":1}`, 422)
	for i, body := range []string{`{"domain_id":"local","cidr":"192.0.0.0/16","config_revision":1}`, `{"domain_id":"local","cidr":"2001:db8::/64","config_revision":1}`, `{"domain_id":"local","cidr":"192.0.2.0/24","config_revision":0}`, `{"domain_id":"local","cidr":"192.0.2.0/24","config_revision":1,"command":"anything"}`} {
		request(t, base, "POST", path, fmt.Sprint("bad-", i), body, 400)
	}
}

func TestNeighborCapabilitiesAndFieldErrors(t *testing.T) {
	_, _, base, _ := fixture(t, Config{})
	c := request(t, base, "GET", "/api/v1/capabilities", "", "", 200)
	caps := c["data"].(map[string]any)["capabilities"].([]any)
	if len(caps) < 3 || caps[0] != "neighbor_probe" {
		t.Fatal(c)
	}
	v := request(t, base, "POST", "/api/v1/probe-templates", "bad-interval", `{"name":"neighbor","properties":{},"neighbor_probe":{"interval_seconds":1,"domains":[{"id":"local","scope":"broadcast","interface":"br0"}]}}`, 400)
	e := v["error"].(map[string]any)
	if e["code"] != "invalid_request" || e["field"] != "neighbor_probe.interval_seconds" || e["details"] == "" {
		t.Fatal(v)
	}
}
