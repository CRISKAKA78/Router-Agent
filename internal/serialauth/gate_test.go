package serialauth

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const tokenA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
const tokenB = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

type fixture struct {
	g        *Gate
	addr     string
	cancel   context.CancelFunc
	done     chan error
	accepted atomic.Int64
	mu       sync.Mutex
	data     []byte
	backend  net.Listener
	peers    map[net.Conn]bool
}

func setup(t *testing.T, change func(*Config)) *fixture {
	t.Helper()
	back, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	f := &fixture{backend: back, done: make(chan error, 1), peers: map[net.Conn]bool{}}
	go func() {
		for {
			c, e := back.Accept()
			if e != nil {
				return
			}
			f.accepted.Add(1)
			f.mu.Lock()
			f.peers[c] = true
			f.mu.Unlock()
			go func() {
				defer func() { c.Close(); f.mu.Lock(); delete(f.peers, c); f.mu.Unlock() }()
				b := make([]byte, 4096)
				for {
					n, e := c.Read(b)
					if n > 0 {
						f.mu.Lock()
						f.data = append(f.data, b[:n]...)
						f.mu.Unlock()
						if _, w := c.Write(b[:n]); w != nil {
							return
						}
					}
					if e != nil {
						return
					}
				}
			}()
		}
	}()
	cfg := Config{Backend: back.Addr().String(), Token: tokenA, HandshakeTimeout: 300 * time.Millisecond, AdmissionsPerSecond: 1000}
	if change != nil {
		change(&cfg)
	}
	f.g, e = New(cfg)
	if e != nil {
		back.Close()
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	f.addr = l.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	go func() { f.done <- f.g.Serve(ctx, l) }()
	t.Cleanup(func() {
		cancel()
		f.g.Close()
		select {
		case e := <-f.done:
			if e != nil {
				t.Error(e)
			}
		case <-time.After(2 * time.Second):
			t.Error("Serve did not finish")
		}
		back.Close()
		f.mu.Lock()
		for c := range f.peers {
			c.Close()
		}
		f.mu.Unlock()
	})
	return f
}
func dial(t *testing.T, f *fixture) net.Conn {
	t.Helper()
	c, e := net.DialTimeout("tcp", f.addr, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	c.SetDeadline(time.Now().Add(2 * time.Second))
	t.Cleanup(func() { c.Close() })
	return c
}
func line(t *testing.T, c net.Conn) string {
	t.Helper()
	var b []byte
	one := make([]byte, 1)
	for {
		n, e := c.Read(one)
		if e != nil {
			t.Fatalf("line %q: %v", b, e)
		}
		if n > 0 {
			b = append(b, one[0])
			if one[0] == '\n' {
				return string(b)
			}
		}
	}
}
func auth(t *testing.T, f *fixture, token string) net.Conn {
	t.Helper()
	c := dial(t, f)
	io.WriteString(c, "AUTH "+token+"\r\n")
	if s := line(t, c); s != "OK\r\n" {
		t.Fatalf("auth: %q", s)
	}
	return c
}
func echo(t *testing.T, c net.Conn, p []byte) {
	t.Helper()
	c.SetDeadline(time.Now().Add(time.Second))
	if _, e := c.Write(p); e != nil {
		t.Fatal(e)
	}
	b := make([]byte, len(p))
	if _, e := io.ReadFull(c, b); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(p, b) {
		t.Fatalf("mismatch %x != %x", b, p)
	}
}
func eventually(t *testing.T, fn func() bool) {
	t.Helper()
	end := time.Now().Add(time.Second)
	for time.Now().Before(end) {
		if fn() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition did not become true")
}
func closed(c net.Conn) bool {
	c.SetReadDeadline(time.Now().Add(time.Second))
	b := make([]byte, 256)
	_, e := c.Read(b)
	return e != nil && !isTimeout(e)
}
func isTimeout(e error) bool { var n net.Error; return errors.As(e, &n) && n.Timeout() }

func TestRegistrationBinaryAndPipelining(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n"} {
		t.Run(strings.TrimSpace(ending)+string(rune(len(ending)+'0')), func(t *testing.T) {
			f := setup(t, nil)
			c := dial(t, f)
			payload := bytes.Repeat([]byte{0, 255, 10, 13, 'A'}, 500)
			p := append([]byte("AUTH "+tokenA+ending), payload...)
			c.Write(p)
			if got := line(t, c); got != "OK\r\n" {
				t.Fatal(got)
			}
			got := make([]byte, len(payload))
			if _, e := io.ReadFull(c, got); e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(got, payload) {
				t.Fatal("pipeline changed")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if !bytes.Equal(f.data, payload) {
				t.Fatal("registration leaked to backend")
			}
		})
	}
}
func TestFragmentedAuthDoesNotOpenBackend(t *testing.T) {
	f := setup(t, nil)
	c := dial(t, f)
	io.WriteString(c, "AUTH "+tokenA[:20])
	time.Sleep(30 * time.Millisecond)
	if f.accepted.Load() != 0 || f.g.Snapshot().Owned {
		t.Fatal("unauthenticated backend use")
	}
	io.WriteString(c, tokenA[20:]+"\n")
	if s := line(t, c); s != "OK\r\n" {
		t.Fatal(s)
	}
	echo(t, c, []byte("fragmented"))
}
func TestInvalidAuthNeverDials(t *testing.T) {
	for _, s := range []string{"GET / HTTP/1.0\r\n", "AUTH wrong\n", "AUTH " + tokenB + "\n", " AUTH " + tokenA + "\n", "AUTH " + strings.ToUpper(tokenA) + "\n", "AUTH " + tokenA + " \n", "\n", strings.Repeat("x", MaxRegistrationBytes+1) + "\n"} {
		t.Run(s[:min(len(s), 12)], func(t *testing.T) {
			f := setup(t, nil)
			c := dial(t, f)
			io.WriteString(c, s+"junk-after-auth")
			if got := line(t, c); got != "ERR AUTH\r\n" {
				t.Fatal(got)
			}
			if !closed(c) {
				t.Fatal("bad auth left open")
			}
			if f.accepted.Load() != 0 {
				t.Fatal("bad auth dialed")
			}
		})
	}
}
func TestIdleAndTrickleHaveAbsoluteDeadline(t *testing.T) {
	for _, trickle := range []bool{false, true} {
		t.Run(map[bool]string{true: "trickle", false: "idle"}[trickle], func(t *testing.T) {
			f := setup(t, func(c *Config) { c.HandshakeTimeout = 90 * time.Millisecond })
			c := dial(t, f)
			start := time.Now()
			if trickle {
				go func() {
					for i := 0; i < 20; i++ {
						if _, e := c.Write([]byte{'A'}); e != nil {
							return
						}
						time.Sleep(20 * time.Millisecond)
					}
				}()
			}
			if got := line(t, c); got != "ERR AUTH\r\n" {
				t.Fatal(got)
			}
			if time.Since(start) > 500*time.Millisecond || f.accepted.Load() != 0 {
				t.Fatal("deadline/backend")
			}
		})
	}
}
func TestExclusiveBusyDoesNotBreakReaccept(t *testing.T) {
	f := setup(t, nil)
	idle := dial(t, f)
	owner := auth(t, f, tokenA)
	other := dial(t, f)
	io.WriteString(other, "AUTH "+tokenA+"\nINTRUDER")
	if got := line(t, other); got != "ERR BUSY\r\n" {
		t.Fatal(got)
	}
	echo(t, owner, []byte("owner"))
	if f.accepted.Load() != 1 {
		t.Fatal("multiple backends")
	}
	idle.Close()
	owner.Close()
	eventually(t, func() bool { return !f.g.Snapshot().Owned })
	next := auth(t, f, tokenA)
	echo(t, next, []byte("next"))
	eventually(t, func() bool { return f.accepted.Load() == 2 })
	f.mu.Lock()
	defer f.mu.Unlock()
	if bytes.Contains(f.data, []byte("INTRUDER")) {
		t.Fatal("intruder leaked")
	}
}
func TestConcurrentAuthenticatedConnections(t *testing.T) {
	f := setup(t, func(c *Config) { c.MaxPending = 32; c.MaxPendingPerIP = 32 })
	var wg sync.WaitGroup
	var ok, busy atomic.Int64
	start := make(chan struct{})
	release := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, e := net.Dial("tcp", f.addr)
			if e != nil {
				t.Error(e)
				ready.Done()
				return
			}
			defer c.Close()
			<-start
			c.SetDeadline(time.Now().Add(time.Second))
			io.WriteString(c, "AUTH "+tokenA+"\n")
			s, e := bufio.NewReader(c).ReadString('\n')
			if e != nil {
				t.Error(e)
			}
			switch s {
			case "OK\r\n":
				ok.Add(1)
			case "ERR BUSY\r\n":
				busy.Add(1)
			default:
				t.Errorf("unexpected %q", s)
			}
			ready.Done()
			<-release
		}()
	}
	close(start)
	ready.Wait()
	close(release)
	wg.Wait()
	if ok.Load() != 1 || busy.Load() != 11 {
		t.Fatalf("owners=%d busy=%d", ok.Load(), busy.Load())
	}
}
func TestRotateRevokesActiveAndPending(t *testing.T) {
	f := setup(t, nil)
	owner := auth(t, f, tokenA)
	pending := dial(t, f)
	io.WriteString(pending, "AUTH "+tokenA[:10])
	eventually(t, func() bool { return f.g.Snapshot().Pending == 1 })
	if e := f.g.RotateToken(tokenB); e != nil {
		t.Fatal(e)
	}
	if !closed(owner) || !closed(pending) {
		t.Fatal("rotation left connections")
	}
	eventually(t, func() bool { return !f.g.Snapshot().Owned })
	bad := dial(t, f)
	io.WriteString(bad, "AUTH "+tokenA+"\n")
	if s := line(t, bad); s != "ERR AUTH\r\n" {
		t.Fatal(s)
	}
	good := auth(t, f, tokenB)
	echo(t, good, []byte("new-token"))
	if strings.Contains(f.g.String(), tokenA) {
		t.Fatal("secret string")
	}
}
func TestCancelPendingDial(t *testing.T) {
	f := setup(t, nil)
	started := make(chan struct{})
	f.g.dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	c := dial(t, f)
	io.WriteString(c, "AUTH "+tokenA+"\nPAYLOAD")
	<-started
	f.cancel()
	eventually(t, func() bool { return f.g.Snapshot().Closed })
	if !closed(c) {
		t.Fatal("cancel did not close")
	}
	if f.accepted.Load() != 0 {
		t.Fatal("unexpected target")
	}
}
func TestRotateDuringDialCannotReturnOK(t *testing.T) {
	f := setup(t, nil)
	started, release := make(chan struct{}), make(chan struct{})
	server, peer := net.Pipe()
	defer peer.Close()
	f.g.dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
		close(started)
		<-release
		return server, nil
	}
	c := dial(t, f)
	io.WriteString(c, "AUTH "+tokenA+"\n")
	<-started
	if e := f.g.RotateToken(tokenB); e != nil {
		t.Fatal(e)
	}
	close(release)
	if !closed(c) {
		t.Fatal("old dial survived")
	}
	peer.SetReadDeadline(time.Now().Add(time.Second))
	if _, e := peer.Read(make([]byte, 1)); e != io.EOF {
		t.Fatalf("late backend not closed: %v", e)
	}
}
func TestLeaseExpiryClosesActiveAndRejectsPending(t *testing.T) {
	f := setup(t, func(c *Config) {
		c.ExpiresAt = time.Now().Add(120 * time.Millisecond)
		c.HandshakeTimeout = time.Second
	})
	owner := auth(t, f, tokenA)
	pending := dial(t, f)
	io.WriteString(pending, "AUTH ")
	eventually(t, func() bool { return f.g.Snapshot().Closed })
	if !closed(owner) || !closed(pending) {
		t.Fatal("expired sockets open")
	}
	if f.accepted.Load() != 1 {
		t.Fatal("expired pending dialed")
	}
}
func TestCloseInterruptsSlowPeer(t *testing.T) {
	f := setup(t, nil)
	c := auth(t, f, tokenA)
	stop := make(chan struct{})
	go func() {
		defer close(stop)
		b := make([]byte, 1<<20)
		for {
			if _, e := c.Write(b); e != nil {
				return
			}
		}
	}()
	time.Sleep(30 * time.Millisecond)
	done := make(chan struct{})
	go func() { f.g.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close blocked")
	}
	<-stop
}
func TestAdmissionLimits(t *testing.T) {
	for _, mode := range []string{"per-ip", "global", "rate"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t, func(c *Config) {
				c.HandshakeTimeout = time.Second
				switch mode {
				case "per-ip":
					c.MaxPendingPerIP = 1
				case "global":
					c.MaxPending = 1
				case "rate":
					c.AdmissionsPerSecond = 1
				}
			})
			first := dial(t, f)
			defer first.Close()
			eventually(t, func() bool { return f.g.Snapshot().Pending == 1 })
			second := dial(t, f)
			if !closed(second) {
				t.Fatal("excess admission not closed")
			}
			if f.accepted.Load() != 0 {
				t.Fatal("pending used backend")
			}
		})
	}
}
func TestBackendFailureDoesNotAcknowledgeOrLeak(t *testing.T) {
	f := setup(t, nil)
	f.g.dial = func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("synthetic backend failure")
	}
	c := dial(t, f)
	io.WriteString(c, "AUTH "+tokenA+"\nPAYLOAD")
	if s := line(t, c); s != "ERR BACKEND\r\n" {
		t.Fatal(s)
	}
	eventually(t, func() bool { return !f.g.Snapshot().Owned })
}
func TestConfigAndSecrets(t *testing.T) {
	token, e := NewToken()
	if e != nil || len(token) != 64 {
		t.Fatal("token generation")
	}
	for _, target := range []string{"example.com:1", "0.0.0.0:1", "192.168.1.1:1", "127.0.0.1:0", "127.0.0.1:65536"} {
		if _, e := New(Config{Backend: target, Token: token}); e == nil {
			t.Fatal(target)
		}
	}
	g, e := New(Config{Backend: "127.0.0.1:1", Token: token})
	if e != nil {
		t.Fatal(e)
	}
	if g.cfg.Token != "" {
		t.Fatal("stored plaintext token")
	}
	g.Close()
	if e := g.RotateToken(token); !errors.Is(e, net.ErrClosed) {
		t.Fatal(e)
	}
}
