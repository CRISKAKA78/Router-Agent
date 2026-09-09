package integration

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"routerprobe/internal/api"
	"routerprobe/internal/device"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/probetemplate"
	"routerprobe/internal/protocol"
	"strconv"
	"strings"
	"testing"
	"time"
)

func assertSystemHeartbeat(t *testing.T, frame protocol.Frame) {
	t.Helper()
	var heartbeat struct {
		Uptime       uint64 `json:"uptime"`
		Valid        *bool  `json:"uptime_valid"`
		RunningTasks uint16 `json:"running_tasks"`
	}
	if frame.Header.Type != protocol.TypeHeartbeat || frame.Header.Flags != 0 || json.Unmarshal(frame.Payload, &heartbeat) != nil || heartbeat.Valid == nil || !*heartbeat.Valid {
		t.Fatalf("invalid system heartbeat: %#v %s", frame.Header, frame.Payload)
	}
}

func TestSystemInfoRealProbeAPI(t *testing.T) {
	binary := probeBinary(t)
	kernel, err := exec.Command("uname", "-r").Output()
	if err != nil {
		t.Fatal(err)
	}
	app, err := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{HeartbeatInterval: 10 * time.Second, Logger: log.New(io.Discard, "", 0)}})
	if err != nil {
		t.Fatal(err)
	}
	control, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(control) }()
	adapter, err := api.New(app, api.Config{})
	if err != nil {
		app.Close()
		<-done
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(adapter)
	defer func() {
		adapter.Close()
		httpServer.Close()
		app.Close()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	wait := func(t *testing.T, id, previous string) device.Snapshot {
		t.Helper()
		deadline := time.Now().Add(4 * time.Second) // Must report immediately, not after the 10s interval.
		for time.Now().Before(deadline) {
			v, err := app.Devices().Get(id)
			if err == nil && v.CurrentSession != nil && v.CurrentSession.ID != previous && v.CurrentSession.Runtime != nil {
				return v
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("missing first heartbeat within 4 seconds")
		return device.Snapshot{}
	}
	for _, name := range []string{"default", "template-default", "template-override", "template-failure"} {
		t.Run(name, func(t *testing.T) {
			args := []string{}
			templateID := ""
			expected := strings.TrimSpace(string(kernel))
			if name != "default" {
				props := map[string]probetemplate.Property{"model": {Name: "型号", Command: "printf model"}}
				if name == "template-override" {
					props["kernel"] = probetemplate.Property{Name: "内核", Command: "printf 5.10-vendor"}
					expected = "5.10-vendor"
				}
				if name == "template-failure" {
					props["kernel"] = probetemplate.Property{Name: "内核", Command: "exit 2"}
					expected = ""
				}
				template, err := app.ProbeTemplates().Put("", 0, probetemplate.Input{Name: name, Properties: props})
				if err != nil {
					t.Fatal(err)
				}
				templateID = template.ID
			}
			if name == "template-override" {
				args = append(args, "--arch", "manual-arch")
			}
			probe := startProbe(t, binary, control.Addr().String(), name, io.Discard, args...)
			v := wait(t, name, "")
			if v.Registration.Kernel != strings.TrimSpace(string(kernel)) || v.Registration.Arch == "" || v.Registration.Arch == "unknown" {
				t.Fatalf("wrong system info: %#v", v.Registration)
			}
			if name == "template-override" && v.Registration.Arch != "manual-arch" {
				t.Fatal("explicit arch ignored")
			}
			if templateID != "" {
				adoptProbe(t, app, name, templateID, nil)
				waitManagedMetric(t, app, name, func(d device.Snapshot, m map[string]device.Metric) bool {
					if name == "template-default" {
						return d.LatestSession.ConfigRevision == 1 && m["model"].Value == "model"
					}
					if name == "template-failure" {
						return m["kernel"].Reason == "command_failed"
					}
					return m["kernel"].Value == expected
				})
			}
			data, err := os.ReadFile("/proc/uptime")
			if err != nil {
				t.Fatal(err)
			}
			host, err := strconv.ParseFloat(strings.Fields(string(data))[0], 64)
			if err != nil {
				t.Fatal(err)
			}
			runtime := v.CurrentSession.Runtime
			if runtime.UptimeSeconds == nil || float64(*runtime.UptimeSeconds) < host-3 || float64(*runtime.UptimeSeconds) > host+3 {
				t.Fatalf("uptime is not system uptime: %#v host=%v", runtime, host)
			}
			wire := p5request(t, httpServer.URL, "GET", "/devices/"+name, "", nil, 200)
			if wire["registration"].(map[string]any)["kernel"] != strings.TrimSpace(string(kernel)) || wire["runtime"].(map[string]any)["uptime_seconds"] != float64(*runtime.UptimeSeconds) {
				t.Fatalf("API discarded sample: %#v", wire)
			}
			if name == "default" {
				// Wait for the negotiated periodic heartbeat, independently of arbitrary task traffic.
				deadline := time.Now().Add(13 * time.Second)
				var next device.Snapshot
				for time.Now().Before(deadline) {
					next, _ = app.Devices().Get(name)
					if next.CurrentSession.Runtime.ReportedAt.After(runtime.ReportedAt) {
						break
					}
					time.Sleep(20 * time.Millisecond)
				}
				if !next.CurrentSession.Runtime.ReportedAt.After(runtime.ReportedAt) || *next.CurrentSession.Runtime.UptimeSeconds <= *runtime.UptimeSeconds {
					t.Fatal("periodic uptime did not advance")
				}
				if disconnected, err := app.Disconnect(name); !disconnected || err != nil {
					t.Fatal("disconnect failed")
				}
				reconnected := wait(t, name, v.CurrentSession.ID)
				if *reconnected.CurrentSession.Runtime.UptimeSeconds < *runtime.UptimeSeconds || reconnected.Registration.Kernel != expected {
					t.Fatal("reconnect reset uptime/kernel")
				}
				if err := probe.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				// Cleanup of the first process remains owned by startProbe; a new real process must still report system uptime.
				startProbe(t, binary, control.Addr().String(), name, io.Discard)
				restarted := wait(t, name, reconnected.CurrentSession.ID)
				if *restarted.CurrentSession.Runtime.UptimeSeconds < *runtime.UptimeSeconds {
					t.Fatal("process restart reset system uptime")
				}
			}
		})
	}
}
