package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"routerprobe/internal/device"
	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
)

func deviceEvent(t *testing.T, s *Server, typ EventType) SessionEvent {
	t.Helper()
	select {
	case e := <-s.Events():
		if e.Type != typ {
			t.Fatalf("event: %#v want %s", e, typ)
		}
		return e
	case <-time.After(3 * time.Second):
		t.Fatal("missing session event")
	}
	return SessionEvent{}
}

func deviceGet(t *testing.T, s *Server, id string) device.Snapshot {
	t.Helper()
	v, err := s.Devices().Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func deviceConnect(t *testing.T, s *Server, address, payload string) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	writeJSONFrame(t, c, protocol.TypeRegister, 1, payload)
	ack := readFrame(t, c)
	if ack.Header.Type != protocol.TypeRegisterAck {
		t.Fatal("missing register ack")
	}
	deviceEvent(t, s, EventOnline)
	return c
}

func TestDeviceGatewayLifecycle(t *testing.T) {
	s, address := startTestServer(t)
	full := `{"device_id":"inventory","serial":"S","model":"M","firmware":"F","probe_version":"v1","hostname":"主机","arch":"arm","kernel":"K","libc":"uclibc","boot_id":"boot","capabilities":["exec","file","future"],"extra":true}`
	firstConn := deviceConnect(t, s, address, full)
	first := deviceGet(t, s, "inventory")
	parsed, _ := parseRegister([]byte(full))
	if !reflect.DeepEqual(first.Registration, device.Registration(parsed)) || first.Status != device.Online || !first.LastOfflineAt.IsZero() {
		t.Fatalf("registration lost: %#v", first)
	}
	s.mu.Lock()
	old := s.sessions["inventory"]
	s.mu.Unlock()
	// Windows wall clocks may return the same timestamp across adjacent I/O.
	time.Sleep(20 * time.Millisecond)
	writeJSONFrame(t, firstConn, protocol.TypeHeartbeat, 2, `{"uptime":1,"running_tasks":0}`)
	readFrame(t, firstConn)
	active := deviceGet(t, s, "inventory")
	if !active.LastSeenAt.After(first.LastSeenAt) {
		t.Fatal("heartbeat did not update last seen")
	}
	time.Sleep(20 * time.Millisecond)
	secondConn := deviceConnect(t, s, address, validRegister("inventory"))
	second := deviceGet(t, s, "inventory")
	if second.CurrentSession.ID == first.CurrentSession.ID || second.Registration.Hostname != "" || second.Registration.Serial != "" || !second.FirstSeenAt.Equal(first.FirstSeenAt) || !second.LastOnlineAt.After(first.LastOnlineAt) || !second.LastOfflineAt.IsZero() {
		t.Fatalf("replacement: %#v", second)
	}
	// Exercise late cleanup after replacement and prove it cannot end the new ID.
	s.endSession(old, device.Disconnected)
	s.devices.Seen("inventory", old.sessionID, time.Now().Add(time.Hour))
	if got := deviceGet(t, s, "inventory"); !reflect.DeepEqual(got, second) {
		t.Fatal("old callback changed current device")
	}
	h, _ := s.Devices().Sessions("inventory")
	if len(h.Ended) != 1 || h.Ended[0].EndReason != device.Replaced || !h.Ended[0].EndedAt.Equal(second.LastOnlineAt) || h.Ended[0].Registration.Hostname != "主机" {
		t.Fatalf("replacement history: %#v", h)
	}
	firstConn.Close()
	if !s.Disconnect("inventory") {
		t.Fatal("disconnect failed")
	}
	deviceEvent(t, s, EventDisconnected)
	offline := deviceGet(t, s, "inventory")
	if offline.Status != device.Offline || offline.CurrentSession != nil || offline.LatestSession.EndReason != device.RequestedDisconnect || offline.LastOfflineAt.IsZero() || !offline.LastOnlineAt.Equal(second.LastOnlineAt) {
		t.Fatalf("offline: %#v", offline)
	}
	secondConn.Close()
	thirdConn := deviceConnect(t, s, address, full)
	third := deviceGet(t, s, "inventory")
	if !third.LastOfflineAt.Equal(offline.LastOfflineAt) || third.TotalSessions != 3 || len(s.Devices().List()) != 1 {
		t.Fatal("reconnect lost inventory")
	}
	time.Sleep(20 * time.Millisecond)
	thirdConn.Close()
	deviceEvent(t, s, EventDisconnected)
	if got := deviceGet(t, s, "inventory"); got.LatestSession.EndReason != device.Disconnected || !got.LastOfflineAt.After(offline.LastOfflineAt) {
		t.Fatalf("peer EOF: %#v", got)
	}
}

func TestDeviceInvalidRegisterAndInvalidActivity(t *testing.T) {
	s, address := startTestServer(t)
	c, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	writeJSONFrame(t, c, protocol.TypeRegister, 1, `{"device_id":"invalid"}`)
	readFrame(t, c)
	if len(s.Devices().List()) != 0 {
		t.Fatal("failed register created device")
	}
	c2 := deviceConnect(t, s, address, validRegister("bad-heartbeat"))
	before := deviceGet(t, s, "bad-heartbeat")
	writeJSONFrame(t, c2, protocol.TypeHeartbeat, 2, `{"uptime":"bad","running_tasks":0}`)
	readFrame(t, c2)
	deviceEvent(t, s, EventDisconnected)
	after := deviceGet(t, s, "bad-heartbeat")
	if !after.LastSeenAt.Equal(before.LastSeenAt) || after.LatestSession.EndReason != device.ProtocolError {
		t.Fatalf("invalid activity: %#v", after)
	}
}

func TestDeviceRegisterAckPublicationAndFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{true: "failed", false: "complete"}[fail], func(t *testing.T) {
			s, _ := New(Config{Logger: log.New(io.Discard, "", 0)})
			a, peer := net.Pipe()
			gate := &gatedWriteConn{Conn: a, entered: make(chan struct{}), release: make(chan struct{})}
			done := make(chan struct{})
			go func() { defer close(done); s.handleConnection(gate) }()
			var once sync.Once
			release := func() { once.Do(func() { close(gate.release) }) }
			t.Cleanup(func() { release(); peer.Close(); a.Close(); <-done; s.Close() })
			writeJSONFrame(t, peer, protocol.TypeRegister, 1, validRegister("ack"))
			<-gate.entered
			if len(s.Devices().List()) != 0 {
				t.Fatal("inventory published before ACK")
			}
			if fail {
				peer.Close()
			}
			release()
			if fail {
				<-done
				if len(s.Devices().List()) != 0 {
					t.Fatal("failed ACK created inventory")
				}
			} else {
				readFrame(t, peer)
				deviceEvent(t, s, EventOnline)
				if deviceGet(t, s, "ack").Status != device.Online {
					t.Fatal("ACK not published")
				}
			}
		})
	}
}

type deviceObservedConn struct {
	net.Conn
	deadlines  chan time.Time
	failWrites atomic.Bool
}

func (c *deviceObservedConn) SetReadDeadline(at time.Time) error {
	err := c.Conn.SetReadDeadline(at)
	c.deadlines <- at
	return err
}
func (c *deviceObservedConn) Write(b []byte) (int, error) {
	if c.failWrites.Load() {
		return 0, io.ErrUnexpectedEOF
	}
	return c.Conn.Write(b)
}

func TestDeviceTimeoutAndWriterFailure(t *testing.T) {
	for _, failWrite := range []bool{false, true} {
		t.Run(map[bool]string{false: "read-timeout", true: "write-error"}[failWrite], func(t *testing.T) {
			s, _ := New(Config{HeartbeatInterval: 20 * time.Second, Logger: log.New(io.Discard, "", 0)})
			a, peer := net.Pipe()
			conn := &deviceObservedConn{Conn: a, deadlines: make(chan time.Time, 16)}
			done := make(chan struct{})
			go func() { defer close(done); s.handleConnection(conn) }()
			t.Cleanup(func() { peer.Close(); a.Close(); <-done; s.Close() })
			writeJSONFrame(t, peer, protocol.TypeRegister, 1, validRegister("io"))
			readFrame(t, peer)
			deviceEvent(t, s, EventOnline)
			if failWrite {
				conn.failWrites.Store(true)
				id, err := s.CreateExec(context.Background(), "io", task.ExecRequest{Command: "true", Timeout: time.Second})
				if id == "" || !errors.Is(err, ErrDispatchUncertain) {
					t.Fatalf("uncertain dispatch changed: %q %v", id, err)
				}
				if _, err := s.TaskSnapshot(id); err != nil {
					t.Fatal("task record lost")
				}
			} else {
				// Observe the actual registered deadline (3 * 20 s), then expire
				// the real pipe read without a 60-second test sleep.
				for {
					select {
					case at := <-conn.deadlines:
						if time.Until(at) > 50*time.Second {
							a.SetReadDeadline(time.Now())
							goto expired
						}
					case <-time.After(2 * time.Second):
						t.Fatal("missing 3x heartbeat deadline")
					}
				}
			}
		expired:
			deviceEvent(t, s, EventDisconnected)
			got := deviceGet(t, s, "io")
			want := device.HeartbeatTimeout
			if failWrite {
				want = device.WriteError
			}
			if got.Status != device.Offline || got.LatestSession.EndReason != want {
				t.Fatalf("I/O closure: %#v", got)
			}
		})
	}
}

func TestDeviceConcurrentReplacementAndServerClose(t *testing.T) {
	s, address := startTestServer(t)
	const count = 12
	var wg sync.WaitGroup
	errs := make(chan error, count)
	conns := make(chan net.Conn, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := net.Dial("tcp", address)
			if err != nil {
				errs <- err
				return
			}
			conns <- c
			err = protocol.WriteFrame(c, protocol.Frame{Header: protocol.Header{Version: protocol.Version1, Type: protocol.TypeRegister, MessageID: 1}, Payload: []byte(validRegister("same"))})
			if err != nil {
				errs <- err
				return
			}
			c.SetReadDeadline(time.Now().Add(3 * time.Second))
			headerBytes := make([]byte, protocol.HeaderSize)
			_, err = io.ReadFull(c, headerBytes)
			if err == nil {
				var header protocol.Header
				header, err = protocol.DecodeHeader(headerBytes, protocol.MaxControlPayload)
				if err == nil {
					_, err = io.CopyN(io.Discard, c, int64(header.PayloadLen))
				}
			}
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	close(conns)
	defer func() {
		for c := range conns {
			c.Close()
		}
	}()
	for err := range errs {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		deviceEvent(t, s, EventOnline)
	}
	v := deviceGet(t, s, "same")
	if v.TotalSessions != count || v.Status != device.Online || !v.LastOfflineAt.IsZero() {
		t.Fatalf("concurrent replacements: %#v", v)
	}
	h, _ := s.Devices().Sessions("same")
	if len(h.Ended) != count-1 {
		t.Fatal("lost history")
	}
	for _, ended := range h.Ended {
		if ended.EndReason != device.Replaced {
			t.Fatal("replacement emitted offline")
		}
	}
	s.Close()
	closed := deviceGet(t, s, "same")
	if closed.Status != device.Offline || closed.LatestSession.EndReason != device.ServerClosed || !closed.LastOnlineAt.Equal(v.LastOnlineAt) {
		t.Fatalf("close: %#v", closed)
	}
	// A completely new Server has no process-restart recovery.
	fresh, _ := New(Config{})
	defer fresh.Close()
	if len(fresh.Devices().List()) != 0 {
		t.Fatal("unexpected persistence")
	}
}

func TestDeviceTaskTrafficUpdatesActivity(t *testing.T) {
	s, address := startTestServer(t)
	c := deviceConnect(t, s, address, validRegister("task-activity"))
	initial := deviceGet(t, s, "task-activity")
	time.Sleep(20 * time.Millisecond)
	id, err := s.CreateExec(context.Background(), "task-activity", task.ExecRequest{Command: "true", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	frame := readFrame(t, c)
	ack, _ := json.Marshal(map[string]interface{}{"reply_to": frame.Header.MessageID, "task_id": id, "accepted": true, "state": "queued"})
	writeJSONFrameWithFlags(t, c, protocol.TypeTaskAck, protocol.FlagResponse, 2, string(ack))
	waitSeen := func(before time.Time) time.Time {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			at := deviceGet(t, s, "task-activity").LastSeenAt
			if at.After(before) {
				return at
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("task traffic did not update activity")
		return time.Time{}
	}
	afterAck := waitSeen(initial.LastSeenAt)
	time.Sleep(20 * time.Millisecond)
	result, _ := json.Marshal(map[string]interface{}{"task_id": id, "status": "success", "started_at": 1, "finished_at": 2, "exit_code": 0, "stdout": "", "stderr": "", "truncated": false, "result": map[string]interface{}{}})
	writeJSONFrame(t, c, protocol.TypeTaskResult, 3, string(result))
	waitSeen(afterAck)
}

func TestDeviceInventoryDoesNotDependOnEvents(t *testing.T) {
	s, address := startTestServer(t)
	for i := 0; i < cap(s.events); i++ {
		s.events <- SessionEvent{Type: EventOnline}
	}
	c, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	writeJSONFrame(t, c, protocol.TypeRegister, 1, validRegister("no-events"))
	readFrame(t, c)
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := s.Devices().Get("no-events"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("inventory depended on dropped online event")
		}
		time.Sleep(time.Millisecond)
	}
	s.Disconnect("no-events")
	if deviceGet(t, s, "no-events").Status != device.Offline {
		t.Fatal("inventory depended on dropped disconnect event")
	}
}
