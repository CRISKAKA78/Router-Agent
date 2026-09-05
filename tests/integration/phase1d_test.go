package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"routerprobe/internal/filetransfer"
	"routerprobe/internal/gateway"
	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
	"strings"
	"sync"
	"testing"
	"time"
)

func fileServer(t *testing.T) (*gateway.Server, string, *lockedBuffer) {
	t.Helper()
	bin := probeBinary(t)
	s, e := gateway.New(gateway.Config{HeartbeatInterval: 10 * time.Second, MaxControlPayload: 1024, FileChunkSize: 65536, Logger: log.New(io.Discard, "", 0)})
	if e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	go s.Serve(l)
	t.Cleanup(func() { s.Close() })
	out := &lockedBuffer{}
	device := "phase1d"
	startProbe(t, bin, l.Addr().String(), device, out)
	waitOnline(t, s.Events(), device, 5*time.Second)
	return s, device, out
}

// hook returns false to drop the selected frame and tear down that session.
// Every reconnect uses the same Probe process and passes through this relay.
func fileRelay(t *testing.T, upstream string, hook func(bool, *protocol.Frame) bool) string {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	var mu sync.Mutex
	var conns []net.Conn
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			p, e := l.Accept()
			if e != nil {
				return
			}
			s, e := net.Dial("tcp", upstream)
			if e != nil {
				p.Close()
				return
			}
			mu.Lock()
			conns = append(conns, p, s)
			mu.Unlock()
			var wg sync.WaitGroup
			wg.Add(2)
			for _, direction := range []bool{true, false} {
				go func(fromProbe bool) {
					defer wg.Done()
					defer p.Close()
					defer s.Close()
					src, dst := p, s
					if !fromProbe {
						src, dst = s, p
					}
					for {
						f, e := readProtocolFrame(src)
						if e != nil {
							return
						}
						if !hook(fromProbe, &f) {
							return
						}
						if e = protocol.WriteFrame(dst, f); e != nil {
							return
						}
					}
				}(direction)
			}
			wg.Wait()
		}
	}()
	t.Cleanup(func() {
		l.Close()
		mu.Lock()
		for _, c := range conns {
			c.Close()
		}
		mu.Unlock()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("file relay failed to stop")
		}
	})
	return l.Addr().String()
}
func relayedFileServer(t *testing.T, hook func(bool, *protocol.Frame) bool) (*gateway.Server, string, *lockedBuffer) {
	t.Helper()
	bin := probeBinary(t)
	s, e := gateway.New(gateway.Config{HeartbeatInterval: 10 * time.Second, Logger: log.New(io.Discard, "", 0)})
	if e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	go s.Serve(l)
	t.Cleanup(func() { s.Close() })
	addr := fileRelay(t, l.Addr().String(), hook)
	out := &lockedBuffer{}
	startProbe(t, bin, addr, "file-relay", out)
	waitOnline(t, s.Events(), "file-relay", 5*time.Second)
	return s, "file-relay", out
}
func TestRealFileFaultsAndCommitAckLoss(t *testing.T) {
	for _, which := range []string{"upload-ack-lost", "upload-interrupt", "download-corrupt", "download-interrupt", "download-done-lost"} {
		t.Run(which, func(t *testing.T) {
			var once sync.Once
			hit := make(chan struct{})
			s, device, out := relayedFileServer(t, func(fromProbe bool, f *protocol.Frame) bool {
				match := false
				switch which {
				case "upload-interrupt":
					match = !fromProbe && f.Header.Type == protocol.TypeFileChunk
				case "download-corrupt", "download-interrupt":
					match = fromProbe && f.Header.Type == protocol.TypeFileChunk
				case "upload-ack-lost", "download-done-lost":
					if f.Header.Type == protocol.TypeFileAck {
						var a filetransfer.Ack
						json.Unmarshal(f.Payload, &a)
						match = a.Status == "done" && fromProbe == (which == "upload-ack-lost")
					}
				}
				pass := true
				if match {
					once.Do(func() {
						close(hit)
						if which == "download-corrupt" {
							f.Payload[28] ^= 255
						} else {
							pass = false
						}
					})
				}
				return pass
			})
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			target := filepath.Join(dir, "target")
			data := bytes.Repeat([]byte{0, 1, 128, 255}, 50000)
			os.WriteFile(source, data, 0600)
			var id string
			var e error
			if strings.HasPrefix(which, "upload") {
				id, e = s.CreateUpload(context.Background(), device, filetransfer.UploadRequest{SourcePath: source, RemotePath: target, Mode: "0600", Timeout: 10 * time.Second})
			} else {
				id, e = s.CreateDownload(context.Background(), device, filetransfer.DownloadRequest{RemotePath: source, ResultName: "file.bin", TargetPath: target, Timeout: 10 * time.Second})
			}
			if e != nil {
				t.Fatal(e)
			}
			select {
			case <-hit:
			case <-time.After(5 * time.Second):
				t.Fatal("fault not reached")
			}
			r := fileResult(t, s, id)
			want := "failed"
			if which == "upload-ack-lost" {
				want = "success"
			}
			if r.Status != want {
				t.Fatalf("want %s got %+v\n%s", want, r, out.String())
			}
			committed := which == "upload-ack-lost" || which == "download-done-lost"
			got, e := os.ReadFile(target)
			if committed {
				if e != nil || !bytes.Equal(got, data) {
					t.Fatal("committed file lost", e)
				}
			} else if !os.IsNotExist(e) {
				t.Fatal("failed file exposed", e)
			}
			if which == "download-done-lost" {
				snap, _ := s.FileSnapshot(id)
				if !snap.Committed {
					t.Fatal("local commit fact lost")
				}
			}
			if e = s.ResendTask(context.Background(), id); e != nil {
				t.Fatal(e)
			}
			runExec(t, s, device, task.ExecRequest{Command: "true", Timeout: time.Second})
			left, _ := filepath.Glob(filepath.Join(dir, ".rmp-transfer-*"))
			if len(left) > 0 {
				t.Fatal("temporary file leaked", left)
			}
		})
	}
}
func TestFileStreamingKeepsHeartbeatAndExec(t *testing.T) {
	// Pace real file chunks for longer than one heartbeat interval. Control frames
	// continue in both directions, and exec completes before the transfer finishes.
	var once sync.Once
	started := make(chan struct{})
	s, device, out := relayedFileServer(t, func(fromProbe bool, f *protocol.Frame) bool {
		if fromProbe && f.Header.Type == protocol.TypeFileChunk {
			once.Do(func() { close(started) })
			time.Sleep(125 * time.Millisecond)
		}
		return true
	})
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	data := bytes.Repeat([]byte{0, 255}, 8*1024*1024)
	os.WriteFile(source, data, 0600)
	id, e := s.CreateDownload(context.Background(), device, filetransfer.DownloadRequest{RemotePath: source, ResultName: "paced.bin", TargetPath: target, Timeout: 50 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	<-started
	// Wait until the stream has lasted through a heartbeat, then dispatch exec.
	deadline := time.Now().Add(22 * time.Second)
	for !strings.Contains(out.String(), "received=HEARTBEAT_ACK") {
		if time.Now().After(deadline) {
			t.Fatalf("heartbeat starved\n%s", out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	r := runExec(t, s, device, task.ExecRequest{Command: "printf during-file", Timeout: 3 * time.Second})
	if r.Stdout != "during-file" {
		t.Fatal(r)
	}
	snap, _ := s.TaskSnapshot(id)
	if snap.Result != nil {
		t.Fatal("transfer finished before concurrent control check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	final, err := s.WaitTaskResult(ctx, id)
	if err != nil || final.Status != "success" {
		t.Fatalf("paced transfer failed: %v %+v", err, final)
	}
	got, _ := os.ReadFile(target)
	if !bytes.Equal(got, data) {
		t.Fatal("paced content")
	}
}

func filePeer(t *testing.T) (net.Listener, *phase1cPeer, *lockedBuffer) {
	t.Helper()
	bin := probeBinary(t)
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { l.Close() })
	out := &lockedBuffer{}
	startProbe(t, bin, l.Addr().String(), "file-wire", out)
	return l, acceptFilePeer(t, l, "first"), out
}
func acceptFilePeer(t *testing.T, l net.Listener, session string) *phase1cPeer {
	t.Helper()
	l.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second))
	c, e := l.Accept()
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	p := &phase1cPeer{t: t, conn: c}
	f := p.read()
	if f.Header.Type != protocol.TypeRegister {
		t.Fatal("register")
	}
	p.send(protocol.TypeRegisterAck, protocol.FlagResponse, map[string]interface{}{"reply_to": 1, "success": true, "session_id": session, "heartbeat_interval": 10, "server_time": time.Now().Unix(), "max_control_payload": 1024, "file_chunk_size": 65536})
	return p
}
func wireFile(n int, kind, remote string, data []byte) (map[string]interface{}, filetransfer.Params) {
	h := sha256.Sum256(data)
	p := filetransfer.Params{TransferID: fmt.Sprintf("00112233-4455-4677-8899-%012d", n), RemotePath: remote, Size: int64(len(data)), SHA256: hex.EncodeToString(h[:]), Mode: "0600", ResultName: "result.bin"}
	return map[string]interface{}{"task_id": fmt.Sprintf("file-%d", n), "type": kind, "timeout": 20, "params": p.Wire(kind)}, p
}
func beginUpload(p *phase1cPeer, w map[string]interface{}, f filetransfer.Params) uint64 {
	return p.send(protocol.TypeFileBegin, 0, filetransfer.Begin{TaskID: w["task_id"].(string), TransferID: f.TransferID, Direction: "server_to_device", Name: "file.bin", RemotePath: f.RemotePath, Size: f.Size, SHA256: f.SHA256, ChunkSize: 65536, Mode: &f.Mode, Overwrite: &f.Overwrite})
}
func binaryChunk(p *phase1cPeer, id string, off uint64, data []byte) {
	p.t.Helper()
	b, e := filetransfer.Chunk(id, off, data)
	if e != nil {
		p.t.Fatal(e)
	}
	p.sent++
	p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if e = protocol.WriteFrame(p.conn, protocol.Frame{Header: protocol.Header{Version: 1, Type: protocol.TypeFileChunk, Flags: protocol.FlagBinary, MessageID: p.sent}, Payload: b}); e != nil {
		p.t.Fatal(e)
	}
}
func fileACK(p *phase1cPeer, reply uint64, status string) {
	p.t.Helper()
	f := p.read()
	a, e := filetransfer.ParseAck(f.Payload)
	if e != nil || f.Header.Type != protocol.TypeFileAck || f.Header.Flags != protocol.FlagResponse || a.ReplyTo != reply || a.Status != status {
		p.t.Fatalf("ACK want %s: %s err %v", status, f.Payload, e)
	}
}
func wireResult(p *phase1cPeer, id, status string) protocol.Frame {
	p.t.Helper()
	f := p.read()
	var r task.Result
	e := json.Unmarshal(f.Payload, &r)
	if e != nil || f.Header.Type != protocol.TypeTaskResult || r.TaskID != id || r.Status != status {
		p.t.Fatalf("RESULT want %s/%s: %s", id, status, f.Payload)
	}
	return f
}
func uploadEnd(p *phase1cPeer, f filetransfer.Params) uint64 {
	return p.send(protocol.TypeFileEnd, 0, filetransfer.End{TransferID: f.TransferID, Size: f.Size, SHA256: f.SHA256})
}
func TestFileMixedFIFOAndControls(t *testing.T) {
	_, p, _ := filePeer(t)
	dir := t.TempDir()
	data := []byte{0, 255, 3}
	source := filepath.Join(dir, "source")
	os.WriteFile(source, data, 0600)
	first, a := wireFile(1, "upload", filepath.Join(dir, "first"), data)
	second, b := wireFile(2, "download", source, nil)
	third, c := wireFile(3, "upload", filepath.Join(dir, "third"), data)
	p.ack(p.send(protocol.TypeTask, 0, first), "queued")
	fileACK(p, beginUpload(p, first, a), "ready")
	p.ack(p.send(protocol.TypeTask, 0, second), "queued")
	p.ack(p.send(protocol.TypeTask, 0, third), "queued")
	thirdBegin := beginUpload(p, third, c)
	p.ack(p.send(protocol.TypeTask, 0, first), "running")
	p.ack(p.send(protocol.TypeTask, 0, second), "queued")
	conflict := map[string]interface{}{}
	for k, v := range third {
		conflict[k] = v
	}
	changed := c
	changed.Mode = "0755"
	conflict["params"] = changed.Wire("upload")
	p.send(protocol.TypeTask, 0, conflict)
	f := p.read()
	if f.Header.Type != protocol.TypeError {
		t.Fatal("file conflict was not ERROR")
	}
	execID := p.send(protocol.TypeTask, 0, map[string]interface{}{"task_id": "control-exec", "type": "exec", "timeout": 2, "params": map[string]interface{}{"command": "printf alive"}})
	p.ack(execID, "queued")
	wireResult(p, "control-exec", "success")
	for _, path := range []string{a.RemotePath, c.RemotePath} {
		if _, e := os.Stat(path); !os.IsNotExist(e) {
			t.Fatal("incomplete/queued file visible")
		}
	}
	binaryChunk(p, a.TransferID, 0, data)
	fileACK(p, uploadEnd(p, a), "done")
	wireResult(p, "file-1", "success")
	f = p.read()
	begin, e := filetransfer.ParseBegin(f.Payload, 65536)
	if e != nil || f.Header.Type != protocol.TypeFileBegin || begin.TransferID != b.TransferID {
		t.Fatalf("FIFO download begin %s %v", f.Payload, e)
	}
	p.send(protocol.TypeFileAck, protocol.FlagResponse, filetransfer.NewAck(f.Header.MessageID, b.TransferID, "ready", 0))
	var got []byte
	for {
		f = p.read()
		if f.Header.Type == protocol.TypeFileEnd {
			break
		}
		if f.Header.Type != protocol.TypeFileChunk {
			t.Fatalf("download type %x", f.Header.Type)
		}
		d, e := filetransfer.ParseChunk(f.Payload, b.TransferID, int64(len(got)), begin.Size, begin.ChunkSize)
		if e != nil {
			t.Fatal(e)
		}
		got = append(got, d...)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("download bytes")
	}
	p.send(protocol.TypeFileAck, protocol.FlagResponse, filetransfer.NewAck(f.Header.MessageID, b.TransferID, "done", int64(len(got))))
	wireResult(p, "file-2", "success")
	fileACK(p, thirdBegin, "ready")
	binaryChunk(p, c.TransferID, 0, data)
	fileACK(p, uploadEnd(p, c), "done")
	wireResult(p, "file-3", "success")
}
func TestFileQueueBoundAndDisconnect(t *testing.T) {
	l, p, _ := filePeer(t)
	dir := t.TempDir()
	var tasks []map[string]interface{}
	var params []filetransfer.Params
	for i := 0; i < 10; i++ {
		w, f := wireFile(i+10, "upload", filepath.Join(dir, fmt.Sprint(i)), []byte("abc"))
		tasks = append(tasks, w)
		params = append(params, f)
		id := p.send(protocol.TypeTask, 0, w)
		if i < 9 {
			p.ack(id, "queued")
			if i == 0 {
				fileACK(p, beginUpload(p, w, f), "ready")
			}
		} else {
			frame := p.read()
			var a task.Ack
			json.Unmarshal(frame.Payload, &a)
			if frame.Header.Type != protocol.TypeTaskAck || a.Accepted || a.State != "rejected" {
				t.Fatalf("queue full %s", frame.Payload)
			}
		}
	}
	binaryChunk(p, params[0].TransferID, 0, []byte("a"))
	p.conn.Close()
	p = acceptFilePeer(t, l, "reconnected")
	seen := map[string]protocol.Frame{}
	for i := 0; i < 9; i++ {
		f := p.read()
		var r task.Result
		json.Unmarshal(f.Payload, &r)
		if f.Header.Type != protocol.TypeTaskResult || r.Status != "failed" {
			t.Fatalf("disconnect result %s", f.Payload)
		}
		seen[r.TaskID] = f
	}
	if len(seen) != 9 {
		t.Fatal("queued results missing")
	}
	for _, w := range tasks[:9] {
		id := w["task_id"].(string)
		p.ack(p.send(protocol.TypeTask, 0, w), "failed")
		f := wireResult(p, id, "failed")
		if string(f.Payload) != string(seen[id].Payload) {
			t.Fatal("failure cache changed")
		}
	}
	for _, f := range params {
		if _, e := os.Stat(f.RemotePath); !os.IsNotExist(e) {
			t.Fatal("interrupted file published")
		}
	}
	left, _ := filepath.Glob(filepath.Join(dir, ".rmp-transfer-*"))
	if len(left) > 0 {
		t.Fatal("interruption left temporary files", left)
	}
}
func TestProbeFileVerificationFailures(t *testing.T) {
	for _, which := range []string{"sha256", "size", "offset", "flags", "timeout"} {
		t.Run(which, func(t *testing.T) {
			l, p, _ := filePeer(t)
			target := filepath.Join(t.TempDir(), "target")
			original := []byte("original")
			os.WriteFile(target, original, 0600)
			w, f := wireFile(30, "upload", target, []byte("abc"))
			f.Overwrite = true
			w["params"] = f.Wire("upload")
			if which == "timeout" {
				w["timeout"] = 1
			}
			p.ack(p.send(protocol.TypeTask, 0, w), "queued")
			fileACK(p, beginUpload(p, w, f), "ready")
			if which == "sha256" {
				binaryChunk(p, f.TransferID, 0, []byte("bad"))
			} else if which == "size" {
				binaryChunk(p, f.TransferID, 0, []byte("a"))
			} else if which == "offset" {
				binaryChunk(p, f.TransferID, 1, []byte("a"))
			} else if which == "flags" {
				chunk, _ := filetransfer.Chunk(f.TransferID, 0, []byte("a"))
				p.sent++
				if e := protocol.WriteFrame(p.conn, protocol.Frame{Header: protocol.Header{Version: 1, Type: protocol.TypeFileChunk, MessageID: p.sent}, Payload: chunk}); e != nil {
					t.Fatal(e)
				}
			}
			if which == "sha256" || which == "size" {
				fileACK(p, uploadEnd(p, f), "failed")
				wireResult(p, "file-30", "failed")
			} else {
				p.conn.SetReadDeadline(time.Now().Add(4 * time.Second))
				for {
					_, e := readProtocolFrame(p.conn)
					if e != nil {
						break
					}
				}
				p.conn.Close()
				p = acceptFilePeer(t, l, "failure-replay")
				status := "failed"
				if which == "timeout" {
					status = "timeout"
				}
				wireResult(p, "file-30", status)
			}
			got, _ := os.ReadFile(target)
			if !bytes.Equal(got, original) {
				t.Fatal("failed transfer replaced final file")
			}
		})
	}
}

func TestRealMixedFileFIFO(t *testing.T) {
	var mu sync.Mutex
	var accepted, started []string
	mapping := map[string]string{}
	active := ""
	overlap := false
	s, device, _ := relayedFileServer(t, func(fromProbe bool, f *protocol.Frame) bool {
		mu.Lock()
		defer mu.Unlock()
		if !fromProbe && f.Header.Type == protocol.TypeTask {
			var w struct {
				TaskID string              `json:"task_id"`
				Params filetransfer.Params `json:"params"`
			}
			json.Unmarshal(f.Payload, &w)
			mapping[w.Params.TransferID] = w.TaskID
		}
		if fromProbe {
			switch f.Header.Type {
			case protocol.TypeTaskAck:
				var a task.Ack
				json.Unmarshal(f.Payload, &a)
				if a.Accepted {
					accepted = append(accepted, a.TaskID)
				}
			case protocol.TypeFileBegin:
				var b filetransfer.Begin
				json.Unmarshal(f.Payload, &b)
				if active != "" {
					overlap = true
				}
				active = b.TaskID
				started = append(started, b.TaskID)
			case protocol.TypeFileAck:
				var a filetransfer.Ack
				json.Unmarshal(f.Payload, &a)
				if a.Status == "ready" {
					if active != "" {
						overlap = true
					}
					active = mapping[a.TransferID]
					started = append(started, active)
				}
			case protocol.TypeTaskResult:
				active = ""
			}
		}
		return true
	})
	dir := t.TempDir()
	src := filepath.Join(dir, "source")
	data := bytes.Repeat([]byte{0, 128, 255}, 1024*1024)
	os.WriteFile(src, data, 0600)
	ids := make([]string, 3)
	for i := range ids {
		target := filepath.Join(dir, fmt.Sprint(i))
		var e error
		if i == 1 {
			ids[i], e = s.CreateDownload(context.Background(), device, filetransfer.DownloadRequest{RemotePath: src, ResultName: "result.bin", TargetPath: target, Timeout: 15 * time.Second})
		} else {
			ids[i], e = s.CreateUpload(context.Background(), device, filetransfer.UploadRequest{SourcePath: src, RemotePath: target, Mode: "0600", Timeout: 15 * time.Second})
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	for i, id := range ids {
		if fileResult(t, s, id).Status != "success" {
			t.Fatal("mixed transfer failed")
		}
		got, _ := os.ReadFile(filepath.Join(dir, fmt.Sprint(i)))
		if !bytes.Equal(got, data) {
			t.Fatal("mixed content")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if overlap || len(started) != 3 || strings.Join(started, ",") != strings.Join(accepted, ",") {
		t.Fatalf("accepted=%v started=%v overlap=%t", accepted, started, overlap)
	}
}
func fileResult(t *testing.T, s *gateway.Server, id string) task.Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r, e := s.WaitTaskResult(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestFileRoundTrip(t *testing.T) {
	s, device, out := fileServer(t)
	dir := t.TempDir()
	for _, n := range []int{0, 1, 65536, 65537, 3*1024*1024 + 17} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			data := make([]byte, n)
			for i := range data {
				data[i] = byte(i*31 + 17)
			}
			src := filepath.Join(dir, fmt.Sprint(n)+"-source")
			remote := src + "-remote"
			dst := src + "-download"
			if e := os.WriteFile(src, data, 0600); e != nil {
				t.Fatal(e)
			}
			id, e := s.CreateUpload(context.Background(), device, filetransfer.UploadRequest{SourcePath: src, RemotePath: remote, Mode: "0751", Timeout: 10 * time.Second})
			if e != nil {
				t.Fatal(e)
			}
			r := fileResult(t, s, id)
			if r.Status != "success" {
				t.Fatalf("upload: %+v\n%s", r, out.String())
			}
			got, e := os.ReadFile(remote)
			if e != nil || !bytes.Equal(got, data) {
				t.Fatalf("upload content %v", e)
			}
			st, _ := os.Stat(remote)
			if st.Mode().Perm() != 0751 {
				t.Fatalf("mode %v", st.Mode())
			}
			did, e := s.CreateDownload(context.Background(), device, filetransfer.DownloadRequest{RemotePath: remote, ResultName: "download.bin", TargetPath: dst, Timeout: 10 * time.Second})
			if e != nil {
				t.Fatal(e)
			}
			r = fileResult(t, s, did)
			if r.Status != "success" {
				t.Fatalf("download: %+v\n%s", r, out.String())
			}
			got, e = os.ReadFile(dst)
			if e != nil || !bytes.Equal(got, data) {
				t.Fatalf("download content %v", e)
			}
			snap, _ := s.FileSnapshot(did)
			if !snap.Committed {
				t.Fatal("download commit not exposed")
			}
			// Replace final files after success. Same task IDs must never touch them again.
			sentinel := []byte("preserve later changes")
			os.WriteFile(remote, sentinel, 0600)
			os.WriteFile(dst, sentinel, 0600)
			for _, tid := range []string{id, did} {
				if e = s.ResendTask(context.Background(), tid); e != nil {
					t.Fatal(e)
				}
			}
			barrier := runExec(t, s, device, task.ExecRequest{Command: "true", Timeout: time.Second})
			if barrier.Status != "success" {
				t.Fatal(barrier)
			}
			for _, p := range []string{remote, dst} {
				got, _ = os.ReadFile(p)
				if !bytes.Equal(got, sentinel) {
					t.Fatal("duplicate changed final file")
				}
			}
		})
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".rmp-transfer-*"))
	if len(leftovers) > 0 {
		t.Fatalf("temporary files: %v", leftovers)
	}
}
