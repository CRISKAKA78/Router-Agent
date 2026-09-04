package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"routerprobe/internal/gateway"
	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
)

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func probeBinary(t *testing.T) string {
	t.Helper()
	binary := os.Getenv("RMP_PROBE_BIN")
	if binary == "" {
		t.Skip("set RMP_PROBE_BIN to the Linux router-probe binary")
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("RMP_PROBE_BIN: %v", err)
	}
	return binary
}

func startProbe(t *testing.T, binary, address, deviceID string, output io.Writer) *exec.Cmd {
	t.Helper()
	command := exec.Command(binary,
		"--server", address,
		"--device-id", deviceID,
		"--boot-id", "phase1b-boot",
		"--hostname", "phase1b-host",
	)
	command.Stdout = output
	command.Stderr = output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if command.Process != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	return command
}

func runExec(t *testing.T, server *gateway.Server, deviceID string, request task.ExecRequest) task.Result {
	t.Helper()
	taskID, err := server.CreateExec(context.Background(), deviceID, request)
	if err != nil {
		t.Fatal(err)
	}
	wait := request.Timeout + 5*time.Second
	if wait < 5*time.Second {
		wait = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	result, err := server.WaitTaskResult(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if result.TaskID != taskID {
		t.Fatalf("TASK_RESULT task_id = %q, want %q", result.TaskID, taskID)
	}
	return result
}

func TestTaskExecEndToEnd(t *testing.T) {
	binary := probeBinary(t)
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

	deviceID := "phase1b-exec-probe"
	var probeLog lockedBuffer
	startProbe(t, binary, listener.Addr().String(), deviceID, &probeLog)
	waitOnline(t, server.Events(), deviceID, 5*time.Second)

	success := runExec(t, server, deviceID, task.ExecRequest{Command: "echo hello", Timeout: 5 * time.Second})
	if success.Status != "success" || success.ExitCode != 0 || !strings.Contains(success.Stdout, "hello") {
		t.Fatalf("success result = %#v\nprobe log:\n%s", success, probeLog.String())
	}

	descriptors := runExec(t, server, deviceID, task.ExecRequest{
		Command: `for f in /proc/$$/fd/*; do readlink "$f"; done; true`, Timeout: 5 * time.Second,
	})
	if descriptors.Status != "success" || descriptors.Stdout == "" || strings.Contains(descriptors.Stdout, "socket:[") {
		t.Fatalf("exec inherited a control socket: %#v", descriptors)
	}

	failed := runExec(t, server, deviceID, task.ExecRequest{Command: `/bin/sh -c "exit 7"`, Timeout: 5 * time.Second})
	if failed.Status != "failed" || failed.ExitCode != 7 {
		t.Fatalf("failed result = %#v", failed)
	}

	stderrResult := runExec(t, server, deviceID, task.ExecRequest{Command: "printf only-stderr >&2", Timeout: 5 * time.Second})
	if stderrResult.Status != "success" || stderrResult.Stdout != "" || stderrResult.Stderr != "only-stderr" {
		t.Fatalf("stderr result = %#v", stderrResult)
	}

	workingDirectory := runExec(t, server, deviceID, task.ExecRequest{
		Command: `printf "%s:%s" "$PWD" "$RMP_VALUE"`, Cwd: "/tmp",
		Env: map[string]string{"RMP_VALUE": "env-ok"}, Timeout: 5 * time.Second,
	})
	if workingDirectory.Status != "success" || workingDirectory.Stdout != "/tmp:env-ok" {
		t.Fatalf("cwd/env result = %#v", workingDirectory)
	}

	pidPath := fmt.Sprintf("/tmp/rmp-phase1b-timeout-%d.pid", time.Now().UnixNano())
	timeoutResult := runExec(t, server, deviceID, task.ExecRequest{
		Command: "echo $$ > " + pidPath + "; sleep 30", Timeout: time.Second,
	})
	if timeoutResult.Status != "timeout" {
		t.Fatalf("timeout result = %#v", timeoutResult)
	}
	pidBytes, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(pidPath)
	childPID := strings.TrimSpace(string(pidBytes))
	if _, err := strconv.Atoi(childPID); err != nil {
		t.Fatalf("timeout pid %q: %v", childPID, err)
	}
	if err := exec.Command("/bin/sh", "-c", "kill -0 "+childPID).Run(); err == nil {
		t.Fatalf("timed out shell %s is still alive", childPID)
	}

	large := runExec(t, server, deviceID, task.ExecRequest{
		Command: "head -c 1100000 /dev/zero & head -c 1100000 /dev/zero >&2 & wait", Timeout: 10 * time.Second,
	})
	if large.Status != "success" || !large.Truncated || len(large.Stdout) == 0 || len(large.Stderr) == 0 ||
		len(large.Stdout) > 1024*1024 || len(large.Stderr) > 1024*1024 {
		t.Fatalf("large output result: status=%s truncated=%t stdout=%d stderr=%d",
			large.Status, large.Truncated, len(large.Stdout), len(large.Stderr))
	}

	longRunning := runExec(t, server, deviceID, task.ExecRequest{Command: "sleep 12; printf alive", Timeout: 20 * time.Second})
	if longRunning.Status != "success" || longRunning.Stdout != "alive" {
		t.Fatalf("long-running result = %#v", longRunning)
	}
	logs := probeLog.String()
	if !strings.Contains(logs, "sent=HEARTBEAT") || !strings.Contains(logs, "running_tasks=1") ||
		!strings.Contains(logs, "received=HEARTBEAT_ACK") {
		t.Fatalf("heartbeat was not processed during exec\nprobe log:\n%s", logs)
	}
}

func readProtocolFrame(conn net.Conn) (protocol.Frame, error) {
	headerBytes := make([]byte, protocol.HeaderSize)
	if _, err := io.ReadFull(conn, headerBytes); err != nil {
		return protocol.Frame{}, err
	}
	header, err := protocol.DecodeHeader(headerBytes, protocol.MaxControlPayload)
	if err != nil {
		return protocol.Frame{}, err
	}
	payload := make([]byte, header.PayloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return protocol.Frame{}, err
	}
	return protocol.Frame{Header: header, Payload: payload}, nil
}

func encodedJSONFrame(messageType uint8, flags uint16, messageID uint64, value interface{}) ([]byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return protocol.EncodeFrame(protocol.Frame{
		Header:  protocol.Header{Version: protocol.Version1, Type: messageType, Flags: flags, MessageID: messageID},
		Payload: payload,
	})
}

func TestProbeRejectsUnsupportedTask(t *testing.T) {
	binary := probeBinary(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var probeLog lockedBuffer
	startProbe(t, binary, listener.Addr().String(), "phase1b-reject-probe", &probeLog)
	if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	register, err := readProtocolFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	if register.Header.Type != protocol.TypeRegister || register.Header.MessageID != 1 {
		t.Fatalf("REGISTER header = %#v", register.Header)
	}

	registerAck, err := encodedJSONFrame(protocol.TypeRegisterAck, protocol.FlagResponse, 1, map[string]interface{}{
		"reply_to": 1, "success": true, "session_id": "sess_phase1b_fake", "heartbeat_interval": 10,
		"server_time": time.Now().Unix(), "max_control_payload": protocol.MaxControlPayload, "file_chunk_size": 65536,
	})
	if err != nil {
		t.Fatal(err)
	}
	unsupportedTask, err := encodedJSONFrame(protocol.TypeTask, 0, 2, map[string]interface{}{
		"task_id": "unsupported-1", "type": "upload", "created_at": time.Now().Unix(), "timeout": 5,
		"params": map[string]interface{}{},
	})
	if err != nil {
		t.Fatal(err)
	}
	combined := append(registerAck, unsupportedTask...)
	if _, err := conn.Write(combined); err != nil {
		t.Fatal(err)
	}
	rejected, err := readProtocolFrame(conn)
	if err != nil {
		t.Fatalf("read rejected TASK_ACK: %v\nprobe log:\n%s", err, probeLog.String())
	}
	var rejectedPayload struct {
		ReplyTo  uint64 `json:"reply_to"`
		TaskID   string `json:"task_id"`
		Accepted bool   `json:"accepted"`
		State    string `json:"state"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(rejected.Payload, &rejectedPayload); err != nil {
		t.Fatal(err)
	}
	if rejected.Header.Type != protocol.TypeTaskAck || rejected.Header.Flags != protocol.FlagResponse ||
		rejected.Header.MessageID != 2 || rejectedPayload.ReplyTo != 2 || rejectedPayload.TaskID != "unsupported-1" ||
		rejectedPayload.Accepted || rejectedPayload.State != "rejected" ||
		!strings.Contains(rejectedPayload.Message, "unsupported task type") {
		t.Fatalf("rejected TASK_ACK header=%#v payload=%#v", rejected.Header, rejectedPayload)
	}

	supportedTask, err := encodedJSONFrame(protocol.TypeTask, 0, 3, map[string]interface{}{
		"task_id": "supported-after-reject", "type": "exec", "created_at": time.Now().Unix(), "timeout": 5,
		"params": map[string]interface{}{"command": "printf ok", "env": map[string]string{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(supportedTask); err != nil {
		t.Fatal(err)
	}
	accepted, err := readProtocolFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	var acceptedPayload struct {
		ReplyTo  uint64 `json:"reply_to"`
		TaskID   string `json:"task_id"`
		Accepted bool   `json:"accepted"`
		State    string `json:"state"`
	}
	if err := json.Unmarshal(accepted.Payload, &acceptedPayload); err != nil {
		t.Fatal(err)
	}
	if accepted.Header.Type != protocol.TypeTaskAck || accepted.Header.Flags != protocol.FlagResponse ||
		accepted.Header.MessageID != 3 || acceptedPayload.ReplyTo != 3 ||
		acceptedPayload.TaskID != "supported-after-reject" || !acceptedPayload.Accepted || acceptedPayload.State != "queued" {
		t.Fatalf("accepted TASK_ACK header=%#v payload=%#v", accepted.Header, acceptedPayload)
	}
	resultFrame, err := readProtocolFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	var resultPayload task.Result
	if err := json.Unmarshal(resultFrame.Payload, &resultPayload); err != nil {
		t.Fatal(err)
	}
	if resultFrame.Header.Type != protocol.TypeTaskResult || resultFrame.Header.Flags != 0 ||
		resultFrame.Header.MessageID != 4 || resultPayload.TaskID != "supported-after-reject" ||
		resultPayload.Status != "success" || resultPayload.Stdout != "ok" {
		t.Fatalf("TASK_RESULT header=%#v payload=%#v", resultFrame.Header, resultPayload)
	}
}
