package gateway

import (
	"strings"
	"testing"
)

const cellularWire = `{"event":"cellular","config_revision":1,"interval_seconds":30,"age_ms":0,"status":"ok","reason":"","limited":false,"ports":[{"path":"/dev/ttyUSB2","device_key":"/sys/devices/platform/usb1/1-1","status":"ok","reason":"","selected":true,"age_ms":12,"ati":{"command":"ATI","status":"ok","value":"Generic modem"},"imei":{"command":"AT+CGSN","status":"ok","value":"867123456789012"}}]}`

func TestCellularWireValidation(t *testing.T) {
	if _, _, e := parseCellular([]byte(cellularWire)); e != nil {
		t.Fatal(e)
	}
	for _, change := range [][2]string{{`"config_revision":1`, `"config_revision":0`}, {`"interval_seconds":30`, `"interval_seconds":1`}, {`/dev/ttyUSB2`, `/dev/ttyS0`}, {`/sys/devices/platform/usb1/1-1`, `../../etc/passwd`}, {`"selected":true`, `"selected":false`}, {`867123456789012`, `serial`}, {`AT+CGSN`, `ATZ`}, {`"ports":[`, `"sampled_at":"2026-09-11T00:00:00Z","ports":[`}, {`"limited":false`, `"limited":null`}, {`"event":"cellular"`, `"event":"cellular","event":"cellular"`}} {
		raw := strings.Replace(cellularWire, change[0], change[1], 1)
		if _, _, e := parseCellular([]byte(raw)); e == nil {
			t.Fatal("accepted", raw)
		}
	}
	for _, state := range []string{"no_ports", "error", "unavailable"} {
		raw := `{"event":"cellular","config_revision":1,"interval_seconds":30,"age_ms":0,"status":"` + state + `","reason":"","limited":false,"ports":[]}`
		if _, _, e := parseCellular([]byte(raw)); e != nil {
			t.Fatal(state, e)
		}
	}
}
