package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"routerprobe/internal/device"
	"routerprobe/internal/tunnel"
	"strings"
	"testing"
	"time"
)

func TestTunnelBindingReplacementAndWriterRevalidation(t *testing.T) {
	s, address := startTestServer(t)
	payload := `{"device_id":"bound","probe_version":"v1","arch":"arm","boot_id":"boot","capabilities":["tunnel"]}`
	deviceConnect(t, s, address, payload)
	binding, e := s.BindTunnel("bound")
	if e != nil {
		t.Fatal(e)
	}
	s.mu.Lock()
	old := s.sessions["bound"]
	s.mu.Unlock()
	old.transport.priority.lock(false)
	if e := binding.Enqueue(context.Background(), false, tunnel.Command{MaintenanceID: "old"}); e != nil {
		t.Fatal(e)
	}
	deviceConnect(t, s, address, payload)
	select {
	case <-binding.Done:
	default:
		t.Fatal("replacement did not synchronously revoke binding")
	}
	old.transport.priority.unlock()
	select {
	case <-old.tunnelDone:
	case <-time.After(time.Second):
		t.Fatal("send stuck")
	}
	if e := binding.Enqueue(context.Background(), false, tunnel.Command{}); e == nil {
		t.Fatal("stale queue admission")
	}
	current, e := s.BindTunnel("bound")
	if e != nil || current.ID == binding.ID {
		t.Fatal(current, e)
	}
	s.endSession(old, device.Disconnected)
	select {
	case <-current.Done:
		t.Fatal("late old cleanup revoked new session")
	default:
	}
	s.Disconnect("bound")
	select {
	case <-current.Done:
	default:
		t.Fatal("disconnect did not revoke")
	}
}

func TestMaintenanceReleaseIndependentOfBlockedControlAndFullQueue(t *testing.T) {
	g, address := startTestServer(t)
	peer := deviceConnect(t, g, address, `{"device_id":"blocked","probe_version":"v1","arch":"arm","boot_id":"boot","capabilities":["tunnel"]}`)
	s, e := tunnel.New(tunnel.Config{DataListen: "127.0.0.1:0", PortFirst: 28000, PortLast: 28100}, g)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	m, e := s.Create(context.Background(), "blocked", 0)
	if e != nil {
		t.Fatal(e)
	}
	client, e := net.Dial("tcp", m.Endpoints[0].Address())
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	frame := readFrame(t, peer)
	var command tunnel.Command
	if e := json.Unmarshal(frame.Payload, &command); e != nil {
		t.Fatal(e)
	}
	data, e := net.Dial("tcp", s.DataAddress())
	if e != nil {
		t.Fatal(e)
	}
	defer data.Close()
	data.SetDeadline(time.Now().Add(time.Second))
	io.WriteString(data, "RMT1"+command.MaintenanceID+command.ConnectionID+command.Token)
	var ack [1]byte
	if _, e := io.ReadFull(data, ack[:]); e != nil || ack[0] != 1 {
		t.Fatal(e, ack)
	}
	g.mu.Lock()
	active := g.sessions["blocked"]
	g.mu.Unlock()
	active.transport.priority.lock(false)
	locked := true
	defer func() {
		if locked {
			active.transport.priority.unlock()
		}
	}()
	binding, e := g.BindTunnel("blocked")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	full := false
	for i := 0; i < 66; i++ {
		e = binding.Enqueue(ctx, false, tunnel.Command{MaintenanceID: strings.Repeat("a", 32)})
		if e == tunnel.ErrCapacity {
			full = true
			break
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	if !full {
		t.Fatal("unbounded control queue")
	}
	// Also register a pending external stream while control delivery is blocked.
	pending, e := net.Dial("tcp", m.Endpoints[1].Address())
	if e != nil {
		t.Fatal(e)
	}
	defer pending.Close()
	finished := make(chan error, 1)
	go func() { finished <- s.CloseMaintenance(m.ID) }()
	select {
	case e := <-finished:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("local release waited on control writer")
	}
	snap, _ := s.Get(m.ID)
	if !snap.Released || snap.Connections != 0 {
		t.Fatal(snap)
	}
	for _, c := range []net.Conn{client, data, pending} {
		c.SetReadDeadline(time.Now().Add(time.Second))
		_, e := c.Read(ack[:])
		if e == nil {
			t.Fatal("socket survived release")
		}
		if ne, ok := e.(net.Error); ok && ne.Timeout() {
			t.Fatal("socket not revoked")
		}
		if c == data && e == io.EOF {
			t.Fatal("revocation confused with normal half-close")
		}
	}
	cancel()
	active.transport.priority.unlock()
	locked = false
	// A marker behind the cancelled queue proves queued CONNECTs are skipped.
	until := time.Now().Add(time.Second)
	for len(active.tunnelQueue) > 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if e := binding.Enqueue(context.Background(), true, tunnel.Command{MaintenanceID: "marker"}); e != nil {
		t.Fatal(e)
	}
	frame = readFrame(t, peer)
	if !strings.Contains(string(frame.Payload), "marker") {
		t.Fatalf("cancelled CONNECT written: %s", frame.Payload)
	}
}
