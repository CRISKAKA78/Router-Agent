//go:build linux

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"routerprobe/internal/gateway"
	"routerprobe/internal/overlay"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A shell fixture substitutes only the EasyTier executable, not Probe or its
// transport. This proves process isolation and task replay, not VPN connectivity.
func TestNetworkAgentProbeDisconnectAndReplay(t *testing.T) {
	binary := probeBinary(t)
	server, e := gateway.New(gateway.Config{Logger: log.New(io.Discard, "", 0)})
	if e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	defer func() { server.Close(); <-done }()
	id := "network-bootstrap-probe"
	probe := startProbe(t, binary, listener.Addr().String(), id, io.Discard)
	waitOnline(t, server.Events(), id, 5*time.Second)
	root := filepath.Join(t.TempDir(), "engine")
	machine := overlay.UUID()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	inspect := overlay.AgentRequest{Action: "inspect", Directory: root, MachineID: machine}
	taskID, e := server.CreateNetworkAgent(ctx, id, inspect)
	if e != nil {
		t.Fatal(e)
	}
	r, e := server.WaitTaskResult(ctx, taskID)
	if e != nil || r.Status != "success" {
		t.Fatal(r, e)
	}
	if _, e = os.Stat(root); !os.IsNotExist(e) {
		t.Fatal("inspect creates remote state", e)
	}
	inspect.Action = "prepare"
	taskID, e = server.CreateNetworkAgent(ctx, id, inspect)
	if e != nil {
		t.Fatal(e)
	}
	r, e = server.WaitTaskResult(ctx, taskID)
	if e != nil || r.Status != "success" {
		t.Fatal(r, e)
	}
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then echo 'easytier-core 2.6.4'; exit 0; fi
while [ "$#" -gt 0 ]; do if [ "$1" = "--config-dir" ]; then shift; dir="$1"; fi; shift; done
printf 'launch\n' >> "$dir/launches"
exec sleep 60
`
	if e = os.WriteFile(filepath.Join(root, "easytier-core"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat("/dev/net/tun"); e != nil {
		t.Skip("typed inspect/prepare passed; detached start requires a TUN device node")
	}
	inspect.Action = "start"
	inspect.ConfigServer = "tcp://127.0.0.1:22020/test"
	taskID, e = server.CreateNetworkAgent(ctx, id, inspect)
	if e != nil {
		t.Fatal(e)
	}
	r, e = server.WaitTaskResult(ctx, taskID)
	if e != nil || r.Status != "success" {
		t.Fatal(r, e)
	}
	var info overlay.AgentInfo
	if json.Unmarshal([]byte(r.Stdout), &info) != nil || !info.Running {
		t.Fatal(r)
	}
	pidData, e := os.ReadFile(filepath.Join(root, "agent.pid"))
	if e != nil {
		t.Fatal(e)
	}
	var pid int
	if _, e = fmt.Sscan(string(pidData), &pid); e != nil || pid <= 1 {
		t.Fatal("invalid fixture pid")
	}
	defer syscall.Kill(pid, syscall.SIGTERM)
	if e = server.ResendTask(ctx, taskID); e != nil {
		t.Fatal(e)
	}
	time.Sleep(150 * time.Millisecond)
	count, e := os.ReadFile(filepath.Join(root, "networks", "launches"))
	if e != nil || strings.Count(string(count), "launch") != 1 {
		t.Fatalf("replayed engine launch %q %v", count, e)
	}
	server.Close()
	probe.Process.Kill()
	time.Sleep(150 * time.Millisecond)
	if e = syscall.Kill(pid, 0); e != nil {
		t.Fatal("management/Probe exit killed detached engine", e)
	}
}
