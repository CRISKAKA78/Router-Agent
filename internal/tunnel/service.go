// Package tunnel implements fixed-service, session-scoped TCP maintenance.
package tunnel

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultLease = 240 * time.Minute

var ErrNotFound = errors.New("maintenance not found")
var ErrCapacity = errors.New("maintenance capacity exhausted")
var ErrSession = errors.New("device session ended or does not support tunnel")

type Command struct {
	SessionID     string `json:"session_id,omitempty"`
	MaintenanceID string `json:"maintenance_id"`
	ConnectionID  string `json:"connection_id"`
	Service       string `json:"service,omitempty"`
	Token         string `json:"token,omitempty"`
	DataHost      string `json:"data_host,omitempty"`
	DataPort      int    `json:"data_port,omitempty"`
	TimeoutMS     int64  `json:"timeout_ms,omitempty"`
	IdleMS        int64  `json:"idle_ms,omitempty"`
}
type Status struct {
	MaintenanceID string `json:"maintenance_id"`
	ConnectionID  string `json:"connection_id"`
	State         string `json:"state"`
}
type Binding struct {
	ID   string
	Done <-chan struct{}
	// Enqueue must return without waiting for network I/O. The transport owns a
	// bounded queue and rechecks context immediately before writing CONNECT.
	Enqueue func(context.Context, bool, Command) error // bool: close instead of connect
}
type Control interface{ BindTunnel(string) (Binding, error) }
type Config struct {
	BindHost, AdvertisedHost, DataListen, DataHost                                   string
	PortFirst, PortLast                                                              int
	MaxMaintenance, PerMaintenance, PerDevice, TotalConnections, Handshakes, History int
	PendingTimeout, HandshakeTimeout, IdleTimeout                                    time.Duration
	PortReuseDelay                                                                   time.Duration
}

func (c Config) defaults() (Config, error) {
	if c.BindHost == "" {
		c.BindHost = "127.0.0.1"
	}
	if c.AdvertisedHost == "" {
		c.AdvertisedHost = "127.0.0.1"
	}
	if c.DataListen == "" {
		c.DataListen = "127.0.0.1:9001"
	}
	if c.DataHost == "" {
		c.DataHost = "127.0.0.1"
	}
	if c.PortFirst == 0 && c.PortLast == 0 {
		c.PortFirst = 20000
		c.PortLast = 20199
	}
	if c.MaxMaintenance == 0 {
		c.MaxMaintenance = 64
	}
	if c.PerMaintenance == 0 {
		c.PerMaintenance = 8
	}
	if c.PerDevice == 0 {
		c.PerDevice = 8
	}
	if c.TotalConnections == 0 {
		c.TotalConnections = 512
	}
	if c.Handshakes == 0 {
		c.Handshakes = 64
	}
	if c.History == 0 {
		c.History = 128
	}
	if c.PendingTimeout == 0 {
		c.PendingTimeout = 10 * time.Second
	}
	if c.HandshakeTimeout == 0 {
		c.HandshakeTimeout = 5 * time.Second
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 24 * time.Hour
	}
	if c.PortReuseDelay == 0 {
		c.PortReuseDelay = 24 * time.Hour
	}
	if net.ParseIP(c.BindHost) == nil || !validDataHost(c.DataHost) || c.AdvertisedHost == "" || c.PortFirst < 1 || c.PortLast > 65535 || c.PortFirst > c.PortLast || c.MaxMaintenance < 1 || c.PerMaintenance < 1 || c.PerDevice < 1 || c.TotalConnections < 1 || c.Handshakes < 1 || c.History < 1 || c.PendingTimeout < time.Millisecond || c.PendingTimeout > time.Minute || c.HandshakeTimeout < time.Millisecond || c.HandshakeTimeout > time.Minute || c.IdleTimeout < time.Millisecond || c.IdleTimeout > 24*time.Hour || c.PortReuseDelay < time.Millisecond {
		return c, errors.New("invalid tunnel configuration")
	}
	return c, nil
}

func validDataHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	host = strings.TrimSuffix(host, ".")
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

func resolveDataHost(ctx context.Context, host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resolver := net.Resolver{}
	ips, err := resolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return "", fmt.Errorf("resolve tunnel data host: %w", err)
	}
	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.String(), nil
		}
	}
	if len(ips) == 0 {
		return "", errors.New("tunnel data host has no IP addresses")
	}
	return ips[0].String(), nil
}

type Endpoint struct {
	Service, Host, State string
	Port                 int
}
type Snapshot struct {
	ID, DeviceID, SessionID, State, Reason string
	CreatedAt, ExpiresAt                   time.Time
	ReusableAfter                          time.Time
	Released                               bool
	Endpoints                              []Endpoint
	Connections                            int
}
type maintenance struct {
	snapshot  Snapshot
	binding   Binding
	dataHost  string
	listeners []net.Listener
	streams   map[string]*stream
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	wg        sync.WaitGroup
}
type stream struct {
	id, token, service string
	external           net.Conn
	data               net.Conn
	paired             chan struct{}
	failed             chan struct{}
	consumed           bool
	deadline           time.Time
}
type Service struct {
	mu         sync.Mutex
	config     Config
	control    Control
	data       net.Listener
	sessions   map[string]*maintenance
	ports      map[int]bool
	quarantine map[int]time.Time
	creates    chan struct{}
	handshakes map[net.Conn]bool
	history    []string
	total      int
	closed     bool
	done       chan struct{}
	wg         sync.WaitGroup
}

func New(config Config, control Control) (*Service, error) {
	c, e := config.defaults()
	if e != nil {
		return nil, e
	}
	if control == nil {
		return nil, errors.New("missing tunnel control")
	}
	l, e := net.Listen("tcp", c.DataListen)
	if e != nil {
		return nil, e
	}
	s := &Service{config: c, control: control, data: l, sessions: make(map[string]*maintenance), ports: make(map[int]bool), quarantine: make(map[int]time.Time), creates: make(chan struct{}, c.MaxMaintenance), handshakes: make(map[net.Conn]bool), done: make(chan struct{})}
	s.wg.Add(1)
	go s.acceptData()
	return s, nil
}
func randomID(n int) (string, error) {
	b := make([]byte, n)
	_, e := rand.Read(b)
	return hex.EncodeToString(b), e
}
func ended(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}
func (s *Service) DataAddress() string { return s.data.Addr().String() }
func (s *Service) Create(ctx context.Context, deviceID string, lease time.Duration) (Snapshot, error) {
	if e := ctx.Err(); e != nil {
		return Snapshot{}, e
	}
	if lease == 0 {
		lease = DefaultLease
	}
	if lease < time.Millisecond {
		return Snapshot{}, errors.New("lease must be at least one millisecond")
	}
	select {
	case s.creates <- struct{}{}:
		defer func() { <-s.creates }()
	default:
		return Snapshot{}, ErrCapacity
	}
	binding, e := s.control.BindTunnel(deviceID)
	if e != nil {
		return Snapshot{}, e
	}
	dataHost, e := resolveDataHost(ctx, s.config.DataHost)
	if e != nil {
		return Snapshot{}, e
	}
	id, e := randomID(16)
	if e != nil {
		return Snapshot{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Snapshot{}, net.ErrClosed
	}
	if ended(binding.Done) {
		return Snapshot{}, ErrSession
	}
	live := 0
	for _, m := range s.sessions {
		if !m.snapshot.Released {
			live++
			if m.snapshot.DeviceID == deviceID {
				return Snapshot{}, errors.New("device already has an unreleased maintenance session")
			}
		}
	}
	if live >= s.config.MaxMaintenance {
		return Snapshot{}, ErrCapacity
	}
	m := &maintenance{binding: binding, dataHost: dataHost, done: make(chan struct{}), streams: make(map[string]*stream)}
	m.ctx, m.cancel = context.WithCancel(context.Background())
	now := time.Now()
	m.snapshot = Snapshot{ID: id, DeviceID: deviceID, SessionID: binding.ID, State: "ready", CreatedAt: now, ExpiresAt: now.Add(lease)}
	rollback := func() {
		for i, l := range m.listeners {
			l.Close()
			delete(s.ports, m.snapshot.Endpoints[i].Port)
		}
		m.cancel()
	}
	for _, service := range []string{"web", "ssh", "telnet"} {
		var listener net.Listener
		port := 0
		for p := s.config.PortFirst; p <= s.config.PortLast; p++ {
			if s.ports[p] || now.Before(s.quarantine[p]) {
				continue
			}
			l, err := net.Listen("tcp", net.JoinHostPort(s.config.BindHost, strconv.Itoa(p)))
			if err == nil {
				delete(s.quarantine, p)
				listener = l
				port = p
				break
			}
		}
		if listener == nil {
			rollback()
			return Snapshot{}, ErrCapacity
		}
		s.ports[port] = true
		m.listeners = append(m.listeners, listener)
		m.snapshot.Endpoints = append(m.snapshot.Endpoints, Endpoint{Service: service, Host: s.config.AdvertisedHost, Port: port, State: "ready"})
	}
	if e := ctx.Err(); e != nil {
		rollback()
		return Snapshot{}, e
	}
	if ended(binding.Done) {
		rollback()
		return Snapshot{}, ErrSession
	}
	s.sessions[id] = m
	for i, l := range m.listeners {
		m.wg.Add(1)
		go s.acceptExternal(m, l, m.snapshot.Endpoints[i].Service)
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		timer := time.NewTimer(time.Until(m.snapshot.ExpiresAt))
		defer timer.Stop()
		reason := "requested"
		select {
		case <-binding.Done:
			reason = "session_ended"
		case <-timer.C:
			reason = "expired"
		case <-m.ctx.Done():
		}
		s.closeMaintenance(m, reason)
	}()
	return snapshot(m), nil
}
func snapshot(m *maintenance) Snapshot {
	v := m.snapshot
	v.Endpoints = append([]Endpoint(nil), v.Endpoints...)
	v.Connections = len(m.streams)
	return v
}
func (s *Service) Get(id string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.sessions[id]
	if m == nil {
		return Snapshot{}, ErrNotFound
	}
	return snapshot(m), nil
}
func (s *Service) List() []Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Snapshot, 0, len(s.sessions))
	for _, m := range s.sessions {
		out = append(out, snapshot(m))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (s *Service) CloseMaintenance(id string) error {
	s.mu.Lock()
	m := s.sessions[id]
	s.mu.Unlock()
	if m == nil {
		return nil // Also idempotent after bounded history eviction or restart.
	}
	s.closeMaintenance(m, "requested")
	return nil
}
func (s *Service) closeMaintenance(m *maintenance, reason string) {
	s.mu.Lock()
	if m.snapshot.State == "closing" || m.snapshot.Released {
		s.mu.Unlock()
		<-m.done
		return
	}
	m.snapshot.State = "closing"
	m.snapshot.Reason = reason
	// Revoke listeners before touching any pending or active stream.
	for _, l := range m.listeners {
		l.Close()
	}
	m.cancel()
	for _, c := range m.streams {
		if c.data != nil {
			abortTCP(c.data)
		}
		c.external.Close()
	}
	s.mu.Unlock()
	m.wg.Wait()
	s.mu.Lock()
	m.snapshot.ReusableAfter = time.Now().Add(s.config.PortReuseDelay)
	for i := range m.snapshot.Endpoints {
		delete(s.ports, m.snapshot.Endpoints[i].Port)
		s.quarantine[m.snapshot.Endpoints[i].Port] = m.snapshot.ReusableAfter
		m.snapshot.Endpoints[i].State = "closed"
	}
	m.listeners = nil
	m.snapshot.State = "closed"
	m.snapshot.Released = true
	s.history = append(s.history, m.snapshot.ID)
	if len(s.history) > s.config.History {
		delete(s.sessions, s.history[0])
		s.history = s.history[1:]
	}
	close(m.done)
	s.mu.Unlock()
	// Admission only: Gateway owns network delivery and its bounded worker.
	_ = m.binding.Enqueue(context.Background(), true, Command{MaintenanceID: m.snapshot.ID})
}

func abortTCP(c net.Conn) {
	if tcp, ok := c.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
	_ = c.Close()
}
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		<-s.done
		return nil
	}
	s.closed = true
	s.data.Close()
	ms := make([]*maintenance, 0, len(s.sessions))
	for _, m := range s.sessions {
		ms = append(ms, m)
	}
	s.mu.Unlock()
	for _, m := range ms {
		s.closeMaintenance(m, "server_closed")
	}
	s.mu.Lock()
	for c := range s.handshakes {
		abortTCP(c)
	}
	s.mu.Unlock()
	s.wg.Wait()
	close(s.done)
	return nil
}
func (s *Service) valid(m *maintenance) bool {
	return !s.closed && m.snapshot.State == "ready" && !ended(m.binding.Done) && time.Now().Before(m.snapshot.ExpiresAt)
}
func (s *Service) acceptExternal(m *maintenance, l net.Listener, service string) {
	defer m.wg.Done()
	for {
		external, e := l.Accept()
		if e != nil {
			return
		}
		s.mu.Lock()
		deviceCount := 0
		for _, v := range s.sessions {
			if v.snapshot.DeviceID == m.snapshot.DeviceID {
				deviceCount += len(v.streams)
			}
		}
		if !s.valid(m) || len(m.streams) >= s.config.PerMaintenance || deviceCount >= s.config.PerDevice || s.total >= s.config.TotalConnections {
			s.mu.Unlock()
			external.Close()
			continue
		}
		id, e := randomID(16)
		token, e2 := randomID(32)
		if e != nil || e2 != nil {
			s.mu.Unlock()
			external.Close()
			continue
		}
		c := &stream{id: id, token: token, service: service, external: external, paired: make(chan struct{}), failed: make(chan struct{}), deadline: time.Now().Add(s.config.PendingTimeout)}
		m.streams[id] = c
		s.total++
		m.wg.Add(1)
		s.mu.Unlock()
		go s.runStream(m, c)
	}
}
func (s *Service) runStream(m *maintenance, c *stream) {
	defer m.wg.Done()
	sent := false
	completed := false
	defer func() {
		s.mu.Lock()
		if c.data != nil {
			if completed {
				c.data.Close()
			} else {
				abortTCP(c.data)
			}
		}
		c.external.Close()
		s.mu.Unlock()
		// This bounded admission never waits for a control writer.
		if sent && !ended(m.ctx.Done()) {
			_ = m.binding.Enqueue(m.ctx, true, Command{MaintenanceID: m.snapshot.ID, ConnectionID: c.id})
		}
		s.mu.Lock()
		delete(m.streams, c.id)
		s.total--
		s.mu.Unlock()
	}()
	_, port, _ := net.SplitHostPort(s.data.Addr().String())
	p, _ := strconv.Atoi(port)
	cmd := Command{SessionID: m.binding.ID, MaintenanceID: m.snapshot.ID, ConnectionID: c.id, Service: c.service, Token: c.token, DataHost: m.dataHost, DataPort: p, TimeoutMS: s.config.PendingTimeout.Milliseconds(), IdleMS: s.config.IdleTimeout.Milliseconds()}
	ctx, cancel := context.WithDeadline(m.ctx, c.deadline)
	defer cancel()
	if e := m.binding.Enqueue(ctx, false, cmd); e != nil {
		return
	}
	sent = true
	select {
	case <-ctx.Done():
		return
	case <-c.failed:
		return
	case <-c.paired:
	}
	s.mu.Lock()
	data := c.data
	valid := s.valid(m)
	s.mu.Unlock()
	if !valid {
		return
	}
	completed = relay(c.external, data, s.config.IdleTimeout)
}
func (s *Service) acceptData() {
	defer s.wg.Done()
	for {
		c, e := s.data.Accept()
		if e != nil {
			return
		}
		s.mu.Lock()
		if s.closed || len(s.handshakes) >= s.config.Handshakes {
			s.mu.Unlock()
			c.Close()
			continue
		}
		s.handshakes[c] = true
		s.wg.Add(1)
		s.mu.Unlock()
		go s.pair(c)
	}
}
func (s *Service) pair(data net.Conn) {
	defer s.wg.Done()
	owned := false
	defer func() {
		s.mu.Lock()
		delete(s.handshakes, data)
		s.mu.Unlock()
		if !owned {
			data.Close()
		}
	}()
	_ = data.SetDeadline(time.Now().Add(s.config.HandshakeTimeout))
	var h [132]byte
	if _, e := io.ReadFull(data, h[:]); e != nil || string(h[:4]) != "RMT1" {
		return
	}
	mid, cid, token := string(h[4:36]), string(h[36:68]), string(h[68:])
	s.mu.Lock()
	m := s.sessions[mid]
	if m == nil || !s.valid(m) {
		s.mu.Unlock()
		return
	}
	c := m.streams[cid]
	if c == nil || c.consumed || ended(c.failed) || time.Now().After(c.deadline) || subtle.ConstantTimeCompare([]byte(c.token), []byte(token)) != 1 {
		s.mu.Unlock()
		return
	}
	c.consumed = true
	c.token = ""
	c.data = data
	owned = true
	// Add before Close can begin waiting. Release requires handshake ACK completion.
	m.wg.Add(1)
	s.mu.Unlock()
	defer m.wg.Done()
	if _, e := data.Write([]byte{1}); e != nil {
		data.Close()
		s.mu.Lock()
		if !ended(c.failed) {
			close(c.failed)
		}
		s.mu.Unlock()
		return
	}
	_ = data.SetDeadline(time.Time{})
	s.mu.Lock()
	if s.valid(m) {
		for i := range m.snapshot.Endpoints {
			if m.snapshot.Endpoints[i].Service == c.service {
				m.snapshot.Endpoints[i].State = "ready"
			}
		}
	}
	close(c.paired)
	s.mu.Unlock()
}
func (s *Service) Report(sessionID string, status Status) error {
	if status.State != "local_unavailable" && status.State != "data_failed" && status.State != "busy" {
		return errors.New("invalid tunnel status")
	}
	if len(status.MaintenanceID) != 32 || len(status.ConnectionID) != 32 {
		return errors.New("invalid tunnel identity")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.sessions[status.MaintenanceID]
	if m == nil || m.binding.ID != sessionID || !s.valid(m) {
		return nil
	}
	c := m.streams[status.ConnectionID]
	if c == nil || c.consumed || ended(c.failed) {
		return nil
	}
	if status.State == "local_unavailable" {
		for i := range m.snapshot.Endpoints {
			if m.snapshot.Endpoints[i].Service == c.service {
				m.snapshot.Endpoints[i].State = "unavailable"
			}
		}
	}
	close(c.failed)
	return nil
}
func relay(a, b net.Conn, idle time.Duration) bool {
	var eof atomic.Int32
	// Both directions share one activity deadline. Updating deadlines also wakes
	// an already blocked Read/Write; no polling timer or extra monitor is needed.
	var activity sync.Mutex
	touch := func() {
		activity.Lock()
		defer activity.Unlock()
		until := time.Now().Add(idle)
		_ = a.SetDeadline(until)
		_ = b.SetDeadline(until)
	}
	touch()
	var wg sync.WaitGroup
	wg.Add(2)
	copyDirection := func(dst, src net.Conn) {
		defer wg.Done()
		buffer := make([]byte, 32*1024)
		for {
			n, e := src.Read(buffer)
			if n > 0 {
				touch()
				if _, we := writeAll(activityWriter{dst, touch}, buffer[:n]); we != nil {
					abortTCP(b)
					a.Close()
					return
				}
			}
			if e != nil {
				if e == io.EOF {
					eof.Add(1)
					if tcp, ok := dst.(*net.TCPConn); ok {
						_ = tcp.CloseWrite()
					} else {
						dst.Close()
					}
				} else {
					abortTCP(b)
					a.Close()
				}
				return
			}
		}
	}
	go copyDirection(a, b)
	go copyDirection(b, a)
	wg.Wait()
	return eof.Load() == 2
}

type activityWriter struct {
	io.Writer
	touch func()
}

func (w activityWriter) Write(p []byte) (int, error) {
	n, e := w.Writer.Write(p)
	if n > 0 {
		w.touch()
	}
	return n, e
}
func writeAll(w io.Writer, b []byte) (int, error) {
	total := 0
	for len(b) > 0 {
		n, e := w.Write(b)
		total += n
		b = b[n:]
		if e != nil {
			return total, e
		}
		if n == 0 {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}
func (e Endpoint) Address() string { return net.JoinHostPort(e.Host, strconv.Itoa(e.Port)) }
func (s Snapshot) String() string {
	return fmt.Sprintf("maintenance=%s device=%s state=%s", s.ID, s.DeviceID, s.State)
}
