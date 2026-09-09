package api

import (
	"encoding/json"
	"net"
	"routerprobe/internal/protocol"
	"testing"
	"time"
)

func templateFrame(t *testing.T, address string, payload string, flags uint16) protocol.Frame {
	t.Helper()
	c, e := net.Dial("tcp", address)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	wire, e := protocol.EncodeFrame(protocol.Frame{Header: protocol.Header{Version: 1, Type: 0x05, Flags: flags, MessageID: 1}, Payload: []byte(payload)})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Write(wire); e != nil {
		t.Fatal(e)
	}
	d := protocol.NewDecoder(1 << 20)
	b := make([]byte, 1024)
	for {
		n, e := c.Read(b)
		if e != nil {
			t.Fatal(e)
		}
		frames, e := d.Feed(b[:n])
		if e != nil {
			t.Fatal(e)
		}
		if len(frames) > 0 {
			return frames[0]
		}
	}
}
func TestTemplateAPIAndRetiredPreparationRejected(t *testing.T) {
	_, app, url, control := fixture(t, Config{})
	body := `{"name":"路由器模板","properties":{"model":{"name":"型号","command":"printf model"},"signal":{"name":"信号","command":"printf 90","timeout_seconds":3}}}`
	v := data(request(t, url, "POST", "/api/v1/probe-templates", "create-template", body, 201))
	id := v["template_id"].(string)
	replay := data(request(t, url, "POST", "/api/v1/probe-templates", "create-template", body, 201))
	if replay["template_id"] != id {
		t.Fatal("idempotency")
	}
	request(t, url, "POST", "/api/v1/probe-templates", "duplicate-template", body, 409)
	f := templateFrame(t, control, `{"name":"retired"}`, 0)
	var retired object
	if json.Unmarshal(f.Payload, &retired) != nil || f.Header.Type != protocol.TypeError || retired["code"] != "UNSUPPORTED_TYPE" {
		t.Fatal(string(f.Payload))
	}
	if len(app.Devices().List()) != 0 {
		t.Fatal("template connection created device")
	}
	request(t, url, "GET", "/api/v1/probe-templates/"+id, "", "", 200)
	updated := `{"version":1,"name":"新版","properties":{"model":{"name":"型号","command":"printf new"}}}`
	request(t, url, "PUT", "/api/v1/probe-templates/"+id, "update-template", updated, 200)
	request(t, url, "PUT", "/api/v1/probe-templates/"+id, "stale-template", updated, 409)
	request(t, url, "DELETE", "/api/v1/probe-templates/"+id, "delete-template", `{"version":2}`, 200)
	request(t, url, "DELETE", "/api/v1/probe-templates/"+id, "delete-template", `{"version":2}`, 200)
	request(t, url, "GET", "/api/v1/probe-templates/"+id, "", "", 404)
	request(t, url, "POST", "/api/v1/probe-templates", "bad-template", `{"name":"x","properties":{"device_id":{"name":"ID","command":"true"}}}`, 400)
}

func TestPropertyVisibilityAPI(t *testing.T) {
	_, _, url, _ := fixture(t, Config{})
	request(t, url, "POST", "/api/v1/probe-templates", "removed-alias", `{"name":"Removed","properties":{},"presentation":{"interface_aliases":{"eth0":"LAN"}}}`, 400)
	body := `{"name":"Visible","properties":{},"monitoring":{"disk_seconds":60},"presentation":{"builtin_visibility":{"disk":false,"network":false},"fields":{"device_id":{"visible":false},"net_aa_rx_bytes":{"visible":true}}}}`
	created := data(request(t, url, "POST", "/api/v1/probe-templates", "visibility", body, 201))
	saved := data(request(t, url, "GET", "/api/v1/probe-templates/"+created["template_id"].(string), "", "", 200))
	p := saved["presentation"].(map[string]any)
	if p["builtin_visibility"].(map[string]any)["disk"] != false || p["fields"].(map[string]any)["net_aa_rx_bytes"].(map[string]any)["visible"] != true || saved["monitoring"].(map[string]any)["disk_seconds"] != float64(60) {
		t.Fatal(saved)
	}
	request(t, url, "POST", "/api/v1/probe-templates", "bad-category", `{"name":"Bad","properties":{},"presentation":{"builtin_visibility":{"unrecognized":false}}}`, 400)
}
