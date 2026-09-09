package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"routerprobe/internal/device"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/probetemplate"
	"strconv"
	"testing"
	"time"
)

func TestManagedPhysicalPortCounters(t *testing.T) {
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
	defer func() { app.Close(); <-done }()
	path := filepath.Join(t.TempDir(), "count")
	if e = os.WriteFile(path, []byte("9007199254740993"), 0600); e != nil {
		t.Fatal(e)
	}
	command := fmt.Sprintf(`n=$(cat '%s'); n=$((n+10)); printf '%%s\n' "$n" > '%s'; printf 'p1\t%%s\t%%s\n' "$n" "$n"`, path, path)
	var input probetemplate.Input
	if e = json.Unmarshal([]byte(`{"name":"physical bytes","properties":{},"monitoring":{"network_seconds":1},"switch_probe":{"backend":"command","command":"printf 'p1\\t-\\t-\\teth1\\t-\\tup\\tunknown\\t1000\\tfull\\n'","ports":[{"id":"p1","display_name":"LAN1","role":"external"}],"counters":{"backend":"command","bits":64,"basis":"fixture cumulative bytes"}}}`), &input); e != nil {
		t.Fatal(e)
	}
	input.SwitchProbe.Counters.Command = command
	tpl, e := app.ProbeTemplates().Put("", 0, input)
	if e != nil {
		t.Fatal(e)
	}
	startProbe(t, binary, listener.Addr().String(), "physical-counter", io.Discard)
	adoptProbe(t, app, "physical-counter", tpl.ID, nil)
	first := waitManagedMetric(t, app, "physical-counter", func(d device.Snapshot, m map[string]device.Metric) bool {
		total, _ := strconv.ParseUint(m["switch_p1_rx_bytes"].Value, 10, 64)
		raw, _ := strconv.ParseUint(m["switch_p1_rx_raw_bytes"].Value, 10, 64)
		return m["switch_p1_state"].Value == "up" && m["switch_p1_rx_bytes_per_sec"].Status == "ok" && raw > 9007199254740993 && total >= 10 && raw-total == 9007199254741003
	})
	if ok, err := app.Disconnect("physical-counter"); err != nil || !ok {
		t.Fatalf("disconnect: %v %v", ok, err)
	}
	firstTotal, _ := strconv.ParseUint(device.EffectiveMetrics(first.LatestSession, time.Now())["switch_p1_rx_bytes"].Value, 10, 64)
	waitManagedMetric(t, app, "physical-counter", func(d device.Snapshot, m map[string]device.Metric) bool {
		total, _ := strconv.ParseUint(m["switch_p1_rx_bytes"].Value, 10, 64)
		raw, _ := strconv.ParseUint(m["switch_p1_rx_raw_bytes"].Value, 10, 64)
		return d.LatestSession.ID != first.LatestSession.ID && total > firstTotal && raw-total == 9007199254741003
	})
}
