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
	wire, e := protocol.EncodeFrame(protocol.Frame{Header: protocol.Header{Version: 1, Type: protocol.TypeTemplateGet, Flags: flags, MessageID: 1}, Payload: []byte(payload)})
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
func TestTemplateAPIAndPreparationConnection(t *testing.T) {
	_, app, url, control := fixture(t, Config{})
	body := `{"name":"路由器模板","properties":{"model":{"name":"型号","command":"printf model"},"signal":{"name":"信号","command":"printf 90","timeout_seconds":3}}}`
	v := data(request(t, url, "POST", "/api/v1/probe-templates", "create-template", body, 201))
	id := v["template_id"].(string)
	replay := data(request(t, url, "POST", "/api/v1/probe-templates", "create-template", body, 201))
	if replay["template_id"] != id {
		t.Fatal("idempotency")
	}
	request(t, url, "POST", "/api/v1/probe-templates", "duplicate-template", body, 409)
	for _, selector := range []string{`{"name":"路由器模板"}`, `{"template_id":"` + id + `"}`} {
		f := templateFrame(t, control, selector, 0)
		var reply object
		if json.Unmarshal(f.Payload, &reply) != nil || f.Header.Type != protocol.TypeTemplateReply || f.Header.Flags != protocol.FlagResponse || reply["success"] != true {
			t.Fatal(f, string(f.Payload))
		}
		if reply["template"].(map[string]any)["template_id"] != id {
			t.Fatal(reply)
		}
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
	for _, selector := range []string{`{}`, `{"name":"missing"}`, `{"name":"x","template_id":"y"}`, `{"name":"x","name":"y"}`} {
		f := templateFrame(t, control, selector, 0)
		var r object
		json.Unmarshal(f.Payload, &r)
		if r["success"] != false {
			t.Fatal(r)
		}
	}
	request(t, url, "POST", "/api/v1/probe-templates", "bad-template", `{"name":"x","properties":{"device_id":{"name":"ID","command":"true"}}}`, 400)
}
