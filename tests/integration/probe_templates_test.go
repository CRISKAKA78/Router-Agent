package integration

import (
	"context"
	"io"
	"log"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"routerprobe/internal/api"
	"routerprobe/internal/device"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/probetemplate"
	"strings"
	"testing"
	"time"
)

func TestProbeTemplateControlLimit(t *testing.T) {
	binary := probeBinary(t)
	app, e := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{MaxControlPayload: 1024, Logger: log.New(io.Discard, "", 0)}})
	if e != nil {
		t.Fatal(e)
	}
	defer app.Close()
	template, e := app.ProbeTemplates().Put("", 0, probetemplate.Input{Name: "large-output", Properties: map[string]probetemplate.Property{"custom": {Name: "Custom", Command: "head -c 1500 /dev/zero | tr '\\000' x"}}})
	if e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(l) }()
	defer func() { app.Close(); <-done }()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, binary, "--server", l.Addr().String(), "--device-id", "limited", "--template-id", template.ID)
	out, e := c.CombinedOutput()
	if e == nil || ctx.Err() != nil || !strings.Contains(string(out), "exceeds server control payload limit") {
		t.Fatal(string(out), e, ctx.Err())
	}
	if len(app.Devices().List()) != 0 {
		t.Fatal("oversize registration published")
	}
}

func TestProbeTemplateStartupSnapshot(t *testing.T) {
	binary := probeBinary(t)
	app, e := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{Logger: log.New(io.Discard, "", 0)}})
	if e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(listener) }()
	adapter, e := api.New(app, api.Config{})
	if e != nil {
		t.Fatal(e)
	}
	httpServer := httptest.NewServer(adapter)
	defer func() {
		adapter.Close()
		httpServer.Close()
		app.Close()
		if e := <-done; e != nil {
			t.Error(e)
		}
	}()
	tmp := t.TempDir()
	counter := filepath.Join(tmp, "calls")
	if e = os.WriteFile(filepath.Join(tmp, "nvram"), []byte("#!/bin/sh\n[ \"$1\" = get ] && [ \"$2\" = SN ] || exit 1\nprintf '  AUTO-SN \\n'\n"), 0700); e != nil {
		t.Fatal(e)
	}
	props := p5object{"model": p5object{"name": "型号", "command": "printf model-v1"}, "custom": p5object{"name": "自定义", "command": "printf x >> '" + counter + "'; printf ' custom-value \\n'"}, "bad": p5object{"name": "失败项", "command": "exit 2"}, "hostname": p5object{"name": "主机名", "command": "printf collected-host"}}
	v := p5request(t, httpServer.URL, "POST", "/probe-templates", "template-startup", p5object{"name": "启动模板", "properties": props}, 201)
	id := v["template_id"].(string)
	start := func(args ...string) *exec.Cmd {
		args = append([]string{"--server", listener.Addr().String()}, args...)
		c := exec.Command(binary, args...)
		c.Env = append(os.Environ(), "PATH="+tmp+":"+os.Getenv("PATH"))
		c.Stdout = io.Discard
		c.Stderr = io.Discard
		if e := c.Start(); e != nil {
			t.Fatal(e)
		}
		return c
	}
	stop := func(c *exec.Cmd) { c.Process.Kill(); c.Wait() }
	wait := func(id string, after string) device.Snapshot {
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			v, e := app.Devices().Get(id)
			if e == nil && v.CurrentSession != nil && v.CurrentSession.ID != after {
				return v
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("device did not register", id)
		return device.Snapshot{}
	}
	c := start("--template-name", "启动模板", "--hostname", "manual-host")
	defer stop(c)
	first := wait("AUTO-SN", "")
	if first.Registration.Model != "model-v1" || first.Registration.Hostname != "manual-host" || first.Registration.Attributes["custom"].Value != "custom-value" || first.Registration.CollectionErrors["bad"].Reason != "command_failed" {
		t.Fatal(first.Registration)
	}
	dto := p5request(t, httpServer.URL, "GET", "/devices/AUTO-SN", "", nil, 200)["registration"].(map[string]any)
	if dto["template"].(map[string]any)["template_id"] != id || dto["attributes"].(map[string]any)["custom"].(map[string]any)["value"] != "custom-value" {
		t.Fatal(dto)
	}
	props["model"] = p5object{"name": "型号", "command": "printf model-v2"}
	p5request(t, httpServer.URL, "PUT", "/probe-templates/"+id, "template-update", p5object{"name": "启动模板", "properties": props, "version": 1}, 200)
	p5request(t, httpServer.URL, "POST", "/devices/AUTO-SN/disconnect", "disconnect-template", p5object{}, 200)
	second := wait("AUTO-SN", first.CurrentSession.ID)
	if second.Registration.Model != "model-v1" || second.Registration.Template.Version != 1 {
		t.Fatal("reconnect recollected", second)
	}
	b, e := os.ReadFile(counter)
	if e != nil || string(b) != "x" {
		t.Fatal("command repeated", string(b), e)
	}
	history, e := app.Devices().Sessions("AUTO-SN")
	if e != nil || len(history.Ended) != 1 || history.Ended[0].Registration.Template.Version != 1 {
		t.Fatal(history, e)
	}
	manual := start("--device_id", "manual-device", "--template-id", id)
	defer stop(manual)
	latest := wait("manual-device", "")
	if latest.Registration.Model != "model-v2" || latest.Registration.Template.Version != 2 || latest.Registration.Hostname != "collected-host" {
		t.Fatal(latest)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	missing := exec.CommandContext(ctx, binary, "--device-id", "missing", "--server", listener.Addr().String(), "--template-name", "不存在")
	if e := missing.Run(); e == nil || ctx.Err() != nil {
		t.Fatal("missing template must exit explicitly", e, ctx.Err())
	}
}
