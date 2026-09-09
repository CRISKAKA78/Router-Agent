package integration

import (
	"context"
	"fmt"
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
	"strings"
	"testing"
	"time"
)

// Runs the actual Probe against the HTTP API. Firmware programs are isolated
// argv-recording test substitutes, not evidence of DD-WRT/OpenWrt compatibility.
func TestRouterConfigProbeAPI(t *testing.T) {
	binary := probeBinary(t)
	app, err := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{Logger: log.New(io.Discard, "", 0)}})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(listener) }()
	adapter, err := api.New(app, api.Config{})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(adapter)
	defer func() {
		adapter.Close()
		server.Close()
		app.Close()
		if e := <-done; e != nil {
			t.Error(e)
		}
	}()
	tmp := t.TempDir()
	script := `#!/bin/sh
backend="${0##*/}"
dir="$CONFIG_DIR/$backend-store"
case "$1" in
 get) [ "$#" = 2 ] || exit 12
      [ "$2" != timeout ] || sleep 10
      [ -f "$dir/$2" ] || exit 4
      cat "$dir/$2";;
 set) [ "$#" = 2 ] || exit 13
      key="${2%%=*}"; value="${2#*=}"
      printf '%s' "$value" > "$dir/$key"
      printf 'set\n' >> "$dir/calls"
      if [ "$key" = slow ]; then touch "$dir/started"; while [ ! -f "$dir/gate" ]; do sleep 0.02; done; fi;;
 unset|delete) [ "$#" = 2 ] || exit 14; rm "$dir/$2";;
 commit) if [ "$backend" = uci ]; then [ "$#" = 2 ] || exit 15; fi
         printf '%s' "${2-all}" > "$dir/committed";;
 *) exit 16;;
esac
`
	for _, backend := range []string{"nvram", "uci"} {
		if err = os.WriteFile(filepath.Join(tmp, backend), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.Mkdir(filepath.Join(tmp, backend+"-store"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	seed := func(backend, key, value string) {
		t.Helper()
		if e := os.WriteFile(filepath.Join(tmp, backend+"-store", key), []byte(value), 0600); e != nil {
			t.Fatal(e)
		}
	}
	seed("nvram", "SN", "serial-one")
	seed("uci", "system.@system[0].hostname", "openwrt-one")
	props := p5object{"serial": p5object{"name": "序列号", "source": "nvram", "key": "SN"}, "hostname": p5object{"name": "主机名", "source": "uci", "key": "system.@system[0].hostname"}, "missing": p5object{"name": "缺失", "source": "nvram", "key": "absent"}}
	template := p5request(t, server.URL, "POST", "/probe-templates", "config-template", p5object{"name": "config sources", "properties": props}, 201)
	proc := exec.Command(binary, "--server", listener.Addr().String(), "--device-id", "config-probe")
	proc.Env = append(os.Environ(), "PATH="+tmp+":"+os.Getenv("PATH"), "CONFIG_DIR="+tmp)
	proc.Stdout = io.Discard
	proc.Stderr = io.Discard
	if err = proc.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { proc.Process.Kill(); proc.Wait() }()
	wait := func(f func() bool) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if f() {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("config wait timeout")
	}
	wait(func() bool { v, e := app.Devices().Get("config-probe"); return e == nil && v.CurrentSession != nil })
	adoptProbe(t, app, "config-probe", template["template_id"].(string), nil)
	first := waitManagedMetric(t, app, "config-probe", func(d device.Snapshot, m map[string]device.Metric) bool {
		return m["serial"].Value == "serial-one" && m["hostname"].Value == "openwrt-one" && m["missing"].Reason == "command_failed"
	})

	count := 0
	create := func(q p5object) p5object {
		t.Helper()
		count++
		return p5request(t, server.URL, "POST", "/devices/config-probe/config-tasks", fmt.Sprint("config-", count), q, 202)
	}
	check := func(id, status, stdout string) {
		t.Helper()
		v := p5wait(t, server.URL, id)
		r := v["result"].(map[string]any)
		if r["status"] != status || r["stdout"] != stdout || v["type"] != "router_config" {
			t.Fatal(v)
		}
	}
	for _, backend := range []string{"nvram", "uci"} {
		key := "SN"
		if backend == "uci" {
			key = "system.@system[0].hostname"
		}
		special := " a'\";$(touch " + filepath.Join(tmp, "injected") + ")\n中文 "
		q := p5object{"backend": backend, "operation": "set", "key": key, "value": special}
		created := create(q)
		id := created["task_id"].(string)
		check(id, "success", "")
		// Same HTTP bytes/key must reuse the exact task identity.
		replay := p5request(t, server.URL, "POST", "/devices/config-probe/config-tasks", fmt.Sprint("config-", count), q, 202)
		if replay["task_id"] != id {
			t.Fatal("HTTP replay created another task")
		}
		q["value"] = "different"
		p5request(t, server.URL, "POST", "/devices/config-probe/config-tasks", fmt.Sprint("config-", count), q, 409)
		snap := p5request(t, server.URL, "GET", "/tasks/"+id, "", nil, 200)
		if snap["params"].(map[string]any)["value"] != special {
			t.Fatal("immutable params changed", snap)
		}
		check(create(p5object{"backend": backend, "operation": "get", "key": key})["task_id"].(string), "success", special)
		if _, e := os.Stat(filepath.Join(tmp, "injected")); !os.IsNotExist(e) {
			t.Fatal("shell injection", e)
		}
		if _, e := os.Stat(filepath.Join(tmp, backend+"-store", "committed")); !os.IsNotExist(e) {
			t.Fatal("implicit commit", e)
		}
		p5request(t, server.URL, "POST", "/tasks/"+id+"/resend", "resend-"+backend, p5object{}, 202)
		time.Sleep(100 * time.Millisecond)
		calls, e := os.ReadFile(filepath.Join(tmp, backend+"-store", "calls"))
		if e != nil || string(calls) != "set\n" {
			t.Fatal("duplicate write", string(calls), e)
		}
		commit := p5object{"backend": backend, "operation": "commit"}
		expected := "all"
		if backend == "uci" {
			commit["package"] = "system"
			expected = "system"
		}
		check(create(commit)["task_id"].(string), "success", "")
		committed, e := os.ReadFile(filepath.Join(tmp, backend+"-store", "committed"))
		if e != nil || string(committed) != expected {
			t.Fatal(string(committed), e)
		}
		check(create(p5object{"backend": backend, "operation": "set", "key": key, "value": ""})["task_id"].(string), "success", "")
		check(create(p5object{"backend": backend, "operation": "get", "key": key})["task_id"].(string), "success", "")
		check(create(p5object{"backend": backend, "operation": "delete", "key": key})["task_id"].(string), "success", "")
		check(create(p5object{"backend": backend, "operation": "get", "key": key})["task_id"].(string), "failed", "")
	}
	check(create(p5object{"backend": "nvram", "operation": "get", "key": "timeout", "timeout_seconds": 1})["task_id"].(string), "timeout", "")
	// Accepted write continues while disconnected; re-registration retains the
	// startup properties even though both underlying keys have now been deleted.
	slow := create(p5object{"backend": "nvram", "operation": "set", "key": "slow", "value": "once"})["task_id"].(string)
	wait(func() bool { _, e := os.Stat(filepath.Join(tmp, "nvram-store", "started")); return e == nil })
	p5request(t, server.URL, "POST", "/devices/config-probe/disconnect", "config-disconnect", p5object{}, 200)
	seed("nvram", "gate", "go")
	wait(func() bool {
		v, e := app.Devices().Get("config-probe")
		return e == nil && v.CurrentSession != nil && v.CurrentSession.ID != first.CurrentSession.ID
	})
	check(slow, "success", "")
	waitManagedMetric(t, app, "config-probe", func(d device.Snapshot, m map[string]device.Metric) bool {
		return m["serial"].Value == "serial-one" && m["hostname"].Value == "openwrt-one"
	})

	calls, _ := os.ReadFile(filepath.Join(tmp, "nvram-store", "calls"))
	if strings.Count(string(calls), "set\n") != 3 {
		t.Fatal("write ran more than once", string(calls))
	}
	if err = os.Rename(filepath.Join(tmp, "uci"), filepath.Join(tmp, "uci-disabled")); err != nil {
		t.Fatal(err)
	}
	v := p5wait(t, server.URL, create(p5object{"backend": "uci", "operation": "get", "key": "system.main.hostname"})["task_id"].(string))
	if v["result"].(map[string]any)["status"] != "failed" {
		t.Fatal("missing command succeeded", v)
	}
	// Existing result remains queryable through the application layer as well.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err = app.WaitTaskResult(ctx, slow); err != nil {
		t.Fatal(err)
	}
}
