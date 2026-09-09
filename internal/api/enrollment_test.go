package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"routerprobe/internal/protocol"
	"testing"
	"time"
)

func TestAdmissionPoolProfilesAndModelMatching(t *testing.T) {
	_, app, base, control := fixture(t, Config{})
	c, _ := register(t, control, "pending-router", true)
	eventually(t, func() bool { _, e := app.Enrollment().Get("pending-router"); return e == nil })
	if data(request(t, base, "GET", "/api/v1/devices", "", "", 200))["total"] != float64(0) {
		t.Fatal("pending leaked into managed list")
	}
	if data(request(t, base, "GET", "/api/v1/discoveries", "", "", 200))["total"] != float64(1) {
		t.Fatal("missing discovery")
	}
	request(t, base, "POST", "/api/v1/tasks", "pending-exec", `{"device_id":"pending-router","command":"true","timeout_seconds":5}`, 409)
	request(t, base, "POST", "/api/v1/maintenance", "pending-maintenance", `{"device_id":"pending-router"}`, 409)
	ignore := `{"version":1,"admission":"ignored","name":"候选设备","model_id":"","template_id":""}`
	request(t, base, "PUT", "/api/v1/devices/pending-router/profile", "ignore", ignore, 200)
	request(t, base, "PUT", "/api/v1/devices/pending-router/profile", "ignore", ignore, 200)
	request(t, base, "PUT", "/api/v1/devices/pending-router/profile", "stale", ignore, 409)
	c.Close()
	register(t, control, "pending-router", true)
	if data(request(t, base, "GET", "/api/v1/discoveries?admission=ignored", "", "", 200))["total"] != float64(1) {
		t.Fatal("reconnect reset ignored status")
	}
	request(t, base, "PUT", "/api/v1/devices/pending-router/profile", "recover", `{"version":2,"admission":"pending","name":"候选设备","model_id":"","template_id":""}`, 200)
	tpl := data(request(t, base, "POST", "/api/v1/probe-templates", "template", `{"name":"默认模板","properties":{"signal":{"name":"信号","command":"printf 90","interval_seconds":7}}}`, 201))
	id := tpl["template_id"].(string)
	request(t, base, "PUT", "/api/v1/device-models/router-a", "model", `{"version":0,"name":"Router A","aliases":["board-a"],"template_id":"`+id+`"}`, 200)
	matched := data(request(t, base, "GET", "/api/v1/device-models/match?name=%20BOARD-A%20", "", "", 200))
	if matched["model_id"] != "router-a" || matched["template_id"] != id {
		t.Fatal(matched)
	}
	request(t, base, "PUT", "/api/v1/device-models/router-b", "alias-conflict", `{"version":0,"name":"Router B","aliases":["board-a"],"template_id":""}`, 409)
	request(t, base, "DELETE", "/api/v1/probe-templates/"+id, "referenced", `{"version":1}`, 409)
	request(t, base, "PUT", "/api/v1/devices/pending-router/profile", "adopt", `{"version":3,"admission":"managed","name":"管理员名称","model_id":"router-a","template_id":"`+id+`"}`, 200)
	d := data(request(t, base, "GET", "/api/v1/devices/pending-router", "", "", 200))
	p := d["profile"].(map[string]any)
	if p["name"] != "管理员名称" || p["configuration_state"] != "waiting_confirmation" || d["registration"].(map[string]any)["model"] != "" {
		t.Fatal(d)
	}
	b, _ := json.Marshal(d)
	if bytes.Contains(b, []byte("printf 90")) {
		t.Fatal("device DTO exposes commands")
	}
	property := p["bound_template"].(map[string]any)["properties"].(map[string]any)["signal"].(map[string]any)
	if property["interval_seconds"] != float64(7) {
		t.Fatal("sampling form metadata dropped", property)
	}
	if data(request(t, base, "GET", "/api/v1/devices", "", "", 200))["total"] != float64(1) {
		t.Fatal("managed missing")
	}
}

func readManagedFrame(t *testing.T, c net.Conn) protocol.Frame {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(13 * time.Second))
	header := make([]byte, protocol.HeaderSize)
	if _, e := io.ReadFull(c, header); e != nil {
		t.Fatal(e)
	}
	h, e := protocol.DecodeHeader(header, 1<<20)
	if e != nil {
		t.Fatal(e)
	}
	b := make([]byte, h.PayloadLen)
	if _, e = io.ReadFull(c, b); e != nil {
		t.Fatal(e)
	}
	return protocol.Frame{Header: h, Payload: b}
}
func TestConfigurationConfirmationRetryAndRevisionIsolation(t *testing.T) {
	_, app, base, control := fixture(t, Config{})
	c, e := net.Dial("tcp", control)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	var message uint64
	send := func(kind uint8, flags uint16, body any) {
		t.Helper()
		message++
		b, _ := json.Marshal(body)
		if e := protocol.WriteFrame(c, protocol.Frame{Header: protocol.Header{Version: 1, Type: kind, Flags: flags, MessageID: message}, Payload: b}); e != nil {
			t.Fatal(e)
		}
	}
	send(protocol.TypeRegister, 0, object{"device_id": "configured", "probe_version": "test", "arch": "x86_64", "boot_id": "b", "capabilities": []string{"exec", "telemetry_v2", "managed_config_v1"}})
	registration := readManagedFrame(t, c)
	if !bytes.Contains(registration.Payload, []byte(`"managed_config_v1":true`)) {
		t.Fatal("capability not negotiated")
	}
	eventually(t, func() bool { _, e := app.Enrollment().Get("configured"); return e == nil })
	adoptHTTP(t, base, "configured")
	first := readManagedFrame(t, c)
	if first.Header.Type != protocol.TypeConfigApply {
		t.Fatal(first)
	}
	retry := readManagedFrame(t, c)
	if retry.Header.Type != protocol.TypeConfigApply || !bytes.Equal(first.Payload, retry.Payload) || retry.Header.MessageID == first.Header.MessageID {
		t.Fatal("ACK loss must resend exact configuration with new correlation")
	}
	send(protocol.TypeConfigAck, protocol.FlagResponse, object{"reply_to": first.Header.MessageID, "revision": 1, "success": true, "error": ""})
	send(protocol.TypeHeartbeat, 0, object{"uptime": 1,"uptime_valid":true, "running_tasks": 0})
	readManagedFrame(t, c)
	d, _ := app.Devices().Get("configured")
	if d.LatestSession.ConfigRevision != 0 {
		t.Fatal("obsolete ACK applied")
	}
	send(protocol.TypeConfigAck, protocol.FlagResponse, object{"reply_to": retry.Header.MessageID, "revision": 1, "success": true, "error": ""})
	eventually(t, func() bool { d, _ := app.Devices().Get("configured"); return d.LatestSession.ConfigRevision == 1 })
	event := func(revision uint64, value string) {
		send(protocol.TypeEvent, 0, object{"event": "telemetry", "group": "cpu", "config_revision": revision, "values": object{"cpu_usage": object{"name": "CPU", "value": value, "unit": "percent", "status": "ok", "interval_seconds": 5, "age_ms": 0}}})
	}
	event(1, "20")
	event(0, "99")
	send(protocol.TypeHeartbeat, 0, object{"uptime": 2,"uptime_valid":true, "running_tasks": 0})
	readManagedFrame(t, c)
	d, _ = app.Devices().Get("configured")
	if d.LatestSession.Telemetry.Groups["cpu"]["cpu_usage"].Value != "20" {
		t.Fatal("old configuration telemetry overwrote current observation")
	}
}
