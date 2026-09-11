package forwarding

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRequestValidation(t *testing.T) {
	q := Request{DeviceID: "d", Kind: "lan", Protocol: "udp", Interface: "eth0", TargetIP: "192.168.1.2", TargetPort: 123}
	if e := q.Normalize(); e != nil || *q.LeaseMinutes != 240 {
		t.Fatal(q, e)
	}
	zero := 0
	q.LeaseMinutes = &zero
	if q.Normalize() != nil {
		t.Fatal("zero lease")
	}
	for _, ip := range []string{"127.0.0.1", "224.0.0.1", "0.0.0.0", "::1", "garbage"} {
		q.TargetIP = ip
		if q.Normalize() == nil {
			t.Fatal(ip)
		}
	}
	q = Request{DeviceID: "d", Kind: "serial", Protocol: "udp", Serial: "/dev/ttyUSB0"}
	if q.Normalize() == nil {
		t.Fatal("serial UDP accepted")
	}
	q.Protocol = "tcp"
	if e := q.Normalize(); e != nil {
		t.Fatal(e)
	}
	q.StopBits = 2
	if q.Normalize() == nil {
		t.Fatal("unsupported GOST stopbits accepted")
	}
}

type testControl struct {
	mu             sync.Mutex
	s              *Service
	done           chan struct{}
	target, binary string
	children       map[string]*Process
}

func (c *testControl) BindForwarding(string) (Binding, error) {
	return Binding{ID: "test-session", Done: c.done, Enqueue: func(ctx context.Context, q Command) error {
		if q.Op == "close" {
			c.mu.Lock()
			p := c.children[q.ID]
			c.mu.Unlock()
			if p != nil {
				p.Close()
			}
			c.s.Report("test-session", Status{ID: q.ID, SessionID: "test-session", State: "closed"})
			return nil
		}
		if q.Op == "inventory" {
			return c.s.Report("test-session", Status{ID: q.ID, SessionID: "test-session", Inventory: &Inventory{Backend: true, Serials: []string{"test-uart"}}})
		}
		dir, e := PrivateDir()
		if e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(dir, "ca.pem"), []byte(q.Certificate), 0600); e != nil {
			return e
		}
		cfg := map[string]any{"log": map[string]any{"level": "info"}, "chains": []any{map[string]any{"name": "r", "hops": []any{map[string]any{"name": "h", "nodes": []any{map[string]any{"name": "n", "addr": q.Relay, "connector": map[string]any{"type": "relay", "auth": map[string]any{"username": q.ID, "password": q.Secret}}, "dialer": map[string]any{"type": "tls", "tls": map[string]any{"caFile": filepath.Join(dir, "ca.pem"), "secure": true, "serverName": "router-forwarding"}}}}}}}}, "services": []any{map[string]any{"name": "test", "addr": q.Listen, "listener": map[string]any{"type": "r" + q.Request.Protocol, "chain": "r", "metadata": map[string]any{"readBufferSize": "65535"}}, "handler": map[string]any{"type": "r" + q.Request.Protocol}, "forwarder": map[string]any{"nodes": []any{map[string]any{"name": "t", "addr": c.target}}}}}}
		p, e := StartGOST(ctx, c.binary, cfg, dir, nil)
		if e != nil {
			return e
		}
		c.mu.Lock()
		c.children[q.ID] = p
		c.mu.Unlock()
		return c.s.Report("test-session", Status{ID: q.ID, SessionID: "test-session", State: "running", SourceIP: "192.168.1.1"})
	}}, nil
}
func TestGOSTLifecycle(t *testing.T) {
	binary := os.Getenv("RMP_FORWARDING_GOST")
	if binary == "" {
		t.Skip("set RMP_FORWARDING_GOST to the pinned patched executable")
	}
	binary, _ = filepath.Abs(binary)
	kinds := []string{"tcp", "udp", "serial"}
	if os.Getenv("RMP_FORWARDING_LONG_TESTS") == "1" {
		kinds = append(kinds, "expiry")
	}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			var target string
			if kind == "udp" {
				u, e := net.ListenPacket("udp", "127.0.0.1:0")
				if e != nil {
					t.Fatal(e)
				}
				defer u.Close()
				target = u.LocalAddr().String()
				go func() {
					b := make([]byte, 65536)
					for {
						n, a, e := u.ReadFrom(b)
						if e != nil {
							return
						}
						u.WriteTo(b[:n], a)
					}
				}()
			} else {
				l, e := net.Listen("tcp", "127.0.0.1:0")
				if e != nil {
					t.Fatal(e)
				}
				defer l.Close()
				target = l.Addr().String()
				go func() {
					for {
						c, e := l.Accept()
						if e != nil {
							return
						}
						go func() { defer c.Close(); io.Copy(c, c) }()
					}
				}()
			}
			c := &testControl{done: make(chan struct{}), target: target, binary: binary, children: map[string]*Process{}}
			cfg := Config{Binary: binary, BindHost: "127.0.0.1", Host: "127.0.0.1", PortFirst: 35100, PortLast: 35199, StateFile: filepath.Join(t.TempDir(), "ports.json")}
			s, e := New(cfg, c)
			if e != nil {
				t.Fatal(e)
			}
			c.s = s
			defer s.Close()
			z := 0
			q := Request{DeviceID: "test", Kind: "lan", Protocol: kind, Interface: "eth0", TargetIP: "192.168.1.2", TargetPort: 123, LeaseMinutes: &z}
			if kind == "expiry" {
				q.Protocol = "tcp"
				minutes := 1
				q.LeaseMinutes = &minutes
			}
			if kind == "serial" {
				q.Kind = "serial"
				q.Protocol = "tcp"
				q.Serial = "test-uart"
			}
			v, e := s.Create(context.Background(), q)
			if e != nil {
				t.Fatal(e)
			}
			end := time.Now().Add(10 * time.Second)
			for {
				v.Mapping, _ = s.Get(v.Mapping.ID)
				if v.Mapping.State == "active" {
					break
				}
				if v.Mapping.State == "failed" || time.Now().After(end) {
					t.Fatalf("not active: %+v", v.Mapping)
				}
				time.Sleep(30 * time.Millisecond)
			}
			b, _ := json.Marshal(s.List())
			if strings.Contains(string(b), "AUTH ") || strings.Contains(string(b), "certificate") {
				t.Fatal("private auth leaked")
			}
			addr := net.JoinHostPort(v.Mapping.Host, strconv.Itoa(v.Mapping.Port))
			n, e := net.Dial(q.Protocol, addr)
			if e != nil {
				t.Fatal(e)
			}
			defer n.Close()
			n.SetDeadline(time.Now().Add(5 * time.Second))
			reader := bufio.NewReader(n)
			if kind == "serial" {
				if !strings.HasSuffix(v.Registration, "\r\n") {
					t.Fatal("registration is not actual CRLF")
				}
				io.WriteString(n, v.Registration)
				line, e := reader.ReadString('\n')
				if e != nil || line != "OK\r\n" {
					t.Fatal(line, e)
				}
			}
			payload := []byte("binary\x00\xffroundtrip")
			if _, e = n.Write(payload); e != nil {
				t.Fatal(e)
			}
			got := make([]byte, len(payload))
			if _, e = io.ReadFull(reader, got); e != nil || string(got) != string(payload) {
				t.Fatal(string(got), e)
			}
			if kind == "expiry" {
				deadline := time.Now().Add(65 * time.Second)
				for {
					expired, _ := s.Get(v.Mapping.ID)
					if expired.Released {
						if expired.Reason != "expired" {
							t.Fatal(expired)
						}
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("lease did not expire")
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
			if kind == "tcp" {
				close(c.done)
			}
			if e = s.CloseMapping(v.Mapping.ID); e != nil {
				t.Fatal(e)
			}
			closed, _ := s.Get(v.Mapping.ID)
			if !closed.Released || closed.ReusableAfter == nil {
				t.Fatal(closed)
			}
			restarted, e := New(cfg, c)
			if e != nil {
				t.Fatal(e)
			}
			if !time.Now().Before(restarted.ports[closed.Port]) {
				t.Fatal("port quarantine lost on restart")
			}
			restarted.Close()
		})
	}
}

func TestBoundEndpointReadiness(t *testing.T) {
	for _, actual := range []string{"0.0.0.0:22000", "[::]:22000", "[::]:22000/tcp"} {
		if !boundEndpoint(map[string]any{"bind": actual, "msg": "bind on " + actual + "/tcp OK"}, "0.0.0.0:22000") {
			t.Fatal(actual)
		}
	}
	if boundEndpoint(map[string]any{"bind": "[::]:22000", "msg": "bind on [::]:22000 OK"}, "127.0.0.1:22000") {
		t.Fatal("serial backend matched wildcard")
	}
}

func TestCrashReservationsBecomeQuarantine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ports.json")
	now := time.Now()
	existing := now.Add(time.Hour)
	data, _ := json.Marshal(map[int]time.Time{22000: now.AddDate(100, 0, 0), 22001: existing})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{StateFile: path}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if until := s.ports[22000]; until.Before(now.Add(24*time.Hour)) || until.After(now.Add(24*time.Hour+time.Minute)) {
		t.Fatal(until)
	}
	if !s.ports[22001].Equal(existing) {
		t.Fatal("existing quarantine changed")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[int]time.Time
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if !saved[22000].Equal(s.ports[22000]) {
		t.Fatal("quarantine not persisted")
	}
}
