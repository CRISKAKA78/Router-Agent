package api

import (
	"io"
	"log"
	"net/http/httptest"
	"routerprobe/internal/forwarding"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"testing"
)

func TestForwardingPublicUnavailable(t *testing.T) {
	app, e := management.New(management.Config{RepositoryDirectory: t.TempDir(), Forwarding: &forwarding.Config{Binary: "/missing/gost"}, Gateway: gateway.Config{Logger: log.New(io.Discard, "", 0)}})
	if e != nil {
		t.Fatal(e)
	}
	defer app.Close()
	a, e := New(app, Config{})
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	h := httptest.NewServer(a)
	defer h.Close()
	request(t, h.URL, "GET", "/api/v1/forwardings", "", "", 200)
	request(t, h.URL, "GET", "/api/v1/devices/missing/forwarding-capabilities", "", "", 503)
	request(t, h.URL, "POST", "/api/v1/forwardings", "forward-serial-udp", `{"device_id":"d","kind":"serial","protocol":"udp","serial":"/dev/ttyUSB0"}`, 400)
	request(t, h.URL, "POST", "/api/v1/forwardings", "forward-missing", `{"device_id":"d","kind":"lan","protocol":"tcp","interface":"br0","target_ip":"192.168.1.2","target_port":80}`, 503)
	request(t, h.URL, "POST", "/api/v1/forwardings", "forward-missing", `{"device_id":"d","kind":"lan","protocol":"tcp","interface":"br0","target_ip":"192.168.1.2","target_port":80}`, 503)
}
