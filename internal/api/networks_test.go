package api

import (
	"encoding/json"
	"routerprobe/internal/task"
	"strings"
	"testing"
)

func TestNetworkCRUDDefaultsIdempotencyAndDisabledEngine(t *testing.T) {
	_, _, base, _ := fixture(t, Config{})
	settings := request(t, base, "GET", "/api/v1/network-settings", "", "", 200)
	if settings["data"].(map[string]any)["configured"] != false {
		t.Fatal(settings)
	}
	first := request(t, base, "POST", "/api/v1/networks", "network-create", "{\"name\":\"异地组网\"}", 201)
	n := first["data"].(map[string]any)
	id := n["network_id"].(string)
	again := request(t, base, "POST", "/api/v1/networks", "network-create", "{\"name\":\"异地组网\"}", 201)
	if again["data"].(map[string]any)["network_id"] != id {
		t.Fatal("duplicate creation")
	}
	if n["cidr"] != "10.144.144.0/24" {
		t.Fatal(n)
	}
	raw, _ := json.Marshal(first)
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "password") {
		t.Fatal("secret DTO")
	}
	request(t, base, "POST", "/api/v1/networks/"+id+"/members", "join", "{\"device_id\":\"test\"}", 503)
	request(t, base, "PUT", "/api/v1/networks/"+id, "update", "{\"name\":\"changed\",\"revision\":1}", 200)
	request(t, base, "PUT", "/api/v1/networks/"+id, "stale", "{\"name\":\"changed\",\"revision\":1}", 409)
	graph := request(t, base, "GET", "/api/v1/networks/"+id+"/topology", "", "", 200)
	data := graph["data"].(map[string]any)
	if len(data["nodes"].([]any)) != 0 || len(data["edges"].([]any)) != 0 {
		t.Fatal(graph)
	}
	request(t, base, "DELETE", "/api/v1/networks/"+id, "delete", "{}", 200)
	request(t, base, "GET", "/api/v1/networks/"+id, "", "", 404)
}
func TestNetworkTaskConfigServerRedacted(t *testing.T) {
	value := taskDTO(task.Snapshot{Spec: task.Spec{Type: "network_agent", Params: json.RawMessage(`{"action":"start","config_server":"tcp://host:22020/private-account","directory":"/tmp/root/net"}`)}})
	b, e := json.Marshal(value)
	if e != nil || strings.Contains(string(b), "private-account") || strings.Contains(string(b), "config_server") {
		t.Fatalf("task params leak %s %v", b, e)
	}
}
func TestNetworkExplicitPasswordAndNewProfile(t *testing.T) {
	_, _, base, _ := fixture(t, Config{})
	created := request(t, base, "POST", "/api/v1/networks", "new-profile", `{"name":"profile","password":"user-chosen","mtu":1380,"peer_urls":["tcp://et.criskaka.com"]}`, 201)
	n := created["data"].(map[string]any)
	id := n["network_id"].(string)
	if n["profile"] != float64(2) || n["mtu"] != float64(1380) {
		t.Fatal(n)
	}
	got := request(t, base, "GET", "/api/v1/networks/"+id+"/password", "", "", 200)
	if got["data"].(map[string]any)["password"] != "user-chosen" {
		t.Fatal("password changed")
	}
	for _, path := range []string{"/api/v1/networks", "/api/v1/networks/" + id, "/api/v1/networks/" + id + "/topology", "/api/v1/networks/" + id + "/operations"} {
		r := request(t, base, "GET", path, "", "", 200)
		raw, _ := json.Marshal(r)
		if strings.Contains(string(raw), "user-chosen") {
			t.Fatal("secret leaked", path)
		}
	}
	request(t, base, "POST", "/api/v1/networks/"+id+"/members/batch", "empty-batch", `{"members":[]}`, 400)
	request(t, base, "POST", "/api/v1/networks/"+id+"/members/batch", "no-anchor", `{"members":[{"device_id":"a"}]}`, 409)
	request(t, base, "PUT", "/api/v1/networks/"+id+"/members/none/config", "member-config", `{"revision":0,"config":{"hostname":"a","enable_manual_routes":true}}`, 404)
}
