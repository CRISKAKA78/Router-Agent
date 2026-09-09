package gateway

import "testing"

func TestPhysicalCounterTelemetryExactUint64(t *testing.T) {
	for _, value := range []string{"9007199254740993", "18446744073709551615"} {
		group, m, e := parseTelemetry([]byte(`{"event":"telemetry","group":"switch","values":{"switch_lan1_rx_raw_bytes":{"name":"raw RX","unit":"bytes","status":"ok","value":"` + value + `"}}}`))
		if e != nil || group != "switch" || m["switch_lan1_rx_raw_bytes"].Value != value {
			t.Fatalf("raw integer lost: %v %v", m, e)
		}
	}
	if _, _, e := parseTelemetry([]byte(`{"event":"telemetry","group":"switch","values":{"switch_lan1_rx_raw_bytes":{"name":"raw RX","unit":"bytes","status":"ok","value":"18446744073709551616"}}}`)); e == nil {
		t.Fatal("overflow accepted")
	}
}
