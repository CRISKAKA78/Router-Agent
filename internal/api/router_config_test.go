package api

import (
	"fmt"
	"testing"
)

func TestRouterConfigValidationAndOldProbe(t *testing.T) {
	_, _, base, control := fixture(t, Config{})
	register(t, control, "old-config-probe")
	path := "/api/v1/devices/old-config-probe/config-tasks"
	v := request(t, base, "POST", path, "unsupported", `{"backend":"nvram","operation":"get","key":"SN"}`, 422)
	if v["error"].(map[string]any)["code"] != "unsupported_capability" {
		t.Fatal(v)
	}
	for i, b := range []string{
		`{"backend":"uci","operation":"delete","key":"network"}`,
		`{"backend":"nvram","operation":"set","key":"SN"}`,
		`{"backend":"nvram","operation":"get","key":"SN","value":""}`,
		`{"backend":"nvram","operation":"get","key":"SN","value":null}`,
		`{"backend":"nvram","operation":"get","key":"SN","timeout_seconds":0}`,
		`{"backend":"nvram","operation":"get","key":"SN","timeout_seconds":31}`,
		`{"backend":"nvram","operation":"get","key":"SN","timeout_seconds":null}`,
		`{"backend":"nvram","operation":"get","key":"SN","command":"reboot"}`,
		`{"backend":"nvram","backend":"uci","operation":"commit"}`,
	} {
		request(t, base, "POST", path, fmt.Sprint("invalid-", i), b, 400)
	}
	request(t, base, "POST", "/api/v1/devices/missing/config-tasks", "missing", `{"backend":"nvram","operation":"commit"}`, 404)
}
