package tunnel

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeControl struct {
	pause    <-chan struct{}
	commands chan Command
	lives    map[string]chan struct{}
}

func (f *fakeControl) BindTunnel(id string) (Binding, error) {
	d := f.lives[id]
	if d == nil {
		return Binding{}, ErrSession
	}
	return Binding{ID: id, Done: d, Enqueue: func(ctx context.Context, close bool, c Command) error {
		if close {
			return nil
		}
		if f.pause != nil {
			return nil // Simulate an admitted command waiting in the transport queue.
		}
		select {
		case f.commands <- c:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}, nil
}
func newTestService(t *testing.T, change func(*Config)) (*Service, *fakeControl) {
	t.Helper()
	f := &fakeControl{commands: make(chan Command, 128), lives: map[string]chan struct{}{"a": make(chan struct{}), "b": make(chan struct{})}}
	c := Config{DataListen: "127.0.0.1:0", PortFirst: 24000, PortLast: 24999, PendingTimeout: time.Second, HandshakeTimeout: 200 * time.Millisecond, IdleTimeout: time.Second}
	if change != nil {
		change(&c)
	}
	s, e := New(c, f)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s, f
}
func create(t *testing.T, s *Service, device string, lease time.Duration) Snapshot {
	t.Helper()
	m, e := s.Create(context.Background(), device, lease)
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func dial(t *testing.T, address string) net.Conn {
	t.Helper()
	c, e := net.DialTimeout("tcp", address, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	return c
}
func command(t *testing.T, f *fakeControl) Command {
	t.Helper()
	select {
	case c := <-f.commands:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("no connect command")
	}
	return Command{}
}
func pairTest(t *testing.T, s *Service, c Command) net.Conn {
	t.Helper()
	d := dial(t, s.DataAddress())
	d.SetDeadline(time.Now().Add(2 * time.Second))
	_, e := d.Write([]byte("RMT1" + c.MaintenanceID + c.ConnectionID + c.Token))
	if e != nil {
		t.Fatal(e)
	}
	b := make([]byte, 1)
	if _, e = io.ReadFull(d, b); e != nil || b[0] != 1 {
		t.Fatalf("pair ack %v %v", b, e)
	}
	d.SetDeadline(time.Time{})
	return d
}
func echoBytes(t *testing.T, a, b net.Conn) {
	t.Helper()
	a.SetDeadline(time.Now().Add(time.Second))
	b.SetDeadline(time.Now().Add(time.Second))
	payload := []byte{0, 1, 2, 255, 'x'}
	if _, e := a.Write(payload); e != nil {
		t.Fatal(e)
	}
	got := make([]byte, len(payload))
	if _, e := io.ReadFull(b, got); e != nil || !bytes.Equal(got, payload) {
		t.Fatalf("relay: %x %v", got, e)
	}
}
func closed(t *testing.T, c net.Conn) {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	var b [65536]byte
	for {
		_, e := c.Read(b[:])
		if e != nil {
			if n, ok := e.(net.Error); ok && n.Timeout() {
				t.Fatal("socket still open")
			}
			return
		}
	}
}
func waitReleased(t *testing.T, s *Service, id string) {
	t.Helper()
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		v, e := s.Get(id)
		if e == nil && v.Released {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("not released")
}
func TestMaintenanceDefaultCustomConcurrentHalfCloseAndIsolation(t *testing.T) {
	s, f := newTestService(t, nil)
	m := create(t, s, "a", 0)
	if m.ExpiresAt.Sub(m.CreatedAt) != DefaultLease || len(m.Endpoints) != 3 {
		t.Fatal(m)
	}
	other := create(t, s, "b", 5*time.Minute)
	if other.ExpiresAt.Sub(other.CreatedAt) != 5*time.Minute {
		t.Fatal(other)
	}
	var clients, data []net.Conn
	for i := 0; i < 6; i++ {
		a := dial(t, m.Endpoints[i%3].Address())
		b := pairTest(t, s, command(t, f))
		clients = append(clients, a)
		data = append(data, b)
		echoBytes(t, a, b)
		echoBytes(t, b, a)
	}
	bClient := dial(t, other.Endpoints[0].Address())
	bData := pairTest(t, s, command(t, f))
	clients[0].(*net.TCPConn).CloseWrite()
	data[0].SetReadDeadline(time.Now().Add(time.Second))
	tail, e := io.ReadAll(data[0])
	if e != nil || len(tail) != 0 {
		t.Fatal(e, tail)
	}
	echoBytes(t, data[0], clients[0])
	data[0].(*net.TCPConn).CloseWrite()
	closed(t, clients[0])
	clients[0].Close()
	echoBytes(t, clients[1], data[1])
	echoBytes(t, bClient, bData)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := s.CloseMaintenance(m.ID); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	for _, c := range clients {
		closed(t, c)
	}
	for _, d := range data {
		closed(t, d)
	}
	echoBytes(t, bClient, bData)
	next := create(t, s, "a", time.Hour)
	for i, e := range m.Endpoints {
		if next.Endpoints[i].Port == e.Port {
			t.Fatal("quarantined port reused")
		}
	}
	s.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.total != 0 || len(s.ports) != 0 || len(s.handshakes) != 0 {
		t.Fatal("resources retained")
	}
}
func TestPairTokensTimeoutUnavailableAndRevocation(t *testing.T) {
	s, f := newTestService(t, nil)
	m := create(t, s, "a", 0)
	a := dial(t, m.Endpoints[0].Address())
	c := command(t, f)
	bad := dial(t, s.DataAddress())
	bad.Write([]byte("RMT1" + c.MaintenanceID + c.ConnectionID + strings.Repeat("0", 64)))
	closed(t, bad)
	good := pairTest(t, s, c)
	duplicate := dial(t, s.DataAddress())
	duplicate.Write([]byte("RMT1" + c.MaintenanceID + c.ConnectionID + c.Token))
	closed(t, duplicate)
	echoBytes(t, a, good)
	unavailable := dial(t, m.Endpoints[1].Address())
	u := command(t, f)
	s.Report("a", Status{u.MaintenanceID, u.ConnectionID, "local_unavailable"})
	closed(t, unavailable)
	state, _ := s.Get(m.ID)
	if state.Endpoints[1].State != "unavailable" || state.Endpoints[0].State != "ready" {
		t.Fatal(state)
	}
	pending := dial(t, m.Endpoints[2].Address())
	late := command(t, f)
	closed(t, pending)
	lateData := dial(t, s.DataAddress())
	lateData.Write([]byte("RMT1" + late.MaintenanceID + late.ConnectionID + late.Token))
	closed(t, lateData)
	close(f.lives["a"])
	waitReleased(t, s, m.ID)
	closed(t, a)
	closed(t, good)
}
func TestExpiryLimitsHandshakesBackpressureAndCleanup(t *testing.T) {
	s, f := newTestService(t, func(c *Config) {
		c.PerMaintenance = 1
		c.TotalConnections = 1
		c.Handshakes = 1
		c.History = 1
		c.IdleTimeout = time.Minute
	})
	m := create(t, s, "a", 300*time.Millisecond)
	a := dial(t, m.Endpoints[0].Address())
	d := pairTest(t, s, command(t, f))
	extra := dial(t, m.Endpoints[0].Address())
	closed(t, extra)
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		chunk := make([]byte, 65536)
		for i := 0; i < 1024; i++ {
			if _, e := a.Write(chunk); e != nil {
				return
			}
		}
	}()
	waitReleased(t, s, m.ID)
	closed(t, d)
	select {
	case <-writeDone:
	case <-time.After(time.Second):
		t.Fatal("backpressured writer leaked")
	}
	h1 := dial(t, s.DataAddress())
	time.Sleep(20 * time.Millisecond)
	h2 := dial(t, s.DataAddress())
	closed(t, h2)
	closed(t, h1)
	next := create(t, s, "a", time.Millisecond)
	waitReleased(t, s, next.ID)
	if _, e := s.Get(m.ID); e != ErrNotFound {
		t.Fatal("history not bounded")
	}
	s.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.total != 0 || len(s.handshakes) != 0 || len(s.ports) != 0 {
		t.Fatal("leaked resources")
	}
}
func TestAllocationRollbackAndInput(t *testing.T) {
	s, _ := newTestService(t, func(c *Config) { c.PortFirst = 25001; c.PortLast = 25002 })
	if _, e := s.Create(context.Background(), "a", 0); e != ErrCapacity {
		t.Fatal(e)
	}
	if len(s.ports) != 0 {
		t.Fatal("partial allocation retained")
	}
	if _, e := s.Create(context.Background(), "a", -time.Second); e == nil {
		t.Fatal("negative lease accepted")
	}
	if _, e := New(Config{PerDevice: -1}, nil); e == nil {
		t.Fatal("invalid config")
	}
}

func TestPendingDeadlineRevokesSocketWhileControlWriterBlocked(t *testing.T) {
	s, f := newTestService(t, func(c *Config) { c.PendingTimeout = 40 * time.Millisecond })
	release := make(chan struct{})
	f.pause = release
	m := create(t, s, "a", 0)
	c := dial(t, m.Endpoints[0].Address())
	c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var b [1]byte
	_, e := c.Read(b[:])
	close(release)
	if e == nil {
		t.Fatal("pending socket survived")
	}
	if ne, ok := e.(net.Error); ok && ne.Timeout() {
		t.Fatal("pending timeout waited for control writer")
	}
	s.CloseMaintenance(m.ID)
}

func TestPortQuarantineAndDelayedReuse(t *testing.T) {
	s, _ := newTestService(t, func(c *Config) {
		c.PortFirst = 25010
		c.PortLast = 25012
		c.PortReuseDelay = 200 * time.Millisecond
		c.History = 1
	})
	m := create(t, s, "a", 0)
	if e := s.CloseMaintenance(m.ID); e != nil {
		t.Fatal(e)
	}
	closedSnapshot, _ := s.Get(m.ID)
	if !closedSnapshot.Released || !closedSnapshot.ReusableAfter.After(time.Now()) {
		t.Fatal(closedSnapshot)
	}
	if _, e := s.Create(context.Background(), "b", 0); e != ErrCapacity {
		t.Fatal("quarantine bypassed", e)
	}
	for _, entry := range m.Endpoints {
		c, e := net.DialTimeout("tcp", entry.Address(), 30*time.Millisecond)
		if e == nil {
			c.Close()
			t.Fatal("old client reconnected during quarantine")
		}
	}
	time.Sleep(time.Until(closedSnapshot.ReusableAfter) + 10*time.Millisecond)
	next := create(t, s, "b", 0)
	if next.ID == m.ID || next.Endpoints[0].Port != m.Endpoints[0].Port {
		t.Fatal("delayed pool reuse failed", next)
	}
	s.CloseMaintenance(next.ID)
	if _, e := s.Get(m.ID); e != ErrNotFound {
		t.Fatal("history not evicted")
	}
	if _, e := s.Create(context.Background(), "a", 0); e != ErrCapacity {
		t.Fatal("history eviction erased quarantine", e)
	}
}

func TestDataHostResolutionAndDefaults(t *testing.T) {
	c, e := (Config{}).defaults()
	if e != nil || c.IdleTimeout != 24*time.Hour || c.PortReuseDelay != 24*time.Hour || c.PerMaintenance != 8 || c.PerDevice != 8 {
		t.Fatal(c, e)
	}
	for _, host := range []string{"https://example.com", "host:9001", "bad host", "bad..host", "-host"} {
		if _, e := (Config{DataHost: host}).defaults(); e == nil {
			t.Fatal("invalid host accepted", host)
		}
	}
	for _, host := range []string{"example.com", "example.com.", "127.0.0.1", "::1"} {
		if _, e := (Config{DataHost: host}).defaults(); e != nil {
			t.Fatal(host, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := resolveDataHost(ctx, "cancelled.invalid"); e == nil {
		t.Fatal("cancelled DNS succeeded")
	}
	s, f := newTestService(t, func(c *Config) { c.DataHost = "localhost" })
	m := create(t, s, "a", 0)
	client := dial(t, m.Endpoints[0].Address())
	command := command(t, f)
	if command.DataHost != "127.0.0.1" {
		t.Fatal("expected server-resolved IPv4", command.DataHost)
	}
	data := pairTest(t, s, command)
	echoBytes(t, client, data)
}

func TestWholeConnectionIdleAndOneWayAfterHalfClose(t *testing.T) {
	s, f := newTestService(t, func(c *Config) { c.IdleTimeout = 300 * time.Millisecond })
	m := create(t, s, "a", 0)
	client := dial(t, m.Endpoints[0].Address())
	data := pairTest(t, s, command(t, f))
	client.(*net.TCPConn).CloseWrite()
	data.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	if _, e := data.Read(b[:]); e != io.EOF {
		t.Fatal("missing half-close", e)
	}
	for i := 0; i < 10; i++ {
		time.Sleep(75 * time.Millisecond)
		echoBytes(t, data, client)
	}
	// No direction now progresses; the explicitly short idle limit must fire.
	closed(t, client)
	closed(t, data)
}
