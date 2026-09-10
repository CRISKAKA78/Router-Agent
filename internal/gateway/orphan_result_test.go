package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
)

func resultTestPeer(t *testing.T, server *Server, address string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	writeJSONFrame(t, conn, protocol.TypeRegister, 1, validRegister("result-device"))
	if f := readFrame(t, conn); f.Header.Type != protocol.TypeRegisterAck {
		t.Fatalf("register: %+v", f)
	}
	select {
	case <-server.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("registration did not publish")
	}
	return conn
}

func resultTestPayload(id, status string) string {
	return fmt.Sprintf(`{"task_id":%q,"status":%q,"started_at":1,"finished_at":2,"exit_code":0,"stdout":"private-output","stderr":"","truncated":false,"result":{}}`, id, status)
}

func TestOrphanTaskResultsKeepSessionOnline(t *testing.T) {
	server, address := startTestServer(t)
	var output bytes.Buffer
	server.config.Logger.SetOutput(&output)
	conn := resultTestPeer(t, server, address)
	messageID := uint64(2)
	for _, status := range []string{"success", "failed", "timeout", "success"} {
		// Repeated orphan results must not create tasks or emit ERROR frames.
		writeJSONFrame(t, conn, protocol.TypeTaskResult, messageID, resultTestPayload("old-"+status, status))
		messageID++
	}
	fileResult := strings.Replace(resultTestPayload("old-file", "success"), `"result":{}`, `"result":{"transfer_id":"old-transfer","size":12,"sha256":"old-digest"}`, 1)
	writeJSONFrame(t, conn, protocol.TypeTaskResult, messageID, fileResult)
	messageID++
	writeJSONFrame(t, conn, protocol.TypeHeartbeat, messageID, `{"uptime":10,"uptime_valid":true,"running_tasks":0}`)
	f := readFrame(t, conn)
	var ack heartbeatAck
	if f.Header.Type != protocol.TypeHeartbeatAck || f.Header.MessageID != 2 || json.Unmarshal(f.Payload, &ack) != nil || ack.ReplyTo != messageID {
		t.Fatalf("orphan results interrupted heartbeat: header=%+v payload=%s", f.Header, f.Payload)
	}
	for _, id := range []string{"old-success", "old-failed", "old-timeout", "old-file"} {
		if _, err := server.TaskSnapshot(id); !errors.Is(err, task.ErrTaskNotFound) {
			t.Fatalf("orphan %s was imported: %v", id, err)
		}
	}
	// A new task on the same TCP session still completes normally.
	id, err := server.CreateExec(context.Background(), "result-device", task.ExecRequest{Command: "true", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if f := readFrame(t, conn); f.Header.Type != protocol.TypeTask {
		t.Fatalf("new task: %+v", f)
	}
	writeJSONFrame(t, conn, protocol.TypeTaskResult, messageID+1, resultTestPayload(id, "success"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if result, err := server.WaitTaskResult(ctx, id); err != nil || result.Status != "success" {
		t.Fatalf("new result: %+v %v", result, err)
	}
	server.Close() // Wait for all log writers before inspecting the buffer.
	logs := output.String()
	if strings.Count(logs, "ignored=TASK_RESULT") != 5 || !strings.Contains(logs, `task_id="old-success"`) || !strings.Contains(logs, "reason=task_not_found") || !strings.Contains(logs, "message_id=2") {
		t.Fatalf("missing orphan diagnostics: %s", logs)
	}
	if strings.Contains(logs, "private-output") {
		t.Fatal("task output leaked into diagnostics")
	}
}

func TestOrphanHandlingPreservesResultValidation(t *testing.T) {
	for _, name := range []string{"invalid-json", "invalid-status", "missing-result", "response-flag", "wrong-device", "undispatched", "rejected", "conflicting"} {
		t.Run(name, func(t *testing.T) {
			server, address := startTestServer(t)
			conn := resultTestPeer(t, server, address)
			id := "unknown"
			var before task.Snapshot
			known := name == "wrong-device" || name == "undispatched" || name == "rejected" || name == "conflicting"
			if known {
				deviceID := "result-device"
				if name == "wrong-device" {
					deviceID = "another-device"
				}
				spec, err := server.tasks.NewExec(deviceID, task.ExecRequest{Command: "true", Timeout: time.Second})
				if err != nil {
					t.Fatal(err)
				}
				id = spec.ID
				if name != "undispatched" {
					if err := server.tasks.MarkDispatched(id, "previous-session", 2); err != nil {
						t.Fatal(err)
					}
				}
				if name == "rejected" {
					if err := server.tasks.HandleAck(deviceID, "previous-session", task.Ack{ReplyTo: 2, TaskID: id, State: "rejected"}); err != nil {
						t.Fatal(err)
					}
				}
				if name == "conflicting" {
					result, err := parseTaskResult([]byte(resultTestPayload(id, "success")))
					if err != nil {
						t.Fatal(err)
					}
					result.Stdout = "original"
					if err := server.tasks.HandleResult(deviceID, result); err != nil {
						t.Fatal(err)
					}
				}
				before, _ = server.TaskSnapshot(id)
			}
			payload := resultTestPayload(id, "success")
			flags := uint16(0)
			switch name {
			case "invalid-json":
				payload = `{`
			case "invalid-status":
				payload = resultTestPayload(id, "running")
			case "missing-result":
				payload = strings.Replace(payload, `,"result":{}`, "", 1)
			case "response-flag":
				flags = protocol.FlagResponse
			}
			writeJSONFrameWithFlags(t, conn, protocol.TypeTaskResult, flags, 2, payload)
			f := readFrame(t, conn)
			var failure errorResponse
			if f.Header.Type != protocol.TypeError || json.Unmarshal(f.Payload, &failure) != nil || failure.ReplyTo != 2 || failure.Code != "INVALID_PAYLOAD" {
				t.Fatalf("invalid result was not rejected: %+v %s", f.Header, f.Payload)
			}
			var b [1]byte
			if _, err := conn.Read(b[:]); err == nil {
				t.Fatal("invalid result left connection open")
			} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				t.Fatal("invalid result did not close connection")
			}
			if known {
				after, err := server.TaskSnapshot(id)
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("rejected result changed task: before=%+v after=%+v err=%v", before, after, err)
				}
			} else if _, err := server.TaskSnapshot(id); !errors.Is(err, task.ErrTaskNotFound) {
				t.Fatalf("malformed orphan created task: %v", err)
			}
		})
	}
}
