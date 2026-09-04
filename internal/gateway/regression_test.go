package gateway

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
)

type gatedWriteConn struct {
	net.Conn
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *gatedWriteConn) Write(p []byte) (int, error) {
	c.once.Do(func() { close(c.entered); <-c.release })
	return c.Conn.Write(p)
}

func TestRegisterPublishesOnlyAfterAck(t *testing.T) {
	for _, replacing := range []bool{false, true} {
		t.Run(map[bool]string{false: "first registration", true: "replacement"}[replacing], func(t *testing.T) {
			s, _ := New(Config{Logger: log.New(io.Discard, "", 0)})
			var previous *session
			if replacing {
				a, b := net.Pipe()
				defer a.Close()
				defer b.Close()
				previous = &session{deviceID: "register-race", sessionID: "old", transport: &connectionWriter{conn: a}}
				s.sessions["register-race"] = previous
			}
			a, peer := net.Pipe()
			conn := &gatedWriteConn{Conn: a, entered: make(chan struct{}), release: make(chan struct{})}
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(conn.release) }) }
			done := make(chan struct{})
			go func() { defer close(done); s.handleConnection(conn) }()
			defer func() { release(); peer.Close(); a.Close(); <-done }()
			writeJSONFrame(t, peer, protocol.TypeRegister, 1, validRegister("register-race"))
			select {
			case <-conn.entered:
			case <-time.After(2 * time.Second):
				t.Fatal("registration did not start writing ACK")
			}
			s.mu.Lock()
			visible := s.sessions["register-race"]
			s.mu.Unlock()
			if visible != previous {
				t.Fatal("unregistered candidate became visible for task dispatch")
			}
			if !replacing {
				if id, err := s.CreateExec(context.Background(), "register-race", task.ExecRequest{Command: "true", Timeout: time.Second}); err == nil || id != "" {
					t.Fatalf("dispatch before registration: %q, %v", id, err)
				}
			}
			release()
			ack := readFrame(t, peer)
			if ack.Header.Type != protocol.TypeRegisterAck || ack.Header.MessageID != 1 {
				t.Fatalf("first response = %#v", ack.Header)
			}
			select {
			case <-s.Events():
			case <-time.After(2 * time.Second):
				t.Fatal("no online event")
			}
			dispatched := make(chan error, 1)
			go func() {
				_, err := s.CreateExec(context.Background(), "register-race", task.ExecRequest{Command: "true", Timeout: time.Second})
				dispatched <- err
			}()
			frame := readFrame(t, peer)
			if frame.Header.Type != protocol.TypeTask || frame.Header.MessageID != 2 {
				t.Fatalf("post-registration frame = %#v", frame.Header)
			}
			if err := <-dispatched; err != nil {
				t.Fatal(err)
			}
		})
	}
}

type failingWriteConn struct {
	net.Conn
	writes, closes int
	deadlineErr    error
	writeBytes     int
}

func (c *failingWriteConn) Write(p []byte) (int, error) {
	c.writes++
	n := c.writeBytes
	if n > len(p) {
		n = len(p)
	}
	return n, io.ErrUnexpectedEOF
}
func (c *failingWriteConn) Close() error                     { c.closes++; return nil }
func (c *failingWriteConn) SetWriteDeadline(time.Time) error { return c.deadlineErr }

func TestWriterFailurePermanentlyInvalidatesStream(t *testing.T) {
	for _, n := range []int{0, 7, 1 << 20} {
		c := &failingWriteConn{writeBytes: n}
		w := &connectionWriter{conn: c, nextOutgoingID: 1, maxControlPayload: 1024}
		id, err := w.sendJSON(protocol.TypeTask, 0, map[string]string{"task_id": "x"}, nil)
		if err == nil || id != 1 || c.closes != 1 {
			t.Fatalf("n=%d: id=%d err=%v closes=%d", n, id, err, c.closes)
		}
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if id, err := w.sendJSON(protocol.TypeHeartbeatAck, protocol.FlagResponse, map[string]int{"reply_to": 1}, nil); id != 0 || err == nil {
					t.Errorf("failed writer reused: %d, %v", id, err)
				}
			}()
		}
		wg.Wait()
		if c.writes != 1 || c.closes != 1 {
			t.Fatalf("failed stream accessed again: writes=%d closes=%d", c.writes, c.closes)
		}
	}
}

func TestCreateExecKeepsUncertainDispatch(t *testing.T) {
	s, _ := New(Config{Logger: log.New(io.Discard, "", 0)})
	c := &failingWriteConn{writeBytes: 7}
	s.sessions["device"] = &session{deviceID: "device", sessionID: "session", transport: &connectionWriter{conn: c, nextOutgoingID: 2, maxControlPayload: 1024}}
	id, err := s.CreateExec(context.Background(), "device", task.ExecRequest{Command: "side-effect", Timeout: time.Second})
	if id == "" || !errors.Is(err, ErrDispatchUncertain) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("dispatch = %q, %v", id, err)
	}
	snapshot, err := s.TaskSnapshot(id)
	if err != nil || snapshot.MessageID != 2 || snapshot.Spec.Command != "side-effect" || snapshot.Result != nil {
		t.Fatalf("lost dispatch record: %#v, %v", snapshot, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.WaitTaskResult(ctx, id); !errors.Is(err, context.Canceled) {
		t.Fatalf("uncertain dispatch incorrectly finalized: %v", err)
	}
}

func TestWriterPreWriteFailures(t *testing.T) {
	c := &failingWriteConn{}
	w := &connectionWriter{conn: c, nextOutgoingID: 1, maxControlPayload: 1024}
	called := false
	before := func(uint64) error { called = true; return nil }
	for _, payload := range []interface{}{make(chan int), strings.Repeat("x", 2048)} {
		if id, err := w.sendJSON(protocol.TypeTask, 0, payload, before); id != 0 || err == nil {
			t.Fatalf("invalid payload: %d, %v", id, err)
		}
	}
	if called || c.writes != 0 || c.closes != 0 {
		t.Fatal("local validation touched the transport or dispatch record")
	}
	c.deadlineErr = errors.New("deadline failed")
	if id, err := w.sendJSON(protocol.TypeTask, 0, struct{}{}, before); id != 0 || err == nil || called || c.closes != 1 {
		t.Fatalf("deadline failure: %d, %v", id, err)
	}
	c = &failingWriteConn{}
	w = &connectionWriter{conn: c, nextOutgoingID: ^uint64(0), maxControlPayload: 1024}
	if id, err := w.sendJSON(protocol.TypeTask, 0, struct{}{}, before); id != 0 || err == nil || called || c.closes != 1 || c.writes != 0 {
		t.Fatalf("exhausted message_id: %d, %v", id, err)
	}
}
