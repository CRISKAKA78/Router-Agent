package api

import (
	"io"
	"log"
	"net/http/httptest"
	"routerprobe/internal/forwarding"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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
	request(t, h.URL, "GET", "/api/v1/networks", "", "", 200)
	request(t, h.URL, "GET", "/api/v1/network-settings", "", "", 200)
	request(t, h.URL, "GET", "/api/v1/devices/missing/forwarding-capabilities", "", "", 503)
	request(t, h.URL, "POST", "/api/v1/forwardings", "forward-serial-udp", `{"device_id":"d","kind":"serial","protocol":"udp","serial":"/dev/ttyUSB0"}`, 400)
	request(t, h.URL, "POST", "/api/v1/forwardings", "forward-missing", `{"device_id":"d","kind":"lan","protocol":"tcp","interface":"br0","target_ip":"192.168.1.2","target_port":80}`, 503)
	request(t, h.URL, "POST", "/api/v1/forwardings", "forward-missing", `{"device_id":"d","kind":"lan","protocol":"tcp","interface":"br0","target_ip":"192.168.1.2","target_port":80}`, 503)
}

// Both default and explicit subscriptions must retain the two independently
// integrated topics rather than letting one merge side replace the other.
func TestMergedNetworkAndForwardingEventTopics(t *testing.T) {
	a, _, base, _ := fixture(t, Config{})
	for _, query := range []string{"", "?topics=devices,maintenance,networks,forwardings"} {
		t.Run(query, func(t *testing.T) {
			c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(base, "http")+"/api/v1/events"+query, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			c.SetReadDeadline(time.Now().Add(5 * time.Second))
			var event struct {
				Type  string `json:"type"`
				Topic string `json:"topic"`
			}
			if err = c.ReadJSON(&event); err != nil || event.Type != "resync_required" {
				t.Fatalf("initial event: %+v, %v", event, err)
			}
			a.broadcast("networks")
			a.broadcast("forwardings")
			seen := map[string]bool{}
			for !seen["networks"] || !seen["forwardings"] {
				if err = c.ReadJSON(&event); err != nil {
					t.Fatalf("missing combined topics: %v: %v", seen, err)
				}
				if event.Type == "resource_changed" {
					seen[event.Topic] = true
				}
			}
		})
	}
}
