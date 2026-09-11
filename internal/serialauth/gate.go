// Package serialauth gates a fixed internal serial TCP channel behind a bounded
// registration handshake. It does not implement UART drivers or Probe control.
package serialauth

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const MaxRegistrationBytes = 512

// Config is one serial mapping, not a general-purpose proxy. Backend must be a
// literal loopback TCP address hidden from external clients. Zero ExpiresAt means
// no absolute expiry; the owning service must still cancel Serve on Session loss.
type Config struct {
	Backend             string
	Token               string
	ExpiresAt           time.Time
	HandshakeTimeout    time.Duration
	DialTimeout         time.Duration
	MaxPending          int
	MaxPendingPerIP     int
	AdmissionsPerSecond int
}

type Snapshot struct {
	Closed         bool   `json:"closed"`
	Pending        int    `json:"pending"`
	Owned          bool   `json:"owned"`
	Accepted       uint64 `json:"accepted"`
	AuthFailures   uint64 `json:"auth_failures"`
	Busy           uint64 `json:"busy"`
	Rejected       uint64 `json:"rejected"`
	BytesToBackend int64  `json:"bytes_to_backend"`
	BytesToClient  int64  `json:"bytes_to_client"`
}

type client struct {
	conn    net.Conn
	backend net.Conn
	cancel  context.CancelFunc
	epoch   uint64
	ip      string
	pending bool
}

type Gate struct {
	mu        sync.Mutex
	cfg       Config
	digest    [32]byte
	epoch     uint64
	listener  net.Listener
	served    bool
	closed    bool
	done      chan struct{}
	clients   map[*client]struct{}
	owner     *client
	perIP     map[string]int
	pending   int
	budget    float64
	budgetAt  time.Time
	stats     Snapshot
	wg        sync.WaitGroup
	toBackend atomic.Int64
	toClient  atomic.Int64
	dial      func(context.Context, string, string) (net.Conn, error)
}

func NewToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func tokenDigest(token string) ([32]byte, error) {
	var zero [32]byte
	if len(token) != 64 || strings.ToLower(token) != token {
		return zero, errors.New("token must be 64 lowercase hexadecimal characters")
	}
	if _, err := hex.DecodeString(token); err != nil {
		return zero, errors.New("token must be 64 lowercase hexadecimal characters")
	}
	return sha256.Sum256([]byte(token)), nil
}
func New(cfg Config) (*Gate, error) {
	host, p, err := net.SplitHostPort(cfg.Backend)
	if err != nil {
		return nil, errors.New("backend must be a literal loopback TCP address")
	}
	ip := net.ParseIP(host)
	port, err := strconv.Atoi(p)
	if ip == nil || !ip.IsLoopback() || err != nil || port < 1 || port > 65535 {
		return nil, errors.New("backend must be a literal loopback TCP address with a valid port")
	}
	digest, err := tokenDigest(cfg.Token)
	if err != nil {
		return nil, err
	}
	cfg.Token = ""
	if cfg.HandshakeTimeout == 0 {
		cfg.HandshakeTimeout = 5 * time.Second
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	if cfg.MaxPending == 0 {
		cfg.MaxPending = 16
	}
	if cfg.MaxPendingPerIP == 0 {
		cfg.MaxPendingPerIP = 4
	}
	if cfg.AdmissionsPerSecond == 0 {
		cfg.AdmissionsPerSecond = 20
	}
	if cfg.HandshakeTimeout < 0 || cfg.DialTimeout < 0 || cfg.MaxPending < 1 || cfg.MaxPendingPerIP < 1 || cfg.AdmissionsPerSecond < 1 {
		return nil, errors.New("invalid timeouts or admission limits")
	}
	g := &Gate{cfg: cfg, digest: digest, done: make(chan struct{}), clients: make(map[*client]struct{}), perIP: make(map[string]int), budget: float64(cfg.AdmissionsPerSecond), budgetAt: time.Now()}
	g.dial = (&net.Dialer{}).DialContext
	return g, nil
}
func (g *Gate) expired() bool {
	return !g.cfg.ExpiresAt.IsZero() && !time.Now().Before(g.cfg.ExpiresAt)
}

// Serve owns l and all accepted sockets. It is single-use and blocks until all
// handlers have released their internal channels. Cancellation is fail-closed.
func (g *Gate) Serve(ctx context.Context, l net.Listener) error {
	if l == nil {
		return errors.New("nil listener")
	}
	g.mu.Lock()
	if g.served || g.closed {
		g.mu.Unlock()
		return errors.New("gate already served or closed")
	}
	g.served = true
	g.listener = l
	g.mu.Unlock()
	defer g.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if !g.cfg.ExpiresAt.IsZero() {
		var stop context.CancelFunc
		ctx, stop = context.WithDeadline(ctx, g.cfg.ExpiresAt)
		defer stop()
	}
	go func() {
		select {
		case <-ctx.Done():
			g.shutdown()
		case <-g.done:
		}
	}()
	for {
		conn, err := l.Accept()
		if err != nil {
			g.mu.Lock()
			closed := g.closed
			g.mu.Unlock()
			if closed || ctx.Err() != nil {
				return nil
			}
			return err
		}
		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		g.mu.Lock()
		now := time.Now()
		g.budget += now.Sub(g.budgetAt).Seconds() * float64(g.cfg.AdmissionsPerSecond)
		g.budgetAt = now
		if max := float64(g.cfg.AdmissionsPerSecond); g.budget > max {
			g.budget = max
		}
		if g.closed || g.expired() || ctx.Err() != nil || g.pending >= g.cfg.MaxPending || g.perIP[ip] >= g.cfg.MaxPendingPerIP || g.budget < 1 {
			g.stats.Rejected++
			g.mu.Unlock()
			conn.Close()
			continue
		}
		g.budget--
		g.pending++
		g.perIP[ip]++
		g.stats.Accepted++
		cctx, ccancel := context.WithCancel(ctx)
		c := &client{conn: conn, cancel: ccancel, epoch: g.epoch, ip: ip, pending: true}
		g.clients[c] = struct{}{}
		g.wg.Add(1)
		g.mu.Unlock()
		go func() { defer g.wg.Done(); g.handle(cctx, c) }()
	}
}
func (g *Gate) releasePending(c *client) {
	if !c.pending {
		return
	}
	c.pending = false
	g.pending--
	g.perIP[c.ip]--
	if g.perIP[c.ip] == 0 {
		delete(g.perIP, c.ip)
	}
}
func reply(c net.Conn, s string) error {
	c.SetWriteDeadline(time.Now().Add(time.Second))
	_, err := io.WriteString(c, s+"\r\n")
	c.SetWriteDeadline(time.Time{})
	return err
}
func (g *Gate) handle(ctx context.Context, c *client) {
	defer func() {
		c.cancel()
		c.conn.Close()
		// backend is assigned only by this handler; shutdown/rotation only close it.
		if c.backend != nil {
			c.backend.Close()
		}
		g.mu.Lock()
		g.releasePending(c)
		if g.owner == c {
			g.owner = nil
		}
		delete(g.clients, c)
		g.mu.Unlock()
	}()
	c.conn.SetReadDeadline(time.Now().Add(g.cfg.HandshakeTimeout))
	reader := bufio.NewReaderSize(c.conn, MaxRegistrationBytes)
	line, err := reader.ReadSlice('\n')
	if err != nil {
		g.authFailed()
		reply(c.conn, "ERR AUTH")
		return
	}
	text := strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r")
	token := strings.TrimPrefix(text, "AUTH ")
	digest, derr := tokenDigest(token)
	if len(line) > MaxRegistrationBytes || !strings.HasPrefix(text, "AUTH ") || derr != nil {
		g.authFailed()
		reply(c.conn, "ERR AUTH")
		return
	}
	g.mu.Lock()
	if g.closed || g.expired() || c.epoch != g.epoch || ctx.Err() != nil {
		g.mu.Unlock()
		reply(c.conn, "ERR CLOSED")
		return
	}
	if subtle.ConstantTimeCompare(digest[:], g.digest[:]) != 1 {
		g.stats.AuthFailures++
		g.mu.Unlock()
		reply(c.conn, "ERR AUTH")
		return
	}
	if g.owner != nil {
		g.stats.Busy++
		g.mu.Unlock()
		reply(c.conn, "ERR BUSY")
		return
	}
	g.owner = c
	g.releasePending(c)
	g.mu.Unlock()
	c.conn.SetReadDeadline(time.Time{})
	dctx, cancel := context.WithTimeout(ctx, g.cfg.DialTimeout)
	backend, err := g.dial(dctx, "tcp", g.cfg.Backend)
	cancel()
	if err != nil {
		if ctx.Err() == nil {
			reply(c.conn, "ERR BACKEND")
		}
		return
	}
	g.mu.Lock()
	if g.closed || g.expired() || c.epoch != g.epoch || ctx.Err() != nil {
		g.mu.Unlock()
		backend.Close()
		reply(c.conn, "ERR CLOSED")
		return
	}
	c.backend = backend
	g.mu.Unlock()
	// OK means this gate acquired its slot and connected the configured backend.
	// It cannot prove a remote GOST UART has opened without a backend-ready signal.
	if err := reply(c.conn, "OK"); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() { io.Copy(countWriter{backend, &g.toBackend}, reader); done <- struct{}{} }()
	go func() { io.Copy(countWriter{c.conn, &g.toClient}, backend); done <- struct{}{} }()
	<-done
	c.conn.Close()
	backend.Close()
	<-done
}
func (g *Gate) authFailed() { g.mu.Lock(); g.stats.AuthFailures++; g.mu.Unlock() }

type countWriter struct {
	io.Writer
	count *atomic.Int64
}

func (w countWriter) Write(p []byte) (int, error) {
	n, e := w.Writer.Write(p)
	w.count.Add(int64(n))
	return n, e
}

func (g *Gate) shutdown() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return
	}
	g.closed = true
	close(g.done)
	if g.listener != nil {
		g.listener.Close()
	}
	for c := range g.clients {
		c.cancel()
		c.conn.Close()
		if c.backend != nil {
			c.backend.Close()
		}
	}
}
func (g *Gate) Close() error { g.shutdown(); g.wg.Wait(); return nil }

// RotateToken revokes all old connections, including handshakes and dials. The
// old owner's slot stays reserved until its handler has fully released sockets.
func (g *Gate) RotateToken(token string) error {
	digest, err := tokenDigest(token)
	if err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed || g.expired() {
		return net.ErrClosed
	}
	g.digest = digest
	g.epoch++
	for c := range g.clients {
		c.cancel()
		c.conn.Close()
		if c.backend != nil {
			c.backend.Close()
		}
	}
	return nil
}
func (g *Gate) Snapshot() Snapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	s := g.stats
	s.Closed = g.closed
	s.Pending = g.pending
	s.Owned = g.owner != nil
	s.BytesToBackend = g.toBackend.Load()
	s.BytesToClient = g.toClient.Load()
	return s
}
func (g *Gate) String() string { return fmt.Sprintf("serialauth(%s)", g.cfg.Backend) }
