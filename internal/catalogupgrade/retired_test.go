package catalogupgrade_test

import (
	"os"
	"path/filepath"
	"routerprobe/internal/enrollment"
	"routerprobe/internal/probetemplate"
	"strings"
	"testing"
)

func TestInvalidCatalogIsNotRewrittenByUpgrade(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		devices    bool
	}{
		{"template-unknown", `{"schema_version":1,"templates":[{"template_id":"t","name":"t","version":1,"properties":{},"presentation":{"interface_aliases":{},"future_field":true}}]}`, false},
		{"template-duplicate", `{"schema_version":1,"templates":[{"template_id":"t","name":"t","version":1,"properties":{},"presentation":{"interface_aliases":{},"interface_aliases":{}}}]}`, false},
		{"template-invalid", `{"schema_version":1,"templates":[{"template_id":"t","name":"t","version":0,"properties":{},"presentation":{"interface_aliases":{}}}]}`, false},
		{"device-unknown", `{"schema_version":1,"devices":{"d":{"device_id":"d","version":1,"admission":"pending","reported":{"ReportIntervals":null,"FutureField":true}}},"models":{}}`, true},
		{"device-invalid", `{"schema_version":1,"devices":{"d":{"device_id":"d","version":0,"admission":"pending","reported":{"ReportIntervals":null}}},"models":{}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "catalog.json")
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			if tc.devices {
				s, e := enrollment.Open(path)
				err = e
				if s != nil {
					s.Close()
				}
			} else {
				s, e := probetemplate.Open(path)
				err = e
				if s != nil {
					s.Close()
				}
			}
			if err == nil || !strings.Contains(err.Error(), path) {
				t.Fatal("expected contextual error", err)
			}
			b, e := os.ReadFile(path)
			if e != nil || string(b) != tc.body {
				t.Fatal("invalid data was changed", e)
			}
			backups, e := filepath.Glob(path + ".pre-upgrade-*.bak")
			if e != nil || len(backups) != 0 {
				t.Fatal("invalid data was marked as upgraded", backups, e)
			}
		})
	}
}
