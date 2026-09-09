package integration

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"routerprobe/internal/filetransfer"
	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
)

func TestFileSourceMutationBeforeStreaming(t *testing.T) {
	for _, kind := range []string{"upload", "download"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "source")
			dst := filepath.Join(dir, "target")
			data := bytes.Repeat([]byte{0, 1, 128, 255}, 50000)
			if e := os.WriteFile(src, data, 0600); e != nil {
				t.Fatal(e)
			}
			changed := make(chan error, 1)
			var once sync.Once
			s, device, out := relayedFileServer(t, func(fromProbe bool, f *protocol.Frame) bool {
				if f.Header.Type == protocol.TypeFileAck && fromProbe == (kind == "upload") {
					var a filetransfer.Ack
					if json.Unmarshal(f.Payload, &a) == nil && a.Status == "ready" {
						once.Do(func() { changed <- os.WriteFile(src, bytes.Repeat([]byte{42}, len(data)), 0600) })
					}
				}
				return true
			})
			var id string
			var err error
			if kind == "upload" {
				id, err = s.CreateUpload(context.Background(), device, filetransfer.UploadRequest{SourcePath: src, RemotePath: dst, Mode: "0600", Timeout: 10 * time.Second})
			} else {
				id, err = s.CreateDownload(context.Background(), device, filetransfer.DownloadRequest{RemotePath: src, ResultName: "source", TargetPath: dst, Timeout: 10 * time.Second})
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-changed:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("ready not reached")
			}
			if r := fileResult(t, s, id); r.Status != "failed" {
				t.Fatal(r, out.String())
			}
			if _, err = os.Stat(dst); !os.IsNotExist(err) {
				t.Fatal("changed source published", err)
			}
			runExec(t, s, device, task.ExecRequest{Command: "true", Timeout: time.Second})
			left, _ := filepath.Glob(filepath.Join(dir, ".rmp-transfer-*"))
			if len(left) != 0 {
				t.Fatal("temp leak", left)
			}
		})
	}
}

func expectProtocolClose(t *testing.T, c net.Conn) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		f, err := readProtocolFrame(c)
		if err != nil {
			if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				t.Fatal("invalid protocol left connection open")
			}
			return
		}
		if f.Header.Type != protocol.TypeError {
			t.Fatalf("invalid frame produced type=%x payload=%s", f.Header.Type, f.Payload)
		}
	}
}

func TestProbeInvalidProtocolReconnect(t *testing.T) {
	bin := probeBinary(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	var output lockedBuffer
	startProbe(t, bin, l.Addr().String(), "invalid-wire", &output)
	cases := []struct {
		name    string
		kind    uint8
		flags   uint16
		payload string
		mutate  func([]byte) []byte
	}{
		{"magic", protocol.TypeTask, 0, `{}`, func(b []byte) []byte { b[0] = 'X'; return b }},
		{"version", protocol.TypeTask, 0, `{}`, func(b []byte) []byte { b[4] = 2; return b }},
		{"zero-id", protocol.TypeTask, 0, `{}`, func(b []byte) []byte { binary.BigEndian.PutUint64(b[12:], 0); return b }},
		{"repeat-id", protocol.TypeTask, 0, `{}`, func(b []byte) []byte { binary.BigEndian.PutUint64(b[12:], 1); return b }},
		{"gap-id", protocol.TypeTask, 0, `{}`, func(b []byte) []byte { binary.BigEndian.PutUint64(b[12:], 3); return b }},
		{"control-limit-header-only", protocol.TypeTask, 0, `{}`, func(b []byte) []byte { binary.BigEndian.PutUint32(b[8:], 65537); return b[:20] }},
		{"chunk-hard-limit-header-only", protocol.TypeFileChunk, protocol.FlagBinary, `{}`, func(b []byte) []byte { binary.BigEndian.PutUint32(b[8:], 28+512*1024+1); return b[:20] }},
		{"invalid-json", protocol.TypeTask, 0, `{`, nil},
		{"array-json", protocol.TypeTask, 0, `[]`, nil},
		{"null-json", protocol.TypeTask, 0, `null`, nil},
		{"invalid-utf8", protocol.TypeTask, 0, "{\"task_id\":\"\xff\"}", nil},
		{"surrogate", protocol.TypeTask, 0, `{"task_id":"\ud800"}`, nil},
		{"reply-zero", protocol.TypeHeartbeatAck, protocol.FlagResponse, `{"reply_to":0,"server_time":0}`, nil},
		{"reply-unsent", protocol.TypeHeartbeatAck, protocol.FlagResponse, `{"reply_to":1,"server_time":0}`, nil},
		{"reply-missing", protocol.TypeHeartbeatAck, protocol.FlagResponse, `{"server_time":0}`, nil},
		{"reply-string", protocol.TypeHeartbeatAck, protocol.FlagResponse, `{"reply_to":"1","server_time":0}`, nil},
		{"ack-no-response", protocol.TypeHeartbeatAck, 0, `{"reply_to":1,"server_time":0}`, nil},
		{"unsupported-cancel", 0x13, 0, `{}`, nil},
	}
	for bit := 0; bit < 16; bit++ {
		cases = append(cases, struct {
			name    string
			kind    uint8
			flags   uint16
			payload string
			mutate  func([]byte) []byte
		}{fmt.Sprintf("task-flag-%d", bit), protocol.TypeTask, 1 << bit, `{}`, nil})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := acceptFilePeer(t, l, tc.name)
			// Validate the immediate heartbeat before injecting a malformed frame.
			// Leave it pending so server message IDs remain 1,2.
			first, err := readProtocolFrame(p.conn)
			if err != nil {
				t.Fatal(err)
			}
			if first.Header.MessageID != 2 {
				t.Fatal("first heartbeat sequence", first.Header)
			}
			assertSystemHeartbeat(t, first)
			data, _ := protocol.EncodeFrame(protocol.Frame{Header: protocol.Header{Version: 1, Type: tc.kind, Flags: tc.flags, MessageID: 2}, Payload: []byte(tc.payload)})
			if tc.mutate != nil {
				data = tc.mutate(data)
			}
			_ = p.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if _, err := p.conn.Write(data); err != nil {
				t.Fatal(err)
			}
			expectProtocolClose(t, p.conn)
			p.conn.Close()
		})
	}
	// Same process remains usable after every malformed connection.
	p := acceptFilePeer(t, l, "recovered")
	p.send(protocol.TypeTask, 0, map[string]interface{}{"task_id": "after-invalid", "type": "exec", "timeout": 2, "params": map[string]string{"command": "printf recovered"}})
	p.ack(2, "queued")
	wireResult(p, "after-invalid", "success")
}

func TestProbeNegotiatedCoalescedLimit(t *testing.T) {
	bin := probeBinary(t)
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	var out lockedBuffer
	startProbe(t, bin, l.Addr().String(), "coalesced", &out)
	for _, n := range []int{65537, 65536} {
		l.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second))
		c, e := l.Accept()
		if e != nil {
			t.Fatal(e)
		}
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		f, e := readProtocolFrame(c)
		if e != nil || f.Header.MessageID != 1 {
			t.Fatal("register", e)
		}
		ack, _ := encodedJSONFrame(protocol.TypeRegisterAck, protocol.FlagResponse, 1, map[string]interface{}{"reply_to": 1, "success": true, "session_id": "coalesced", "heartbeat_interval": 10, "server_time": 0, "max_control_payload": 65536, "telemetry_v2": true, "managed_config_v1": true, "file_chunk_size": 65536})
		payload := `{"task_id":"limit","type":"exec","timeout":2,"params":{"command":"true"},"padding":"`
		payload += strings.Repeat("x", n-len(payload)-2) + `"}`
		data, _ := protocol.EncodeFrame(protocol.Frame{Header: protocol.Header{Version: 1, Type: protocol.TypeTask, MessageID: 2}, Payload: []byte(payload)})
		if _, e = c.Write(append(ack, data...)); e != nil {
			t.Fatal(e)
		}
		if n == 65537 {
			expectProtocolClose(t, c)
		} else {
			p := &phase1cPeer{t: t, conn: c, sent: 2, received: 1}
			p.ack(2, "queued")
			wireResult(p, "limit", "success")
		}
		c.Close()
	}
}

func TestUploadStreamingKeepsHeartbeatAndExec(t *testing.T) {
	var once sync.Once
	started := make(chan struct{})
	s, device, out := relayedFileServer(t, func(fromProbe bool, f *protocol.Frame) bool {
		if !fromProbe && f.Header.Type == protocol.TypeFileChunk {
			once.Do(func() { close(started) })
			time.Sleep(125 * time.Millisecond)
		}
		return true
	})
	dir := t.TempDir()
	src := filepath.Join(dir, "source")
	dst := filepath.Join(dir, "target")
	data := bytes.Repeat([]byte{0, 255}, 8*1024*1024)
	if e := os.WriteFile(src, data, 0600); e != nil {
		t.Fatal(e)
	}
	id, e := s.CreateUpload(context.Background(), device, filetransfer.UploadRequest{SourcePath: src, RemotePath: dst, Mode: "0600", Timeout: 50 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("stream not started")
	}
	deadline := time.Now().Add(22 * time.Second)
	for !strings.Contains(out.String(), "received=HEARTBEAT_ACK") {
		if time.Now().After(deadline) {
			t.Fatal("upload heartbeat starved", out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	r := runExec(t, s, device, task.ExecRequest{Command: "printf upload-control", Timeout: 3 * time.Second})
	if r.Stdout != "upload-control" {
		t.Fatal(r)
	}
	snap, _ := s.TaskSnapshot(id)
	if snap.Result != nil {
		t.Fatal("upload ended before control check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if r, e = s.WaitTaskResult(ctx, id); e != nil || r.Status != "success" {
		t.Fatal(r, e, out.String())
	}
	got, e := os.ReadFile(dst)
	if e != nil || !bytes.Equal(got, data) {
		t.Fatal("upload content", e)
	}
}

// A real interrupted receiver must release its temporary file, socket, workers
// and file descriptors on each reconnect; task identity memory is retained by design.
func TestProbeReconnectResourceConvergence(t *testing.T) {
	bin := probeBinary(t)
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	var out lockedBuffer
	cmd := startProbe(t, bin, l.Addr().String(), "resources", &out)
	dir := t.TempDir()
	count := func(name string) int {
		entries, e := os.ReadDir(fmt.Sprintf("/proc/%d/%s", cmd.Process.Pid, name))
		if e != nil {
			t.Fatal(e)
		}
		return len(entries)
	}
	p := acceptFilePeer(t, l, "resource-0")
	p.send(protocol.TypeTask, 0, map[string]interface{}{"task_id": "warmup", "type": "start_process", "timeout": 1, "params": map[string]string{}})
	if f := p.read(); f.Header.Type != protocol.TypeTaskAck {
		t.Fatal("warmup ACK missing")
	}
	baseFD, baseThreads := count("fd"), count("task")
	for i := 0; i < 8; i++ {
		w, f := wireFile(800+i, "upload", filepath.Join(dir, fmt.Sprintf("target-%d", i)), []byte("incomplete"))
		p.ack(p.send(protocol.TypeTask, 0, w), "queued")
		fileACK(p, beginUpload(p, w, f), "ready")
		binaryChunk(p, f.TransferID, 0, []byte("in"))
		p.conn.Close()
		p = acceptFilePeer(t, l, fmt.Sprintf("resource-%d", i+1))
		for j := 0; j <= i; j++ {
			wireResult(p, fmt.Sprintf("file-%d", 800+j), "failed")
		}
		left, e := filepath.Glob(filepath.Join(dir, ".rmp-transfer-*"))
		if e != nil || len(left) != 0 {
			t.Fatal("temp leak", left, e)
		}
		if count("fd") > baseFD || count("task") > baseThreads {
			t.Fatalf("resource growth: fd=%d/%d threads=%d/%d", count("fd"), baseFD, count("task"), baseThreads)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("interrupted target published")
	}
}
