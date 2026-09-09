package api

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"strings"
	"testing"
	"time"
)

// Exercise the same application construction as cmd/server, followed by the
// public endpoints used by the generator and desktop to repair a deployment.
func TestStartupWithRetiredSnapshotsAndTemplateRecovery(t *testing.T) {
	for _, scenario := range []string{"fresh", "empty", "missing", "retired"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			templatePath := filepath.Join(root, "probe-templates", "catalog.json")
			devicePath := filepath.Join(root, "devices", "catalog.json")
			write := func(path, body string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			oldTemplate := `{"template_id":"old","name":"路由器模板","version":11,"properties":{"serial":{"name":"序列号","command":"printf original","timeout_seconds":5}},"presentation":{"groups":[],"fields":{},"interface_aliases":{"eth0":"旧名称"}}}`
			oldDevices := `{"schema_version":1,"devices":{"router":{"device_id":"router","version":10,"admission":"managed","name":"管理员名称","model_id":"model","model_name":"型号","bound_template":` + oldTemplate + `,"configuration":{"revision":8,"template":` + oldTemplate + `},"reported":{"DeviceID":"router","Arch":"arm","ReportIntervals":{"serial":5},"Template":{"id":"old"},"Attributes":{"serial":"prior"},"CollectionErrors":{}},"first_seen":"2026-09-08T00:00:00Z","last_seen":"2026-09-09T00:00:00Z"}},"models":{"model":{"model_id":"model","name":"型号","version":2,"aliases":[],"template_id":"old"}}}`
			if scenario != "fresh" {
				write(devicePath, oldDevices)
				write(templatePath+".lock", "1")
			}
			if scenario == "empty" {
				write(templatePath, `{"schema_version":1,"templates":[]}`)
			}
			oldCatalog := `{"schema_version":1,"templates":[` + oldTemplate + `]}`
			if scenario == "retired" {
				write(templatePath, oldCatalog)
			}
			var logs bytes.Buffer
			cfg := management.Config{RepositoryDirectory: root, Gateway: gateway.Config{HeartbeatInterval: 10 * time.Second, Logger: log.New(&logs, "", 0)}}
			app, err := management.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer app.Close()
			a, err := New(app, Config{})
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			http := httptest.NewServer(a)
			defer http.Close()
			request(t, http.URL, "GET", "/api/v1/probe-templates", "", "", 200)
			if scenario != "fresh" {
				p, err := app.Enrollment().Get("router")
				if err != nil || p.Name != "管理员名称" || p.Version != 10 || p.Configuration.Revision != 8 || p.BoundTemplate.ID != "old" || p.BoundTemplate.Version != 11 || p.Reported.Arch != "arm" || p.FirstSeen.Day() != 8 || p.Configuration.Template.Properties["serial"].Command != "printf original" {
					t.Fatal("upgrade changed retained device facts", p, err)
				}
				if m := app.Enrollment().Models(); len(m) != 1 || m[0].TemplateID != "old" || m[0].Version != 2 {
					t.Fatal(m)
				}
				assertCatalogBackup(t, devicePath, oldDevices)
			}
			if scenario == "missing" && !strings.Contains(logs.String(), "template catalog missing") {
				t.Fatal(logs.String())
			}
			input := `{"name":"新模板","properties":{"serial":{"name":"序列号","command":"printf updated"}}}`
			created := data(request(t, http.URL, "POST", "/api/v1/probe-templates", "publish", input, 201))
			id := created["template_id"].(string)
			if scenario == "retired" {
				assertCatalogBackup(t, templatePath, oldCatalog)
				old := data(request(t, http.URL, "GET", "/api/v1/probe-templates/old", "", "", 200))
				if old["version"] != float64(11) {
					t.Fatal(old)
				}
				request(t, http.URL, "PUT", "/api/v1/probe-templates/old", "replace", `{"version":11,"name":"路由器模板","properties":{"serial":{"name":"序列号","command":"printf updated"}}}`, 200)
				request(t, http.URL, "PUT", "/api/v1/probe-templates/old", "stale", `{"version":11,"name":"路由器模板","properties":{"serial":{"name":"序列号","command":"printf stale"}}}`, 409)
			}
			request(t, http.URL, "POST", "/api/v1/probe-templates", "reject-retired", `{"name":"非法","properties":{},"presentation":{"interface_aliases":{}}}`, 400)
			if scenario != "fresh" {
				body, _ := json.Marshal(object{"version": 10, "admission": "managed", "name": "管理员名称", "model_id": "model", "template_id": id, "template_version": 1, "apply_template": true})
				request(t, http.URL, "PUT", "/api/v1/devices/router/profile", "apply", string(body), 200)
				p, _ := app.Enrollment().Get("router")
				if p.BoundTemplate.ID != id || p.Configuration.Revision != 9 || p.Configuration.TemplateGeneration != 1 {
					t.Fatal(p)
				}
			}
			http.Close()
			a.Close()
			app.Close()
			logs.Reset()
			reopened, err := management.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if _, err = reopened.ProbeTemplates().Resolve(id, ""); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(logs.String(), "warning=") {
				t.Fatal("upgrade repeated", logs.String())
			}
		})
	}
}

func assertCatalogBackup(t *testing.T, path, original string) {
	t.Helper()
	files, err := filepath.Glob(path + ".pre-upgrade-*.bak")
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	b, err := os.ReadFile(files[0])
	if err != nil || string(b) != original {
		t.Fatal("backup does not preserve original bytes", err)
	}
	b, err = os.ReadFile(path)
	if err != nil || bytes.Contains(b, []byte("interface_aliases")) || bytes.Contains(b, []byte("ReportIntervals")) {
		t.Fatal("retired fields still persisted", err)
	}
}
