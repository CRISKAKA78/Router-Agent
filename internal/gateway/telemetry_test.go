package gateway

import (
	"strings"
	"testing"
)

func TestTelemetryValidation(t *testing.T) {
	good := `{"event":"telemetry","group":"cpu","values":{"cpu_usage":{"name":"CPU","value":"25.50","unit":"percent","status":"ok","interval_seconds":1,"age_ms":20}}}`
	if _, _, e := parseTelemetry([]byte(good)); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(good, "25.50", "101", 1), strings.Replace(good, `"age_ms":20`, `"age_ms":-1`, 1), strings.Replace(good, `"interval_seconds":1`, `"interval_seconds":null`, 1), strings.Replace(good, `"unit":"percent"`, `"unit":"text"`, 1), strings.Replace(good, `"group":"cpu"`, `"group":"fake"`, 1), strings.Replace(good, `"name":"CPU"`, `"name":null`, 1)} {
		if _, _, e := parseTelemetry([]byte(bad)); e == nil {
			t.Fatal("accepted", bad)
		}
	}
}

func TestMonitoringV2PayloadValidation(t *testing.T) {
	good := `{"event":"telemetry","group":"egress","values":{"egress_ipv6":{"name":"IPv6","value":"2001:4860:4860::8888","unit":"text","status":"ok","reason":""}}}`
	if _, _, e := parseTelemetry([]byte(good)); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(good, "2001:4860:4860::8888", "8.8.8.8", 1), strings.Replace(good, `"reason":""`, `"reason":null`, 1), strings.Replace(good, "egress_ipv6", "other_ip", 1)} {
		if _, _, e := parseTelemetry([]byte(bad)); e == nil {
			t.Fatal("accepted", bad)
		}
	}
	traffic := `{"event":"telemetry","group":"network","values":{"net_65746830_elapsed_seconds":{"name":"时长","value":"3600","unit":"seconds","status":"ok"}}}`
	if _, _, e := parseTelemetry([]byte(traffic)); e != nil {
		t.Fatal(e)
	}
	if _, _, e := parseTelemetry([]byte(strings.Replace(traffic, "3600", "-1", 1))); e == nil {
		t.Fatal("negative duration")
	}
}
