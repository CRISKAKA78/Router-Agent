package gateway

import (
	"strings"
	"testing"
)

func TestNeighborEventValidation(t *testing.T) {
	raw := `{"event":"neighbors","config_revision":1,"interval_seconds":30,"age_ms":10,"unclassified":[],"domains":[{"id":"local","scope":"broadcast","interface":"br0","status":"ok","reason":"","rows":[{"ip":"192.0.2.2","mac":"02:00:00:00:00:02","source":"arp","state":"cached"}]}]}`
	if _, _, e := parseNeighbors([]byte(raw)); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(raw, "192.0.2.2", "not-ip", 1), strings.Replace(raw, "02:00:00:00:00:02", "ff:ff:ff:ff:ff:ff", 1), strings.Replace(raw, "broadcast", "uplink", 1), strings.Replace(raw, `"config_revision":1`, `"config_revision":0`, 1), strings.Replace(raw, `"unclassified":[]`, `"unclassified":[{"interface":"../x"}]`, 1)} {
		if _, _, e := parseNeighbors([]byte(bad)); e == nil {
			t.Fatal(bad)
		}
	}
}
