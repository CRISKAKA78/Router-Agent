package forwarding

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"routerprobe/internal/serialauth"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Binary, BindHost, Host, StateFile string
	PortFirst, PortLast               int
}
type mapping struct {
	v           Snapshot
	b           Binding
	cancel      context.CancelFunc
	done        chan struct{}
	deviceReady chan struct{}
	readyOnce   sync.Once
	process     *Process
	gate        *serialauth.Gate
	listener    net.Listener
	relayPort   int
	command     Command
}
type query struct {
	session string
	result  chan Status
}
type Service struct {
	mu      sync.Mutex
	cfg     Config
	control Control
	items   map[string]*mapping
	queries map[string]query
	ports   map[int]time.Time
	closed  bool
}

func New(cfg Config, c Control) (*Service, error) {
	if cfg.Binary == "" || cfg.Binary == "gost" {
		exe, _ := os.Executable()
		name := "gost"
		if strings.HasSuffix(strings.ToLower(exe), ".exe") {
			name += ".exe"
		}
		candidate := filepath.Join(filepath.Dir(exe), name)
		if BackendAvailable(candidate) {
			cfg.Binary = candidate
		}
	}
	if cfg.BindHost == "" {
		cfg.BindHost = "0.0.0.0"
	}
	if cfg.Host == "" {
		cfg.Host = "47.119.168.150"
	}
	if cfg.PortFirst == 0 {
		cfg.PortFirst = 22000
	}
	if cfg.PortLast == 0 {
		cfg.PortLast = 22399
	}
	if net.ParseIP(cfg.BindHost) == nil || cfg.PortFirst < 1 || cfg.PortLast > 65535 || cfg.PortFirst >= cfg.PortLast {
		return nil, ErrInvalid
	}
	s := &Service{cfg: cfg, control: c, items: map[string]*mapping{}, queries: map[string]query{}, ports: map[int]time.Time{}}
	if cfg.StateFile != "" {
		b, e := os.ReadFile(cfg.StateFile)
		if e == nil {
			if e = json.Unmarshal(b, &s.ports); e != nil {
				return nil, e
			}
		} else if !os.IsNotExist(e) {
			return nil, e
		}
	}
	if s.ports == nil {
		return nil, ErrInvalid
	}
	// Active reservations survive a crash, but become a fresh quarantine on restart.
	// Channels are session-owned and are deliberately never restored.
	limit := time.Now().Add(24 * time.Hour)
	changed := false
	for port, until := range s.ports {
		if until.After(limit) {
			s.ports[port] = limit
			changed = true
		}
	}
	if changed {
		if err := s.savePorts(); err != nil {
			return nil, err
		}
	}
	return s, nil
}
func (s *Service) savePorts() error {
	if s.cfg.StateFile == "" {
		return nil
	}
	b, e := json.Marshal(s.ports)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(s.cfg.StateFile), 0700); e != nil {
		return e
	}
	tmp := s.cfg.StateFile + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, s.cfg.StateFile)
}
func (s *Service) reserve() (int, error) {
	for p := s.cfg.PortFirst; p <= s.cfg.PortLast; p++ {
		if time.Now().Before(s.ports[p]) {
			continue
		}
		a := net.JoinHostPort(s.cfg.BindHost, strconv.Itoa(p))
		l, e := net.Listen("tcp", a)
		if e != nil {
			continue
		}
		u, e := net.ListenPacket("udp", a)
		l.Close()
		if e != nil {
			continue
		}
		u.Close()
		s.ports[p] = time.Now().AddDate(100, 0, 0)
		if e = s.savePorts(); e != nil {
			return 0, e
		}
		return p, nil
	}
	return 0, ErrCapacity
}
func (s *Service) Inventory(ctx context.Context, device string) (Inventory, error) {
	if !BackendAvailable(s.cfg.Binary) {
		return Inventory{}, ErrUnavailable
	}
	b, e := s.control.BindForwarding(device)
	if e != nil {
		return Inventory{}, e
	}
	id, e := serialauth.NewToken()
	if e != nil {
		return Inventory{}, e
	}
	ch := make(chan Status, 1)
	s.mu.Lock()
	if s.closed || len(s.queries) >= 32 {
		s.mu.Unlock()
		return Inventory{}, ErrCapacity
	}
	s.queries[id] = query{b.ID, ch}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.queries, id); s.mu.Unlock() }()
	if e = b.Enqueue(ctx, Command{ID: id, SessionID: b.ID, Op: "inventory"}); e != nil {
		return Inventory{}, e
	}
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return Inventory{}, ctx.Err()
	case <-b.Done:
		return Inventory{}, ErrUnavailable
	case <-timer.C:
		return Inventory{}, ErrUnavailable
	case r := <-ch:
		if r.Inventory == nil {
			return Inventory{}, ErrUnavailable
		}
		return *r.Inventory, nil
	}
}
func (s *Service) Create(ctx context.Context, q Request) (Created, error) {
	if e := q.Normalize(); e != nil {
		return Created{}, e
	}
	if !BackendAvailable(s.cfg.Binary) {
		return Created{}, ErrUnavailable
	}
	b, e := s.control.BindForwarding(q.DeviceID)
	if e != nil {
		return Created{}, e
	}
	id, e := serialauth.NewToken()
	if e != nil {
		return Created{}, e
	}
	secret, e := serialauth.NewToken()
	if e != nil {
		return Created{}, e
	}
	token, e := serialauth.NewToken()
	if e != nil {
		return Created{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Created{}, ErrUnavailable
	}
	active := 0
	deviceCount := 0
	for _, m := range s.items {
		if !m.v.Released {
			active++
			if m.v.DeviceID == q.DeviceID {
				deviceCount++
				if q.Kind == "serial" && m.v.Serial == q.Serial {
					return Created{}, fmt.Errorf("serial already reserved")
				}
			}
		}
	}
	if active >= 32 || deviceCount >= 8 {
		return Created{}, ErrCapacity
	}
	port, e := s.reserve()
	if e != nil {
		return Created{}, e
	}
	committed := false
	defer func() {
		if !committed {
			s.ports[port] = time.Now().Add(24 * time.Hour)
			s.savePorts()
		}
	}()
	rp, e := s.reserve()
	if e != nil {
		return Created{}, e
	}
	defer func() {
		if !committed {
			s.ports[rp] = time.Now().Add(24 * time.Hour)
			s.savePorts()
		}
	}()
	now := time.Now().UTC()
	v := Snapshot{ID: id, Request: q, SessionID: b.ID, State: "creating", Host: s.cfg.Host, Port: port, CreatedAt: now}
	if *q.LeaseMinutes > 0 {
		t := now.Add(time.Duration(*q.LeaseMinutes) * time.Minute)
		v.ExpiresAt = &t
	}
	mc, cancel := context.WithCancel(context.Background())
	m := &mapping{v: v, b: b, cancel: cancel, done: make(chan struct{}), deviceReady: make(chan struct{}), relayPort: rp}
	dir, e := PrivateDir()
	if e != nil {
		cancel()
		return Created{}, e
	}
	cert, e := Certificate(dir)
	if e != nil {
		os.RemoveAll(dir)
		cancel()
		return Created{}, e
	}
	listen := net.JoinHostPort(s.cfg.BindHost, strconv.Itoa(port))
	registration := ""
	if q.Kind == "serial" {
		l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", "0"))
		if err != nil {
			os.RemoveAll(dir)
			cancel()
			return Created{}, err
		}
		listen = l.Addr().String()
		l.Close()
		gate, err := serialauth.New(serialauth.Config{Backend: listen, Token: token})
		if err != nil {
			os.RemoveAll(dir)
			cancel()
			return Created{}, err
		}
		m.gate = gate
		m.listener, e = net.Listen("tcp", net.JoinHostPort(s.cfg.BindHost, strconv.Itoa(port)))
		if e != nil {
			os.RemoveAll(dir)
			cancel()
			return Created{}, e
		}
		registration = "AUTH " + token + "\r\n"
	}
	config := map[string]any{"log": map[string]any{"level": "info"}, "services": []any{map[string]any{"name": id, "addr": net.JoinHostPort(s.cfg.BindHost, strconv.Itoa(rp)), "listener": map[string]any{"type": "tls", "tls": map[string]any{"certFile": filepath.Join(dir, "cert.pem"), "keyFile": filepath.Join(dir, "key.pem")}}, "handler": map[string]any{"type": "relay", "auth": map[string]any{"username": id, "password": secret}, "metadata": map[string]any{"bind": true, "udp.bufferSize": "65535"}}}}}
	proc, e := StartGOST(mc, s.cfg.Binary, config, dir, func(row map[string]any) bool {
		return boundEndpoint(row, listen)
	})
	if e != nil {
		os.RemoveAll(dir)
		if m.listener != nil {
			m.listener.Close()
		}
		cancel()
		return Created{}, e
	}
	m.process = proc
	m.command = Command{ID: id, SessionID: b.ID, Op: "create", Request: q, Relay: net.JoinHostPort(s.cfg.Host, strconv.Itoa(rp)), Secret: secret, Certificate: cert, Listen: listen}
	s.items[id] = m
	committed = true
	go s.run(m, mc)
	return Created{Mapping: v, Registration: registration}, nil
}
func (s *Service) run(m *mapping, ctx context.Context) {
	reason := "closed_by_user"
	defer func() {
		m.cancel()
		if m.listener != nil {
			m.listener.Close()
		}
		if m.gate != nil {
			m.gate.Close()
		}
		m.process.Close()
		m.b.Enqueue(context.Background(), Command{ID: m.v.ID, SessionID: m.b.ID, Op: "close"})
		s.mu.Lock()
		defer s.mu.Unlock()
		now := time.Now().UTC()
		m.v.ClosedAt = &now
		t := now.Add(24 * time.Hour)
		m.v.ReusableAfter = &t
		m.v.Released = true
		if m.v.State != "failed" {
			m.v.State = "closed"
			m.v.Reason = reason
		}
		s.ports[m.v.Port] = t
		s.ports[m.relayPort] = t
		if e := s.savePorts(); e != nil {
			m.v.Reason += "; port_state_write_failed"
		}
		close(m.done)
		s.prune()
	}()
	if e := m.b.Enqueue(ctx, m.command); e != nil {
		s.fail(m, "device_dispatch_failed")
		return
	}
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	ready := m.process.Ready
	device := m.deviceReady
	for ready != nil || device != nil {
		select {
		case <-ctx.Done():
			return
		case <-m.b.Done:
			reason = "device_session_ended"
			return
		case <-m.process.Done:
			s.fail(m, "relay_exited")
			return
		case <-timer.C:
			s.fail(m, "create_timeout")
			return
		case <-ready:
			ready = nil
		case <-device:
			device = nil
		}
	}
	s.mu.Lock()
	if ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	m.v.State = "active"
	m.command.Secret = ""
	m.command.Certificate = ""
	s.mu.Unlock()
	if m.gate != nil {
		go m.gate.Serve(ctx, m.listener)
	}
	var expiry <-chan time.Time
	var et *time.Timer
	if m.v.ExpiresAt != nil {
		et = time.NewTimer(time.Until(*m.v.ExpiresAt))
		defer et.Stop()
		expiry = et.C
	}
	select {
	case <-ctx.Done():
	case <-m.b.Done:
		reason = "device_session_ended"
	case <-expiry:
		reason = "expired"
	case <-m.process.Done:
		s.fail(m, "relay_exited")
	}
}
func (s *Service) fail(m *mapping, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m.v.State = "failed"
	m.v.Reason = reason
}
func (s *Service) Report(session string, r Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.SessionID != session {
		return ErrInvalid
	}
	if r.State == "helper_failed" {
		for _, m := range s.items {
			if m.b.ID == session && !m.v.Released {
				m.v.State = "failed"
				m.v.Reason = "device_helper_exited"
				m.cancel()
			}
		}
		return nil
	}
	if q, ok := s.queries[r.ID]; ok && q.session == session {
		select {
		case q.result <- r:
		default:
		}
		return nil
	}
	m := s.items[r.ID]
	if m == nil || m.b.ID != session {
		return nil
	}
	switch r.State {
	case "running":
		m.v.SourceIP = r.SourceIP
		m.readyOnce.Do(func() { close(m.deviceReady) })
	case "closed":
		m.v.DeviceReleased = true
		if !m.v.Released {
			m.cancel()
		}
	case "failed":
		if !m.v.Released {
			m.v.State = "failed"
			m.v.Reason = r.Reason
			m.cancel()
		}
	}
	return nil
}
func (s *Service) prune() {
	if len(s.items) <= 160 {
		return
	}
	var all []*mapping
	for _, m := range s.items {
		if m.v.Released {
			all = append(all, m)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].v.CreatedAt.Before(all[j].v.CreatedAt) })
	for _, m := range all {
		if len(s.items) <= 160 {
			break
		}
		delete(s.items, m.v.ID)
	}
}
func (s *Service) List() []Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Snapshot, 0, len(s.items))
	for _, m := range s.items {
		v := m.v
		if m.gate != nil {
			g := m.gate.Snapshot()
			v.SerialAuth = &g
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}
func (s *Service) Get(id string) (Snapshot, error) {
	for _, v := range s.List() {
		if v.ID == id {
			return v, nil
		}
	}
	return Snapshot{}, ErrNotFound
}
func (s *Service) CloseMapping(id string) error {
	s.mu.Lock()
	m := s.items[id]
	s.mu.Unlock()
	if m == nil {
		return ErrNotFound
	}
	m.cancel()
	<-m.done
	return nil
}
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	var all []*mapping
	for _, m := range s.items {
		all = append(all, m)
		m.cancel()
	}
	s.mu.Unlock()
	for _, m := range all {
		<-m.done
	}
}

func boundEndpoint(row map[string]any, listen string) bool {
	msg, _ := row["msg"].(string)
	bound, _ := row["bind"].(string)
	if !strings.HasPrefix(msg, "bind on ") || !strings.HasSuffix(msg, " OK") {
		return false
	}
	bound = strings.TrimSuffix(strings.TrimSuffix(bound, "/tcp"), "/udp")
	host, port, e := net.SplitHostPort(bound)
	want, wp, we := net.SplitHostPort(listen)
	if e != nil || we != nil || port != wp {
		return false
	}
	ip, wip := net.ParseIP(host), net.ParseIP(want)
	return ip != nil && wip != nil && (ip.Equal(wip) || (ip.IsUnspecified() && wip.IsUnspecified()))
}
