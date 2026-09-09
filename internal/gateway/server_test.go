package gateway

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
)

func startTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	server, err := New(Config{
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
	t.Cleanup(func() {
		_ = server.Close()
		select {
		case err := <-serveDone:
			if err != nil {
				t.Errorf("Serve: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Serve did not stop")
		}
	})
	return server, listener.Addr().String()
}

func writeJSONFrame(t *testing.T, conn net.Conn, messageType uint8, messageID uint64, payload string) {
	writeJSONFrameWithFlags(t, conn, messageType, 0, messageID, payload)
}

func writeJSONFrameWithFlags(t *testing.T, conn net.Conn, messageType uint8, flags uint16, messageID uint64, payload string) {
	t.Helper()
	err := protocol.WriteFrame(conn, protocol.Frame{
		Header:  protocol.Header{Version: protocol.Version1, Type: messageType, Flags: flags, MessageID: messageID},
		Payload: []byte(payload),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTaskDispatchAckAndResult(t *testing.T) {
	server, address := startTestServer(t)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	writeJSONFrame(t, conn, protocol.TypeRegister, 1, validRegister("device-task"))
	registerAck := readFrame(t, conn)
	if registerAck.Header.Type != protocol.TypeRegisterAck {
		t.Fatalf("REGISTER_ACK header = %#v", registerAck.Header)
	}
	select {
	case <-server.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("online event timeout")
	}

	taskID, err := server.CreateExec(context.Background(), "device-task", task.ExecRequest{
		Command: "echo hello", Cwd: "/tmp", Env: map[string]string{"VALUE": "test"}, Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	taskFrame := readFrame(t, conn)
	if taskFrame.Header.Type != protocol.TypeTask || taskFrame.Header.Flags != 0 || taskFrame.Header.MessageID != 2 {
		t.Fatalf("TASK header = %#v", taskFrame.Header)
	}
	var wire taskMessage
	if err := json.Unmarshal(taskFrame.Payload, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.TaskID != taskID || wire.Type != "exec" || wire.Timeout != 5 || wire.Params.Command != "echo hello" || wire.Params.Env["VALUE"] != "test" {
		t.Fatalf("TASK payload = %#v", wire)
	}

	ackPayload, _ := json.Marshal(map[string]interface{}{
		"reply_to": taskFrame.Header.MessageID, "task_id": taskID, "accepted": true, "state": "queued",
	})
	writeJSONFrameWithFlags(t, conn, protocol.TypeTaskAck, protocol.FlagResponse, 2, string(ackPayload))
	resultPayload, _ := json.Marshal(map[string]interface{}{
		"task_id": taskID, "status": "success", "started_at": 10, "finished_at": 11,
		"exit_code": 0, "stdout": "hello\n", "stderr": "", "truncated": false,
		"result": map[string]interface{}{},
	})
	writeJSONFrame(t, conn, protocol.TypeTaskResult, 3, string(resultPayload))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := server.WaitTaskResult(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "success" || result.ExitCode != 0 || result.Stdout != "hello\n" || result.TaskID != taskID {
		t.Fatalf("TASK_RESULT = %#v", result)
	}
}

func TestConnectionWritesSerializeTaskAndHeartbeatAck(t *testing.T) {
	server, address := startTestServer(t)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	writeJSONFrame(t, conn, protocol.TypeRegister, 1, validRegister("device-writer"))
	_ = readFrame(t, conn)
	select {
	case <-server.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("online event timeout")
	}

	taskCreated := make(chan struct {
		id  string
		err error
	}, 1)
	go func() {
		id, err := server.CreateExec(context.Background(), "device-writer", task.ExecRequest{
			Command: "true", Timeout: time.Second,
		})
		taskCreated <- struct {
			id  string
			err error
		}{id: id, err: err}
	}()
	writeJSONFrame(t, conn, protocol.TypeHeartbeat, 2, `{"uptime":1,"uptime_valid":true,"running_tasks":0}`)

	first := readFrame(t, conn)
	second := readFrame(t, conn)
	if first.Header.MessageID != 2 || second.Header.MessageID != 3 {
		t.Fatalf("outgoing message IDs = %d, %d", first.Header.MessageID, second.Header.MessageID)
	}
	seen := map[uint8]bool{first.Header.Type: true, second.Header.Type: true}
	if !seen[protocol.TypeTask] || !seen[protocol.TypeHeartbeatAck] {
		t.Fatalf("outgoing types = 0x%02X, 0x%02X", first.Header.Type, second.Header.Type)
	}
	created := <-taskCreated
	if created.err != nil || created.id == "" {
		t.Fatalf("CreateExec = %q, %v", created.id, created.err)
	}
}

func readFrame(t *testing.T, conn net.Conn) protocol.Frame {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	headerBytes := make([]byte, protocol.HeaderSize)
	if _, err := io.ReadFull(conn, headerBytes); err != nil {
		t.Fatal(err)
	}
	header, err := protocol.DecodeHeader(headerBytes, protocol.MaxControlPayload)
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, header.PayloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatal(err)
	}
	return protocol.Frame{Header: header, Payload: payload}
}

func validRegister(deviceID string) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"device_id":     deviceID,
		"probe_version": "1.0.0",
		"arch":          "x86_64",
		"boot_id":       "test-boot",
		"capabilities":  []string{"managed_config_v1", "telemetry_v2"},
	})
	return string(payload)
}

func TestRegisterAndHeartbeatReplyTo(t *testing.T) {
	server, address := startTestServer(t)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	writeJSONFrame(t, conn, protocol.TypeRegister, 1, validRegister("device-legal"))
	ack := readFrame(t, conn)
	if ack.Header.Type != protocol.TypeRegisterAck || ack.Header.Flags != protocol.FlagResponse || ack.Header.MessageID != 1 {
		t.Fatalf("REGISTER_ACK header = %#v", ack.Header)
	}
	var registerResponse registerAckSuccess
	if err := json.Unmarshal(ack.Payload, &registerResponse); err != nil {
		t.Fatal(err)
	}
	if !registerResponse.Success || registerResponse.ReplyTo != 1 || registerResponse.SessionID == "" || registerResponse.HeartbeatInterval != 10 {
		t.Fatalf("REGISTER_ACK = %#v", registerResponse)
	}

	select {
	case event := <-server.Events():
		if event.Type != EventOnline || event.DeviceID != "device-legal" || event.SessionID != registerResponse.SessionID {
			t.Fatalf("online event = %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("online event timeout")
	}

	writeJSONFrame(t, conn, protocol.TypeHeartbeat, 2, `{"uptime":123,"uptime_valid":true,"running_tasks":0}`)
	heartbeatResponseFrame := readFrame(t, conn)
	if heartbeatResponseFrame.Header.Type != protocol.TypeHeartbeatAck || heartbeatResponseFrame.Header.Flags != protocol.FlagResponse || heartbeatResponseFrame.Header.MessageID != 2 {
		t.Fatalf("HEARTBEAT_ACK header = %#v", heartbeatResponseFrame.Header)
	}
	var heartbeatResponse heartbeatAck
	if err := json.Unmarshal(heartbeatResponseFrame.Payload, &heartbeatResponse); err != nil {
		t.Fatal(err)
	}
	if heartbeatResponse.ReplyTo != 2 || heartbeatResponse.ServerTime < 0 {
		t.Fatalf("HEARTBEAT_ACK = %#v", heartbeatResponse)
	}
}

func TestRegisterValidationFailure(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "missing required field", payload: `{"probe_version":"1.0.0","arch":"x86_64","boot_id":"boot","capabilities":["managed_config_v1","telemetry_v2"]}`},
		{name: "field out of range", payload: validRegister(strings.Repeat("d", 129))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, address := startTestServer(t)
			conn, err := net.Dial("tcp", address)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			writeJSONFrame(t, conn, protocol.TypeRegister, 1, test.payload)
			ack := readFrame(t, conn)
			if ack.Header.Type != protocol.TypeRegisterAck || ack.Header.Flags != protocol.FlagResponse {
				t.Fatalf("header = %#v", ack.Header)
			}
			var response registerAckFailure
			if err := json.Unmarshal(ack.Payload, &response); err != nil {
				t.Fatal(err)
			}
			if response.Success || response.ReplyTo != 1 || response.ErrorCode != "INVALID_REGISTER" || response.RetryAfter < 1 {
				t.Fatalf("REGISTER_ACK failure = %#v", response)
			}
		})
	}
}
