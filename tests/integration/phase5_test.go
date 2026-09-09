package integration

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
	"os"
	"os/exec"
	"path/filepath"
	"routerprobe/internal/api"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/tunnel"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type p5object = map[string]any

func p5request(t *testing.T, base, method, path, key string, body any, status int) p5object {
	t.Helper()
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r, _ := http.NewRequest(method, base+"/api/v1"+path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", key)
	v, e := http.DefaultClient.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	defer v.Body.Close()
	b, e = io.ReadAll(v.Body)
	if e != nil {
		t.Fatal(e)
	}
	var out p5object
	if json.Unmarshal(b, &out) != nil || v.StatusCode != status {
		t.Fatalf("%s %s status=%d want=%d: %s", method, path, v.StatusCode, status, b)
	}
	if status >= 400 {
		return out
	}
	return out["data"].(map[string]any)
}
func p5wait(t *testing.T, base, id string) p5object {
	t.Helper()
	end := time.Now().Add(10 * time.Second)
	for time.Now().Before(end) {
		v := p5request(t, base, "GET", "/tasks/"+id, "", nil, 200)
		if v["result"] != nil {
			return v
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("HTTP task result timeout")
	return nil
}
func TestPhase5RealProbeHTTPWebSocketWorkflow(t *testing.T) {
	binary := probeBinary(t)
	app, e := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{HeartbeatInterval: 10 * time.Second, Logger: log.New(io.Discard, "", 0)}, Tunnel: &tunnel.Config{DataListen: "127.0.0.1:0", PortFirst: 29000, PortLast: 29999, PortReuseDelay: time.Millisecond}})
	if e != nil {
		t.Fatal(e)
	}
	control, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	controlDone := make(chan error, 1)
	go func() { controlDone <- app.Serve(control) }()
	a, e := api.New(app, api.Config{PollInterval: 10 * time.Millisecond})
	if e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	apiDone := make(chan error, 1)
	go func() { apiDone <- a.Serve(listener) }()
	base := "http://" + listener.Addr().String()
	defer func() {
		a.Close()
		app.Close()
		if e := <-apiDone; e != nil {
			t.Error(e)
		}
		if e := <-controlDone; e != nil {
			t.Error(e)
		}
	}()
	ws, _, e := websocket.DefaultDialer.Dial("ws://"+listener.Addr().String()+"/api/v1/events", nil)
	if e != nil {
		t.Fatal(e)
	}
	var eventsMu sync.Mutex
	seen := map[string]bool{}
	eventsDone := make(chan struct{})
	go func() {
		defer close(eventsDone)
		for {
			var v p5object
			if ws.ReadJSON(&v) != nil {
				return
			}
			eventsMu.Lock()
			if topic, ok := v["topic"].(string); ok {
				seen[topic] = true
			}
			eventsMu.Unlock()
		}
	}()
	defer func() { ws.Close(); <-eventsDone }()
	start := func() *exec.Cmd {
		cmd := exec.Command(binary, "--server", control.Addr().String(), "--device-id", "phase5-device", "--boot-id", "phase5-boot")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if e = cmd.Start(); e != nil {
			t.Fatal(e)
		}
		return cmd
	}
	probe := start()
	defer func() { probe.Process.Kill(); probe.Wait() }()
	end := time.Now().Add(5 * time.Second)
	for {
		v, e := app.Devices().Get("phase5-device")
		if e == nil && v.CurrentSession != nil {
			break
		}
		if time.Now().After(end) {
			t.Fatal("probe offline")
		}
		time.Sleep(10 * time.Millisecond)
	}
	adoptProbe(t, app, "phase5-device", "", nil)
	d := p5request(t, base, "GET", "/devices/phase5-device", "", nil, 200)
	first := d["current_session"].(map[string]any)["session_id"]
	dir := t.TempDir()
	marker := filepath.Join(dir, "exec-marker")
	q := p5object{"device_id": "phase5-device", "command": "printf x >> '" + marker + "'; sleep 0.1; printf api-exec-ok", "timeout_seconds": 3}
	op := p5request(t, base, "POST", "/tasks", "exec", q, 202)
	id := op["task_id"].(string)
	if p5request(t, base, "POST", "/tasks", "exec", q, 202)["task_id"] != id {
		t.Fatal("HTTP retry created task")
	}
	v := p5wait(t, base, id)
	if v["result"].(map[string]any)["stdout"] != "api-exec-ok" {
		t.Fatal(v)
	}
	p5request(t, base, "POST", "/tasks/"+id+"/resend", "resend", p5object{}, 202)
	time.Sleep(40 * time.Millisecond)
	b, e := os.ReadFile(marker)
	if e != nil || string(b) != "x" {
		t.Fatal("exec repeated", string(b), e)
	}
	content := bytes.Repeat([]byte("phase5-file-"), 20000)
	digest := sha256.Sum256(content)
	r, _ := http.NewRequest("POST", base+"/api/v1/assets?name=phase5-tool", bytes.NewReader(content))
	r.Header.Set("Idempotency-Key", "asset")
	r.Header.Set("Content-Type", "application/octet-stream")
	r.Header.Set("X-Content-SHA256", hex.EncodeToString(digest[:]))
	res, e := http.DefaultClient.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	var envelope p5object
	json.NewDecoder(res.Body).Decode(&envelope)
	res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatal(envelope)
	}
	asset := envelope["data"].(map[string]any)["asset_id"].(string)
	remote := filepath.Join(dir, "uploaded")
	op = p5request(t, base, "POST", "/uploads", "upload", p5object{"device_id": "phase5-device", "asset_id": asset, "remote_path": remote, "mode": "0644", "timeout_seconds": 5}, 202)
	uploadID := op["task_id"].(string)
	v = p5wait(t, base, uploadID)
	if v["state"] != "success" {
		t.Fatal(v)
	}
	b, e = os.ReadFile(remote)
	if e != nil || !bytes.Equal(b, content) {
		t.Fatal("upload bytes", e)
	}
	p5request(t, base, "GET", "/tasks/"+uploadID+"/operation", "", nil, 200)
	op = p5request(t, base, "POST", "/downloads", "download", p5object{"device_id": "phase5-device", "remote_path": remote, "name": "downloaded", "timeout_seconds": 5}, 202)
	downloadID := op["task_id"].(string)
	v = p5wait(t, base, downloadID)
	if v["state"] != "success" {
		t.Fatal(v)
	}
	for i := 0; ; i++ {
		f := p5request(t, base, "GET", "/tasks/"+downloadID+"/transfer", "", nil, 200)
		if f["released"] == true && f["committed"] == true {
			break
		}
		if i > 100 {
			t.Fatal(f)
		}
		time.Sleep(10 * time.Millisecond)
	}
	complete := p5request(t, base, "POST", "/downloads/"+downloadID+"/complete", "complete", p5object{}, 200)
	downloadAsset := complete["asset"].(map[string]any)["asset_id"].(string)
	if p5request(t, base, "POST", "/downloads/"+downloadID+"/complete", "complete-again", p5object{}, 200)["asset"].(map[string]any)["asset_id"] != downloadAsset {
		t.Fatal("duplicate completed import")
	}
	res, e = http.Get(base + "/api/v1/assets/" + downloadAsset + "/content")
	if e != nil {
		t.Fatal(e)
	}
	b, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Equal(b, content) {
		t.Fatal("download bytes")
	}
	p5request(t, base, "POST", "/downloads/"+downloadID+"/cleanup", "cleanup", p5object{}, 200)
	tool := p5request(t, base, "POST", "/tools", "tool", p5object{"name": "phase5-tool"}, 201)["tool_id"].(string)
	version := p5request(t, base, "PUT", "/tools/"+tool+"/versions/v1", "publish", p5object{"artifacts": []any{p5object{"asset_id": asset, "platform": "linux", "mode": "0755", "rules": p5object{"arch": []string{"any"}, "libc": []string{"any"}}}}}, 200)
	artifact := version["artifacts"].([]any)[0].(map[string]any)["artifact_id"].(string)
	compat := p5request(t, base, "GET", "/tools/"+tool+"/versions/v1/compatibility?device_id=phase5-device", "", nil, 200)
	if compat["items"].([]any)[0].(map[string]any)["status"] != "compatible" {
		t.Fatal(compat)
	}
	deployPath := filepath.Join(dir, "deployed")
	op = p5request(t, base, "POST", "/deployments", "deploy", p5object{"device_id": "phase5-device", "tool_id": tool, "version": "v1", "artifact_id": artifact, "remote_path": deployPath, "timeout_seconds": 5}, 202)
	if p5wait(t, base, op["task_id"].(string))["state"] != "success" {
		t.Fatal(op)
	}
	st, e := os.Stat(deployPath)
	if e != nil || st.Mode().Perm() != 0755 {
		t.Fatal(st, e)
	}
	// The API returns the stable Phase 4 entry; HTTP bytes still use its data path.
	local, e := net.Listen("tcp", "127.0.0.1:80")
	if e != nil {
		t.Fatal(e)
	}
	web := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "phase5-maintenance-web") })}
	webDone := make(chan error, 1)
	go func() { webDone <- web.Serve(local) }()
	defer func() { web.Close(); <-webDone }()
	m := p5request(t, base, "POST", "/maintenance", "maintenance", p5object{"device_id": "phase5-device"}, 201)
	mid := m["maintenance_id"].(string)
	url := m["endpoints"].([]any)[0].(map[string]any)["url"].(string)
	res, e = http.Get(url)
	if e != nil {
		t.Fatal(e)
	}
	b, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if string(b) != "phase5-maintenance-web" {
		t.Fatal(string(b))
	}
	// Replacing the actual Probe invalidates the old maintenance and old Session.
	probe.Process.Kill()
	probe.Wait()
	probe = start()
	end = time.Now().Add(5 * time.Second)
	for {
		d = p5request(t, base, "GET", "/devices/phase5-device", "", nil, 200)
		current, _ := d["current_session"].(map[string]any)
		if current != nil && current["session_id"] != first {
			break
		}
		if time.Now().After(end) {
			t.Fatal("replacement not observed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	end = time.Now().Add(3 * time.Second)
	for {
		m = p5request(t, base, "GET", "/maintenance/"+mid, "", nil, 200)
		if m["released"] == true {
			break
		}
		if time.Now().After(end) {
			t.Fatal(m)
		}
		time.Sleep(10 * time.Millisecond)
	}
	p5request(t, base, "POST", "/maintenance/"+mid+"/close", "close", p5object{}, 200)
	end = time.Now().Add(time.Second)
	for {
		eventsMu.Lock()
		all := seen["devices"] && seen["tasks"] && seen["files"] && seen["maintenance"]
		eventsMu.Unlock()
		if all {
			break
		}
		if time.Now().After(end) {
			eventsMu.Lock()
			t.Error("missing WS topics", seen)
			eventsMu.Unlock()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", base+"/api/v1/tasks", strings.NewReader(`{}`))
	if _, e = http.DefaultClient.Do(req); e == nil {
		t.Fatal("cancelled HTTP request completed")
	}
}
