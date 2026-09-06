package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/protocol"
	"routerprobe/internal/tunnel"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func fixture(t *testing.T, c Config) (*Server, *management.Server, string, string) {
	t.Helper()
	app, e := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{HeartbeatInterval: 10 * time.Second, Logger: log.New(io.Discard, "", 0)}, Tunnel: &tunnel.Config{DataListen: "127.0.0.1:0", PortFirst: 28000, PortLast: 28999, PortReuseDelay: time.Millisecond}})
	if e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(l) }()
	a, e := New(app, c)
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(a)
	t.Cleanup(func() {
		a.Close()
		server.Close()
		app.Close()
		if e := <-done; e != nil {
			t.Error(e)
		}
	})
	return a, app, server.URL, l.Addr().String()
}
func request(t *testing.T, base, method, path, key, body string, status int) object {
	t.Helper()
	r, e := http.NewRequest(method, base+path, strings.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	v, e := http.DefaultClient.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	defer v.Body.Close()
	b, e := io.ReadAll(v.Body)
	if e != nil {
		t.Fatal(e)
	}
	if v.StatusCode != status {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, v.StatusCode, status, b)
	}
	var out object
	if e = json.Unmarshal(b, &out); e != nil {
		t.Fatal(e, string(b))
	}
	return out
}
func data(v object) object { return v["data"].(map[string]any) }
func register(t *testing.T, address, id string) (net.Conn, string) {
	t.Helper()
	c, e := net.Dial("tcp", address)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	b, _ := json.Marshal(object{"device_id": id, "probe_version": "test", "arch": "x86_64", "boot_id": "test", "capabilities": []string{"exec", "file", "tunnel"}})
	frame := protocol.Frame{Header: protocol.Header{Version: 1, Type: protocol.TypeRegister, MessageID: 1}, Payload: b}
	wire, e := protocol.EncodeFrame(frame)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Write(wire); e != nil {
		t.Fatal(e)
	}
	c.SetReadDeadline(time.Now().Add(time.Second))
	decoder := protocol.NewDecoder(1 << 20)
	buf := make([]byte, 4096)
	for {
		n, e := c.Read(buf)
		if e != nil {
			t.Fatal(e)
		}
		frames, e := decoder.Feed(buf[:n])
		if e != nil {
			t.Fatal(e)
		}
		if len(frames) > 0 {
			var ack object
			json.Unmarshal(frames[0].Payload, &ack)
			c.SetReadDeadline(time.Time{})
			return c, ack["session_id"].(string)
		}
	}
}
func eventually(t *testing.T, f func() bool) {
	t.Helper()
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		if f() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition did not converge")
}

func TestHTTPDeviceSessionsMaintenanceAndCancellation(t *testing.T) {
	_, app, base, control := fixture(t, Config{PollInterval: 5 * time.Millisecond})
	request(t, base, "GET", "/api/v1/devices/missing", "", "", 404)
	c, first := register(t, control, "api-device")
	eventually(t, func() bool { v, e := app.Devices().Get("api-device"); return e == nil && v.CurrentSession != nil })
	v := data(request(t, base, "GET", "/api/v1/devices?status=online&limit=1", "", "", 200))
	if v["total"] != float64(1) {
		t.Fatal(v)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r, _ := http.NewRequestWithContext(ctx, "POST", base+"/api/v1/maintenance", strings.NewReader(`{"device_id":"api-device"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "maintenance-default")
	res, e := http.DefaultClient.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	var envelope object
	json.NewDecoder(res.Body).Decode(&envelope)
	res.Body.Close()
	cancel()
	m := data(envelope)
	id := m["maintenance_id"].(string)
	stored, e := app.Maintenance().Get(id)
	if e != nil || stored.State != "ready" || stored.ExpiresAt.Sub(stored.CreatedAt) != tunnel.DefaultLease {
		t.Fatal(stored, e)
	}
	encoded, _ := json.Marshal(m)
	for _, secret := range []string{"token", "connection_id", "data_host", "socket"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatal(string(encoded))
		}
	}
	if len(m["endpoints"].([]any)) != 3 {
		t.Fatal(m)
	}
	request(t, base, "POST", "/api/v1/maintenance", "maintenance-conflict", `{"device_id":"api-device"}`, 409)
	_, second := register(t, control, "api-device")
	if first == second {
		t.Fatal("session unchanged")
	}
	eventually(t, func() bool { v, _ := app.Maintenance().Get(id); return v.Released })
	d := data(request(t, base, "GET", "/api/v1/devices/api-device", "", "", 200))
	if d["current_session"].(map[string]any)["session_id"] != second || d["last_offline_at"] != nil {
		t.Fatal(d)
	}
	h := data(request(t, base, "GET", "/api/v1/devices/api-device/sessions", "", "", 200))
	if h["total"] != float64(2) {
		t.Fatal(h)
	}
	request(t, base, "POST", "/api/v1/maintenance/"+id+"/close", "close-old", "{}", 200)
	m = data(request(t, base, "POST", "/api/v1/maintenance", "short", `{"device_id":"api-device","lease_ms":40}`, 201))
	id = m["maintenance_id"].(string)
	eventually(t, func() bool { v, _ := app.Maintenance().Get(id); return v.Released && v.Reason == "expired" })
	request(t, base, "POST", "/api/v1/maintenance", "zero", `{"device_id":"api-device","lease_ms":0}`, 400)
	request(t, base, "POST", "/api/v1/maintenance", "huge", `{"device_id":"api-device","lease_ms":9223372036855}`, 400)
	long := data(request(t, base, "POST", "/api/v1/maintenance", "long-lease", `{"device_id":"api-device","lease_ms":31622400000}`, 201))
	request(t, base, "POST", "/api/v1/maintenance/"+long["maintenance_id"].(string)+"/close", "close-long", "{}", 200)
	request(t, base, "POST", "/api/v1/devices/api-device/disconnect", "disconnect", "{}", 200)
	c.Close()
	eventually(t, func() bool { d, _ := app.Devices().Get("api-device"); return d.CurrentSession == nil })
	request(t, base, "POST", "/api/v1/tasks", "offline", `{"device_id":"api-device","command":"true","timeout_seconds":1}`, 409)
}

func TestHTTPIdempotencyConcurrencyValidationAndRepository(t *testing.T) {
	_, _, base, _ := fixture(t, Config{})
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v := data(request(t, base, "POST", "/api/v1/tools", "same", `{"name":"test"}`, 201))
			ids <- v["tool_id"].(string)
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for v := range ids {
		if id != "" && id != v {
			t.Fatal("duplicate tool")
		}
		id = v
	}
	request(t, base, "POST", "/api/v1/tools", "same", `{"name":"changed"}`, 409)
	for i, body := range []string{`{"name":"a","name":"b"}`, `{"name":null}`, `[]`, `{"name":"a"} {}`, `{"name":"a","unknown":1}`, `{"name":"\ud800"}`} {
		request(t, base, "POST", "/api/v1/tools", fmt.Sprint("invalid", i), body, 400)
	}
	request(t, base, "GET", "/api/v1/tools?limit=201", "", "", 400)
	request(t, base, "GET", "/api/v1/tasks?state=received%7Cqueued", "", "", 400)
	request(t, base, "DELETE", "/api/v1/tools/"+id, "", "", 405)
	request(t, base, "GET", "/api/v1/nope", "", "", 404)
	content := []byte("asset bytes\x00with binary")
	digest := sha256.Sum256(content)
	importAsset := func(key string, status int) object {
		r, _ := http.NewRequest("POST", base+"/api/v1/assets?name=payload", bytes.NewReader(content))
		r.Header.Set("Content-Type", "application/octet-stream")
		r.Header.Set("Idempotency-Key", key)
		r.Header.Set("X-Content-SHA256", hex.EncodeToString(digest[:]))
		v, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer v.Body.Close()
		var out object
		json.NewDecoder(v.Body).Decode(&out)
		if v.StatusCode != status {
			t.Fatal(v.StatusCode, out)
		}
		return data(out)
	}
	asset := importAsset("asset", 201)
	if importAsset("asset", 201)["asset_id"] != asset["asset_id"] {
		t.Fatal("duplicate import")
	}
	assetID := asset["asset_id"].(string)
	v, e := http.Get(base + "/api/v1/assets/" + assetID + "/content")
	if e != nil {
		t.Fatal(e)
	}
	got, _ := io.ReadAll(v.Body)
	v.Body.Close()
	if !bytes.Equal(got, content) {
		t.Fatal(string(got))
	}
	rangeRequest, _ := http.NewRequest("GET", base+"/api/v1/assets/"+assetID+"/content", nil)
	rangeRequest.Header.Set("Range", "bytes=1-3")
	rangeResponse, e := http.DefaultClient.Do(rangeRequest)
	if e != nil {
		t.Fatal(e)
	}
	part, _ := io.ReadAll(rangeResponse.Body)
	rangeResponse.Body.Close()
	if rangeResponse.StatusCode != 206 || !bytes.Equal(part, content[1:4]) {
		t.Fatal(rangeResponse.StatusCode, string(part))
	}
	rangeRequest, _ = http.NewRequest("GET", base+"/api/v1/assets/"+assetID+"/content", nil)
	rangeRequest.Header.Set("Range", "bytes=9999-")
	rangeResponse, e = http.DefaultClient.Do(rangeRequest)
	if e != nil {
		t.Fatal(e)
	}
	part, _ = io.ReadAll(rangeResponse.Body)
	rangeResponse.Body.Close()
	if rangeResponse.StatusCode != 416 || !bytes.Contains(part, []byte(`"code":"range_not_satisfiable"`)) {
		t.Fatal(rangeResponse.StatusCode, string(part))
	}
	body := fmt.Sprintf(`{"artifacts":[{"asset_id":%q,"platform":"linux","mode":"0755","rules":{"arch":["any"],"libc":["any"]}}]}`, assetID)
	version := data(request(t, base, "PUT", "/api/v1/tools/"+id+"/versions/v1", "publish", body, 200))
	if len(version["artifacts"].([]any)) != 1 {
		t.Fatal(version)
	}
	request(t, base, "PUT", "/api/v1/tools/"+id+"/versions/v1", "publish-repeat", body, 200)
	request(t, base, "PUT", "/api/v1/tools/"+id+"/versions/bad", "bad-version", `{"artifacts":[]}`, 400)
	request(t, base, "POST", "/api/v1/assets/"+assetID+"/archive", "referenced", "{}", 409)
	request(t, base, "POST", "/api/v1/tools/"+id+"/versions/v1/archive", "archive-version", "{}", 200)
	request(t, base, "POST", "/api/v1/assets/"+assetID+"/archive", "archive-asset", "{}", 200)
	request(t, base, "GET", "/api/v1/assets/"+assetID+"/content", "", "", 409)
}

func TestWebSocketClientsSlowConsumerAndShutdown(t *testing.T) {
	a, _, base, control := fixture(t, Config{MaxClients: 2, WriteTimeout: 30 * time.Millisecond, PollInterval: 5 * time.Millisecond})
	dial := func() *websocket.Conn {
		c, _, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(base, "http")+"/api/v1/events", nil)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { c.Close() })
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		var v object
		if e = c.ReadJSON(&v); e != nil || v["type"] != "resync_required" {
			t.Fatal(v, e)
		}
		return c
	}
	one, two := dial(), dial()
	_, res, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(base, "http")+"/api/v1/events", nil)
	if e == nil || res.StatusCode != 503 {
		t.Fatal(res, e)
	}
	res.Body.Close()
	register(t, control, "ws-device")
	for _, c := range []*websocket.Conn{one, two} {
		for {
			var v object
			if e = c.ReadJSON(&v); e != nil {
				t.Fatal(e)
			}
			if v["topic"] == "devices" {
				break
			}
		}
	}
	one.Close()
	eventually(t, func() bool { a.mu.Lock(); defer a.mu.Unlock(); return len(a.clients) == 1 })
	// Exercise the actual nonblocking overflow path without relying on OS buffer size.
	a.mu.Lock()
	var serverConn *websocket.Conn
	for existing := range a.clients {
		serverConn = existing.conn
	}
	slow := &client{conn: serverConn, queue: make(chan string, 1), topics: map[string]bool{"tasks": true}}
	a.clients[slow] = struct{}{}
	a.mu.Unlock()
	a.broadcast("tasks")
	a.broadcast("tasks")
	if _, _, e := two.ReadMessage(); e == nil {
		if _, _, e = two.ReadMessage(); e == nil {
			t.Fatal("slow client remains open")
		}
	}
	a.mu.Lock()
	delete(a.clients, slow)
	a.mu.Unlock()
	eventually(t, func() bool { a.mu.Lock(); defer a.mu.Unlock(); return len(a.clients) == 0 })
	three := dial()
	done := make(chan struct{})
	go func() { a.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked")
	}
	if _, _, e := three.ReadMessage(); e == nil {
		t.Fatal("WS survived shutdown")
	}
	request(t, base, "GET", "/api/v1/devices", "", "", 503)
}

func TestAPIAdmissionAndKeyCapacity(t *testing.T) {
	a, _, base, _ := fixture(t, Config{IdempotencyCapacity: 1, MaxRequests: 1})
	request(t, base, "POST", "/api/v1/tools", "first", `{"name":"a"}`, 201)
	request(t, base, "POST", "/api/v1/tools", "second", `{"name":"b"}`, 503)
	request(t, base, "POST", "/api/v1/tools", "first", `{"name":"a"}`, 201)
	a.requests <- struct{}{}
	request(t, base, "GET", "/api/v1/devices", "", "", 503)
	<-a.requests
	r, _ := http.NewRequest("GET", base+"/api/v1/devices", nil)
	r.Header.Set("Origin", "https://untrusted.example")
	res, e := http.DefaultClient.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal(res.StatusCode)
	}
}

func TestHTTPCancelAfterAdmissionRetainsCreatedObjectAndReplay(t *testing.T) {
	a, app, base, control := fixture(t, Config{})
	register(t, control, "cancel-device")
	eventually(t, func() bool { d, e := app.Devices().Get("cancel-device"); return e == nil && d.CurrentSession != nil })
	created := make(chan string, 1)
	release := make(chan struct{})
	finished := make(chan struct{})
	a.route("POST /api/v1/test-create", true, func(r *http.Request) response {
		v, e := app.Maintenance().Create(r.Context(), "cancel-device", 0)
		if e != nil {
			return failure(e)
		}
		created <- v.ID
		<-release
		defer close(finished)
		if r.Context().Err() != nil {
			return failure(r.Context().Err())
		}
		return response{status: 201, data: maintenanceDTO(v)}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, "POST", base+"/api/v1/test-create", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "cancel-create")
	clientDone := make(chan error, 1)
	go func() {
		res, e := http.DefaultClient.Do(r)
		if res != nil {
			res.Body.Close()
		}
		clientDone <- e
	}()
	var id string
	select {
	case id = <-created:
	case <-time.After(time.Second):
		t.Fatal("creation timeout")
	}
	cancel()
	if e := <-clientDone; e == nil {
		t.Fatal("request should be cancelled")
	}
	close(release)
	<-finished
	v, e := app.Maintenance().Get(id)
	if e != nil || v.State != "ready" {
		t.Fatal(v, e)
	}
	replay := data(request(t, base, "POST", "/api/v1/test-create", "cancel-create", "{}", 201))
	if replay["maintenance_id"] != id {
		t.Fatal(replay)
	}
}

func TestDispatchUncertainRetainsIdentityWithoutPrivateDiagnostics(t *testing.T) {
	r := accepted("task-identity", object{"task_id": "task-identity"}, fmt.Errorf("%w: socket token connection_id private", gateway.ErrDispatchUncertain))
	w := httptest.NewRecorder()
	write(w, r)
	if w.Code != 202 || !strings.Contains(w.Body.String(), `"dispatch_uncertain":true`) || strings.Contains(w.Body.String(), "private") || w.Header().Get("Location") != "/api/v1/tasks/task-identity" {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestBoundedListenerAndIdleShutdown(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	b := &boundedListener{Listener: l, slots: make(chan struct{}, 1)}
	defer b.Close()
	first, e := net.Dial("tcp", l.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer first.Close()
	server, e := b.Accept()
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	done := make(chan error, 1)
	go func() { _, e := b.Accept(); done <- e }()
	second, e := net.Dial("tcp", l.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer second.Close()
	second.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 1)
	if _, e = second.Read(buf); e == nil {
		t.Fatal("overflow connection survived")
	}
	b.Close()
	if e = <-done; e == nil {
		t.Fatal("closed listener accepted")
	}
	server.Close()
	if len(b.slots) != 0 {
		t.Fatal("slot retained")
	}
}
