package integration

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"routerprobe/internal/gateway"
	"routerprobe/internal/task"
)

func TestProbeSurvivesServerRestartWithCachedResults(t *testing.T) {
	binary := probeBinary(t)
	var probeLog, serverLog lockedBuffer
	// Recreate the whole Gateway (including empty Task/File Services) on the
	// same address, leaving the actual Probe process and its cache alive.
	start := func(address string) *gateway.Server {
		t.Helper()
		server, err := gateway.New(gateway.Config{HeartbeatInterval: 10 * time.Second, Logger: log.New(&serverLog, "", 0)})
		if err != nil {
			t.Fatal(err)
		}
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- server.Serve(listener) }()
		t.Cleanup(func() {
			server.Close()
			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(2 * time.Second):
				t.Error("Server did not stop")
			}
		})
		return server
	}
	// Reserve an ephemeral port without depending on a production listener.
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.Addr().String()
	reservation.Close()
	first := start(address)
	startProbe(t, binary, address, "restart-probe", &probeLog)
	initial := waitOnline(t, first.Events(), "restart-probe", 5*time.Second)
	dir := t.TempDir()
	marker, gate := filepath.Join(dir, "executions"), filepath.Join(dir, "gate")
	completed := runExec(t, first, "restart-probe", task.ExecRequest{
		Command: fmt.Sprintf("echo completed >> '%s'; printf before-restart", marker), Timeout: 2 * time.Second,
	})
	if completed.Status != "success" || completed.Stdout != "before-restart" {
		t.Fatalf("initial task failed: %+v", completed)
	}
	pendingID, err := first.CreateExec(context.Background(), "restart-probe", task.ExecRequest{
		Command: fmt.Sprintf("echo pending >> '%s'; touch '%s/started'; while [ ! -f '%s' ]; do sleep 0.02; done; printf after-restart", marker, dir, gate), Timeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitFile(t, filepath.Join(dir, "started"))
	first.Close()
	second := start(address)
	current := waitOnline(t, second.Events(), "restart-probe", 7*time.Second)
	if current.SessionID == initial.SessionID {
		t.Fatal("Server restart did not create a new session")
	}
	release(t, gate) // The already-running task finishes against the empty Server.
	deadline := time.Now().Add(5 * time.Second)
	for {
		logs := serverLog.String()
		if strings.Contains(logs, fmt.Sprintf("task_id=%q reason=task_not_found", completed.TaskID)) && strings.Contains(logs, fmt.Sprintf("task_id=%q reason=task_not_found", pendingID)) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("both old results were not handled\nServer:\n%s\nProbe:\n%s", logs, probeLog.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, id := range []string{completed.TaskID, pendingID} {
		if _, err := second.TaskSnapshot(id); !errors.Is(err, task.ErrTaskNotFound) {
			t.Fatalf("old task was recreated: %s %v", id, err)
		}
	}
	newResult := runExec(t, second, "restart-probe", task.ExecRequest{Command: "printf healthy", Timeout: 2 * time.Second})
	if newResult.Status != "success" || newResult.Stdout != "healthy" {
		t.Fatalf("new task failed after restart: %+v", newResult)
	}
	// Wait past a complete heartbeat interval: immediate post-registration
	// activity alone would not catch the recurring disconnect in this incident.
	select {
	case event := <-second.Events():
		t.Fatalf("unexpected session change after restart: %+v\n%s", event, probeLog.String())
	case <-time.After(11 * time.Second):
	}
	if strings.Count(probeLog.String(), "received=HEARTBEAT_ACK") < 2 || strings.Contains(probeLog.String(), "state=PROTOCOL_ERROR") {
		t.Fatalf("heartbeats did not survive restart: %s", probeLog.String())
	}
	// A later ordinary reconnect still replays the same cache harmlessly.
	if !second.Disconnect("restart-probe") {
		t.Fatal("disconnect failed")
	}
	if next := waitOnline(t, second.Events(), "restart-probe", 5*time.Second); next.SessionID == current.SessionID {
		t.Fatal("session was not renewed")
	}
	if got := runExec(t, second, "restart-probe", task.ExecRequest{Command: "printf reconnected", Timeout: 2 * time.Second}); got.Stdout != "reconnected" {
		t.Fatal(got)
	}
	if _, err := second.TaskSnapshot(completed.TaskID); !errors.Is(err, task.ErrTaskNotFound) {
		t.Fatalf("reconnect imported old result: %v", err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "completed\npending\n" {
		t.Fatalf("old task side effects repeated: %q %v", data, err)
	}
}
