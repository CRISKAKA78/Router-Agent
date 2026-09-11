package gateway

import (
	"encoding/json"
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

func TestCellularTelemetryWireValidation(t *testing.T) {
	var obj map[string]any
	_ = json.Unmarshal([]byte(cellularWire), &obj)
	obj["event"] = "cellular_telemetry"
	port := obj["ports"].([]any)[0].(map[string]any)
	port["profile"] = "fibocom-fm160-v1"
	port["ati"].(map[string]any)["value"] = "Manufacturer: Fibocom Wireless Inc.\nModel: FM160-CN"
	queries := []any{}
	for _, c := range []string{"AT+CPIN?", "AT+CCID", "AT+CIMI", "AT+COPS?", "AT+CEREG?", "AT+C5GREG?", "AT+CSQ", "AT+CESQ"} {
		queries = append(queries, map[string]any{"command": c, "status": "not_queried", "value": ""})
	}
	port["queries"] = queries
	raw, _ := json.Marshal(obj)
	n, _, e := parseCellular(raw)
	if e != nil || !n.Telemetry {
		t.Fatal(e)
	}
	for _, bad := range [][]byte{[]byte(strings.Replace(string(raw), "AT+CSQ", "ATZ", 1)), []byte(strings.Replace(string(raw), "FM160-CN", "FM650", 1)), []byte(strings.Replace(string(raw), "cellular_telemetry", "cellular", 1))} {
		if _, _, e := parseCellular(bad); e == nil {
			t.Fatal("accepted invalid extension")
		}
	}
	port["signals"] = []any{}
	raw, _ = json.Marshal(obj)
	if _, _, e := parseCellular(raw); e == nil {
		t.Fatal("probe injected derived metrics")
	}
}
func TestCellularDetailsWire(t *testing.T) {
	var obj map[string]any
	_ = json.Unmarshal([]byte(cellularWire), &obj)
	obj["event"] = "cellular_details"
	p := obj["ports"].([]any)[0].(map[string]any)
	p["profile"] = "fibocom-fm160-details-v1"
	p["ati"].(map[string]any)["value"] = "Manufacturer: Fibocom Wireless Inc.\nModel: FM160-CN"
	queries := []any{}
	for _, cmd := range []string{"AT+CPIN?", "AT+CCID", "AT+CIMI", "AT+COPS?", "AT+CEREG?", "AT+C5GREG?", "AT+CSQ", "AT+CESQ", "AT+CBC", "AT+MTSM?", "AT+MTSM=1", "AT+MTSM=6", "AT+MTSM=7", "AT+CGATT?", "AT+CGACT?", "AT+CGDCONT?", "AT+CGPADDR", "AT+CGCONTRDP", "AT+GTACT?", "AT+GTACT=?", "AT+GTCELLLOCK?", "AT+GTCAINFO?", "AT+GTCELLINFO?", "AT+GTCCINFO?"} {
		queries = append(queries, map[string]any{"command": cmd, "status": "not_queried", "value": ""})
	}
	p["queries"] = queries
	b, _ := json.Marshal(obj)
	n, _, e := parseCellular(b)
	if e != nil || !n.Details || !n.Telemetry {
		t.Fatal(n, e)
	}
	for _, pair := range [][2]string{{"cellular_details", "cellular_telemetry"}, {"AT+GTCELLLOCK?", "AT+GTCELLLOCK=1"}, {"AT+GTACT?", "AT+GTACT=14"}, {"AT+MTSM=1", "AT+MTSM=2"}, {"AT+GTCELLINFO?", "AT+GTCELLINFO=1"}} {
		if _, _, e := parseCellular([]byte(strings.Replace(string(b), pair[0], pair[1], 1))); e == nil {
			t.Fatal("accepted write/mismatched version", pair)
		}
	}
}
