package integration

import (
	"bytes"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"routerprobe/internal/gateway"
)

func waitOnline(t *testing.T, events <-chan gateway.SessionEvent, deviceID string, timeout time.Duration) gateway.SessionEvent {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case event := <-events:
			if event.Type == gateway.EventOnline && event.DeviceID == deviceID {
				return event
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for online event for %s", deviceID)
		}
	}
}

func TestProbeReconnectCreatesNewSession(t *testing.T) {
	probeBinary := os.Getenv("RMP_PROBE_BIN")
	if probeBinary == "" {
		t.Skip("set RMP_PROBE_BIN to the Linux router-probe binary")
	}
	if _, err := os.Stat(probeBinary); err != nil {
		t.Fatalf("RMP_PROBE_BIN: %v", err)
	}

	server, err := gateway.New(gateway.Config{
		HeartbeatInterval: 10 * time.Second,
		Logger:            log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()

	deviceID := "integration-probe"
	var probeLog bytes.Buffer
	command := exec.Command(probeBinary,
		"--server", listener.Addr().String(),
		"--device-id", deviceID,
		"--boot-id", "integration-boot",
		"--hostname", "integration-host",
	)
	command.Stdout = &probeLog
	command.Stderr = &probeLog
	if err := command.Start(); err != nil {
		_ = server.Close()
		t.Fatal(err)
	}

	defer func() {
		if command.Process != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
		_ = server.Close()
		select {
		case err := <-serveDone:
			if err != nil {
				t.Errorf("Serve: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Serve did not stop")
		}
	}()

	first := waitOnline(t, server.Events(), deviceID, 5*time.Second)
	if first.SessionID == "" {
		t.Fatal("first session_id is empty")
	}
	if !server.Disconnect(deviceID) {
		t.Fatal("failed to disconnect first session")
	}
	second := waitOnline(t, server.Events(), deviceID, 7*time.Second)
	if second.SessionID == "" || second.SessionID == first.SessionID {
		t.Fatalf("session_id was not renewed: first=%q second=%q\nprobe log:\n%s",
			first.SessionID, second.SessionID, probeLog.String())
	}
}
