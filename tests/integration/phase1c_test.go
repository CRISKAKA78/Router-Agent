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
	"strings"
	"sync"
	"testing"
	"time"

	"routerprobe/internal/gateway"
	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
)

func waitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("file did not appear: %s", path)
}

func release(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("go"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentTasksOutOfOrderAndResend(t *testing.T) {
	binary := probeBinary(t)
	server, err := gateway.New(gateway.Config{HeartbeatInterval: 10 * time.Second, Logger: log.New(io.Discard, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(listener)
	t.Cleanup(func() { _ = server.Close() })
	var output lockedBuffer
	startProbe(t, binary, listener.Addr().String(), "concurrent", &output)
	old := waitOnline(t, server.Events(), "concurrent", 5*time.Second)
	directory := t.TempDir()
	ids := make([]string, 3)
	for i := range ids {
		gate := filepath.Join(directory, fmt.Sprintf("gate%d", i))
		marker := filepath.Join(directory, fmt.Sprintf("marker%d", i))
		command := fmt.Sprintf(`echo once >> '%s'; while [ ! -f '%s' ]; do sleep 0.02; done; for f in /proc/$$/fd/*; do readlink "$f" >&2; done; printf result%d`, marker, gate, i)
		ids[i], err = server.CreateExec(context.Background(), "concurrent", task.ExecRequest{Command: command, Timeout: 15 * time.Second})
		if err != nil {
			t.Fatal(err)
		}
	}
	// Each marker must exist before any gate opens: a serial worker cannot pass.
	for i := range ids {
		waitFile(t, filepath.Join(directory, fmt.Sprintf("marker%d", i)))
	}
	var sends sync.WaitGroup
	for i := 0; i < 12; i++ {
		sends.Add(1)
		go func(i int) {
			defer sends.Done()
			if err := server.ResendTask(context.Background(), ids[i%3]); err != nil {
				t.Error(err)
			}
		}(i)
	}
	sends.Wait()
	if !server.Disconnect("concurrent") {
		t.Fatal("disconnect")
	}
	newer := waitOnline(t, server.Events(), "concurrent", 5*time.Second)
	if newer.SessionID == old.SessionID {
		t.Fatal("session did not change")
	}
	for _, id := range ids {
		if err := server.ResendTask(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	for i := 2; i >= 0; i-- {
		release(t, filepath.Join(directory, fmt.Sprintf("gate%d", i)))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		got, err := server.WaitTaskResult(ctx, ids[i])
		cancel()
		if err != nil || got.Status != "success" || got.Stdout != fmt.Sprintf("result%d", i) || strings.Contains(got.Stderr, "socket:[") {
			t.Fatalf("result %d: %+v err=%v\n%s", i, got, err, output.String())
		}
		for j := 0; j < i; j++ {
			snapshot, _ := server.TaskSnapshot(ids[j])
			if snapshot.Result != nil {
				t.Fatal("closed gate task finished")
			}
		}
	}
	// Reconnect after all results were delivered. They must replay harmlessly.
	if !server.Disconnect("concurrent") {
		t.Fatal("second disconnect")
	}
	waitOnline(t, server.Events(), "concurrent", 5*time.Second)
	for _, id := range ids {
		if err := server.ResendTask(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	barrier := runExec(t, server, "concurrent", task.ExecRequest{Command: "printf barrier", Timeout: 2 * time.Second})
	if barrier.Stdout != "barrier" {
		t.Fatal(barrier)
	}
	for i, id := range ids {
		bytes, err := os.ReadFile(filepath.Join(directory, fmt.Sprintf("marker%d", i)))
		if err != nil || string(bytes) != "once\n" {
			t.Fatalf("side effect repeated: %q %v", bytes, err)
		}
		snapshot, _ := server.TaskSnapshot(id)
		if snapshot.State != task.StateSuccess || len(snapshot.Dispatches) < 4 {
			t.Fatalf("snapshot=%+v", snapshot)
		}
	}
}

type phase1cPeer struct {
	t              *testing.T
	conn           net.Conn
	sent, received uint64
}

func acceptPhase1cPeer(t *testing.T, listener net.Listener, session string) *phase1cPeer {
	t.Helper()
	_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second))
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	p := &phase1cPeer{t: t, conn: conn}
	frame := p.read()
	if frame.Header.Type != protocol.TypeRegister {
		t.Fatalf("first frame: %+v", frame)
	}
	p.send(protocol.TypeRegisterAck, protocol.FlagResponse, map[string]interface{}{
		"reply_to": 1, "success": true, "session_id": session, "heartbeat_interval": 10,
		"server_time": time.Now().Unix(), "max_control_payload": protocol.MaxControlPayload, "file_chunk_size": 65536,
	})
	return p
}

func (p *phase1cPeer) send(kind uint8, flags uint16, value interface{}) uint64 {
	p.t.Helper()
	p.sent++
	data, err := encodedJSONFrame(kind, flags, p.sent, value)
	if err != nil {
		p.t.Fatal(err)
	}
	_ = p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := p.conn.Write(data); err != nil {
		p.t.Fatal(err)
	}
	return p.sent
}

func (p *phase1cPeer) read() protocol.Frame {
	p.t.Helper()
	for {
		_ = p.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		frame, err := readProtocolFrame(p.conn)
		if err != nil {
			p.t.Fatal(err)
		}
		p.received++
		if frame.Header.MessageID != p.received {
			p.t.Fatalf("message_id=%d expected=%d", frame.Header.MessageID, p.received)
		}
		if frame.Header.Type == protocol.TypeHeartbeat {
			p.send(protocol.TypeHeartbeatAck, protocol.FlagResponse, map[string]interface{}{"reply_to": frame.Header.MessageID, "server_time": time.Now().Unix()})
			continue
		}
		return frame
	}
}

func (p *phase1cPeer) ack(reply uint64, state string) {
	p.t.Helper()
	frame := p.read()
	var ack task.Ack
	if err := json.Unmarshal(frame.Payload, &ack); err != nil {
		p.t.Fatal(err)
	}
	if frame.Header.Type != protocol.TypeTaskAck || frame.Header.Flags != protocol.FlagResponse || ack.ReplyTo != reply || ack.State != state || !ack.Accepted {
		p.t.Fatalf("ACK header=%+v ack=%+v expected state=%s", frame.Header, ack, state)
	}
}

func TestProbeWireDuplicatesConflictsAndOfflineResult(t *testing.T) {
	binary := probeBinary(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var output lockedBuffer
	startProbe(t, binary, listener.Addr().String(), "wire-replay", &output)
	p := acceptPhase1cPeer(t, listener, "one")
	directory := t.TempDir()
	gate, count := filepath.Join(directory, "gate"), filepath.Join(directory, "count")
	command := fmt.Sprintf("echo once >> '%s'; while [ ! -f '%s' ]; do sleep 0.02; done; printf cached", count, gate)
	wire := map[string]interface{}{"task_id": "wire-task", "type": "exec", "timeout": 10, "params": map[string]interface{}{"command": command}}
	id := p.send(protocol.TypeTask, 0, wire)
	p.ack(id, "queued")
	waitFile(t, count)
	// Omitted/empty defaults and unknown fields do not change execution identity.
	wire["created_at"] = 123
	wire["extension"] = "ignored"
	wire["params"] = map[string]interface{}{"command": command, "cwd": "", "env": map[string]string{}, "extension": true}
	id = p.send(protocol.TypeTask, 0, wire)
	p.ack(id, "running")
	for _, change := range []map[string]interface{}{
		{"type": "upload"}, {"timeout": 11}, {"params": map[string]interface{}{"command": "printf wrong"}},
		{"params": map[string]interface{}{"command": command, "env": map[string]string{"X": "Y"}}},
		{"params": map[string]interface{}{"command": command, "cwd": "/tmp"}},
	} {
		conflict := map[string]interface{}{}
		for k, v := range wire {
			conflict[k] = v
		}
		for k, v := range change {
			conflict[k] = v
		}
		reply := p.send(protocol.TypeTask, 0, conflict)
		frame := p.read()
		var failure struct {
			ReplyTo uint64 `json:"reply_to"`
			Code    string `json:"code"`
		}
		_ = json.Unmarshal(frame.Payload, &failure)
		if frame.Header.Type != protocol.TypeError || frame.Header.Flags != protocol.FlagResponse || failure.ReplyTo != reply || failure.Code != "INVALID_PAYLOAD" {
			t.Fatalf("conflict frame=%+v %s", frame.Header, frame.Payload)
		}
	}
	_ = p.conn.Close()
	release(t, gate) // Complete while the TCP connection is absent.
	p = acceptPhase1cPeer(t, listener, "two")
	replay := p.read()
	var got task.Result
	_ = json.Unmarshal(replay.Payload, &got)
	if replay.Header.Type != protocol.TypeTaskResult || replay.Header.Flags != 0 || got.TaskID != "wire-task" || got.Stdout != "cached" {
		t.Fatalf("replay=%s", replay.Payload)
	}
	id = p.send(protocol.TypeTask, 0, wire)
	p.ack(id, "success")
	duplicate := p.read()
	if string(duplicate.Payload) != string(replay.Payload) {
		t.Fatal("cached RESULT changed")
	}
	_ = p.conn.Close()
	p = acceptPhase1cPeer(t, listener, "three")
	again := p.read()
	if string(again.Payload) != string(replay.Payload) {
		t.Fatal("successful prior write prevented reconnect replay")
	}
	bytes, err := os.ReadFile(count)
	if err != nil || string(bytes) != "once\n" {
		t.Fatalf("side effect=%q %v", bytes, err)
	}
}

// This relay discards one complete Probe frame and tears down both TCP sides.
// The real Server cannot observe the dropped ACK/RESULT; later sessions pass
// through unchanged. Probe remains the same process throughout.
func droppingRelay(t *testing.T, upstream string, kind uint8) (string, <-chan struct{}) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dropped, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		pending := true
		for {
			probe, err := listener.Accept()
			if err != nil {
				return
			}
			server, err := net.Dial("tcp", upstream)
			if err != nil {
				_ = probe.Close()
				return
			}
			func() {
				copied := make(chan struct{})
				go func() { defer close(copied); _, _ = io.Copy(probe, server); _ = probe.Close() }()
				defer func() { _ = probe.Close(); _ = server.Close(); <-copied }()
				for {
					frame, err := readProtocolFrame(probe)
					if err != nil {
						return
					}
					if pending && frame.Header.Type == kind {
						pending = false
						close(dropped)
						return
					}
					if protocol.WriteFrame(server, frame) != nil {
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("relay did not stop")
		}
	})
	return listener.Addr().String(), dropped
}

func TestLostAckOrResultReplayedToRealServer(t *testing.T) {
	binary := probeBinary(t)
	for _, kind := range []uint8{protocol.TypeTaskAck, protocol.TypeTaskResult} {
		t.Run(fmt.Sprintf("drop-%02x", kind), func(t *testing.T) {
			server, err := gateway.New(gateway.Config{Logger: log.New(io.Discard, "", 0)})
			if err != nil {
				t.Fatal(err)
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			go server.Serve(listener)
			t.Cleanup(func() { _ = server.Close() })
			address, dropped := droppingRelay(t, listener.Addr().String(), kind)
			var output lockedBuffer
			startProbe(t, binary, address, "lost-frame", &output)
			old := waitOnline(t, server.Events(), "lost-frame", 5*time.Second)
			count := filepath.Join(t.TempDir(), "count")
			id, err := server.CreateExec(context.Background(), "lost-frame", task.ExecRequest{
				Command: fmt.Sprintf("echo once >> '%s'; sleep 0.2; printf survived", count), Timeout: 5 * time.Second,
			})
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-dropped:
			case <-time.After(5 * time.Second):
				t.Fatal("frame was not dropped")
			}
			newer := waitOnline(t, server.Events(), "lost-frame", 5*time.Second)
			if old.SessionID == newer.SessionID {
				t.Fatal("session did not change")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := server.WaitTaskResult(ctx, id)
			if err != nil || got.Stdout != "survived" || got.Status != "success" {
				t.Fatalf("result=%+v err=%v\n%s", got, err, output.String())
			}
			snapshot, _ := server.TaskSnapshot(id)
			if kind == protocol.TypeTaskAck && snapshot.Ack != nil {
				t.Fatal("lost ACK was fabricated on reconnect")
			}
			if err := server.ResendTask(context.Background(), id); err != nil {
				t.Fatal(err)
			}
			runExec(t, server, "lost-frame", task.ExecRequest{Command: "true", Timeout: time.Second})
			bytes, err := os.ReadFile(count)
			if err != nil || string(bytes) != "once\n" {
				t.Fatalf("side effect=%q %v", bytes, err)
			}
		})
	}
}
