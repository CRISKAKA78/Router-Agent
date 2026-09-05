package gateway

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"routerprobe/internal/filetransfer"
	"routerprobe/internal/task"
	"strings"
	"testing"
	"time"
)

func TestRepositoryUploadSessionAndContentPreconditions(t *testing.T) {
	s, _ := New(Config{})
	defer s.files.Close()
	path := filepath.Join(t.TempDir(), "source")
	os.WriteFile(path, []byte("data"), 0600)
	c := &failingWriteConn{writeBytes: 7}
	active := &session{deviceID: "device", sessionID: "old", done: make(chan struct{}), transport: &connectionWriter{conn: c, nextOutgoingID: 1, maxControlPayload: 1024}}
	s.sessions["device"] = active
	q := filetransfer.UploadRequest{SourcePath: path, RemotePath: "/tmp/tool", Mode: "0755", Timeout: time.Second, ExpectedSessionID: "new"}
	if id, e := s.CreateUpload(context.Background(), "device", q); id != "" || !errors.Is(e, ErrSessionChanged) || c.writes != 0 {
		t.Fatal(id, e)
	}
	q.ExpectedSessionID = "old"
	q.Expected = &filetransfer.ContentMetadata{Size: 4, SHA256: strings.Repeat("0", 64)}
	if id, e := s.CreateUpload(context.Background(), "device", q); id != "" || e == nil || c.writes != 0 {
		t.Fatal(id, e)
	}
	q.Expected = nil
	var preparedID string
	id, e := s.createFile(context.Background(), "device", "old", func(a *session) (task.Spec, error) {
		spec, err := s.files.Upload(context.Background(), "device", q, s.fileTransport(a))
		preparedID = spec.ID
		s.mu.Lock()
		s.sessions["device"] = &session{deviceID: "device", sessionID: "new"}
		s.mu.Unlock()
		return spec, err
	})
	if id != "" || !errors.Is(e, ErrSessionChanged) || c.writes != 0 {
		t.Fatal(id, e, c.writes)
	}
	if _, e = s.TaskSnapshot(preparedID); !errors.Is(e, task.ErrTaskNotFound) {
		t.Fatal("undispatched task retained", e)
	}
	if _, e = s.FileSnapshot(preparedID); !errors.Is(e, task.ErrTaskNotFound) {
		t.Fatal("undispatched file retained", e)
	}
	s.mu.Lock()
	s.sessions["device"] = active
	s.mu.Unlock()
	id, e = s.CreateUpload(context.Background(), "device", q)
	if id == "" || !errors.Is(e, ErrDispatchUncertain) {
		t.Fatal(id, e)
	}
	snap, e := s.TaskSnapshot(id)
	if e != nil || len(snap.Dispatches) != 1 || snap.Dispatches[0].SessionID != "old" {
		t.Fatal(snap, e)
	}
	for _, forbidden := range []string{"asset_id", "tool_id", "artifact_id", "Expected", "expected", "session"} {
		if strings.Contains(string(snap.Spec.Params), forbidden) {
			t.Fatal("local fields on wire", string(snap.Spec.Params))
		}
	}
}

// Replacement while the writer is queued must be checked at dispatch admission,
// not merely when selecting a Session before file hashing.
func TestRepositoryDispatchChecksSessionAfterWriterWait(t *testing.T) {
	s, _ := New(Config{})
	c := &failingWriteConn{}
	a := &session{deviceID: "device", sessionID: "old", transport: &connectionWriter{conn: c, nextOutgoingID: 1, maxControlPayload: 1024}}
	s.sessions["device"] = a
	spec, e := s.tasks.NewExec("device", task.ExecRequest{Command: "true", Timeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	a.transport.priority.lock(false)
	result := make(chan error, 1)
	go func() { _, err := s.dispatchChecked(a, spec, true); result <- err }()
	deadline := time.Now().Add(time.Second)
	for {
		a.transport.priority.mu.Lock()
		n := a.transport.priority.controls
		a.transport.priority.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			a.transport.priority.unlock()
			t.Fatal("writer not queued")
		}
		time.Sleep(time.Millisecond)
	}
	s.mu.Lock()
	s.sessions["device"] = &session{sessionID: "replacement"}
	s.mu.Unlock()
	a.transport.priority.unlock()
	if e = <-result; !errors.Is(e, ErrSessionChanged) || c.writes != 0 {
		t.Fatal(e, c.writes)
	}
	snap, _ := s.TaskSnapshot(spec.ID)
	if len(snap.Dispatches) != 0 {
		t.Fatal("stale session marked dispatched")
	}
}
