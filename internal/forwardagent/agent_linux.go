// Package forwardagent is the optional Linux sidecar owned by one Probe control session.
package forwardagent

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"routerprobe/internal/forwarding"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type child struct {
	process *forwarding.Process
	cancel  context.CancelFunc
	request forwarding.Request
	lock    *os.File
}
type Agent struct {
	mu              sync.Mutex
	session, binary string
	children        map[string]*child
	report          func(forwarding.Status)
	closed          bool
	wg              sync.WaitGroup
}

func New(session, binary string, report func(forwarding.Status)) *Agent {
	return &Agent{session: session, binary: binary, report: report, children: map[string]*child{}}
}
func Inventory(binary string) forwarding.Inventory {
	v := forwarding.Inventory{Backend: forwarding.BackendAvailable(binary), Interfaces: []forwarding.Interface{}, Serials: []string{}}
	interfaces, _ := net.Interfaces()
	for _, i := range interfaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		aa, _ := i.Addrs()
		for _, a := range aa {
			ip, _, e := net.ParseCIDR(a.String())
			if e == nil && ip.To4() != nil && ip.IsGlobalUnicast() {
				v.Interfaces = append(v.Interfaces, forwarding.Interface{Name: i.Name, Address: a.String()})
			}
		}
	}
	console, _ := os.ReadFile("/proc/consoles")
	exclude := map[string]bool{}
	for _, line := range strings.Split(string(console), "\n") {
		f := strings.Fields(line)
		if len(f) > 0 {
			exclude["/dev/"+f[0]] = true
		}
	}
	for _, pat := range []string{"/dev/ttyS*", "/dev/ttyUSB*", "/dev/ttyACM*", "/dev/ttyAMA*", "/dev/ttyAP*"} {
		names, _ := filepath.Glob(pat)
		for _, n := range names {
			st, e := os.Lstat(n)
			if e == nil && st.Mode()&os.ModeCharDevice != 0 && !exclude[n] {
				v.Serials = append(v.Serials, n)
			}
		}
	}
	sort.Strings(v.Serials)
	return v
}
func Source(q forwarding.Request, v forwarding.Inventory) (string, error) {
	target := net.ParseIP(q.TargetIP)
	for _, i := range v.Interfaces {
		if i.Name != q.Interface {
			continue
		}
		ip, n, e := net.ParseCIDR(i.Address)
		if e != nil || target == nil || !n.Contains(target) || ip.Equal(target) {
			continue
		}
		ones, bits := n.Mask.Size()
		if bits != 32 || ones < 1 {
			return "", errors.New("invalid_interface_subnet")
		}
		b := target.To4()
		broadcast := true
		for k := 0; k < 4; k++ {
			if b[k] != (n.IP.To4()[k] | ^n.Mask[k]) {
				broadcast = false
			}
		}
		if ones < 31 && (target.Equal(n.IP) || broadcast) {
			return "", errors.New("not_unicast_target")
		}
		return ip.String(), nil
	}
	return "", errors.New("target_not_on_selected_interface")
}
func (a *Agent) Handle(c forwarding.Command) {
	a.mu.Lock()
	defer a.mu.Unlock()
	report := func(state, reason, source string) {
		a.report(forwarding.Status{ID: c.ID, SessionID: a.session, State: state, Reason: reason, SourceIP: source})
	}
	if a.closed || c.SessionID != a.session {
		return
	}
	if b, e := hex.DecodeString(c.ID); e != nil || len(b) != 32 {
		return
	}
	if c.Op == "inventory" {
		v := Inventory(a.binary)
		a.report(forwarding.Status{ID: c.ID, SessionID: a.session, State: "inventory", Inventory: &v})
		return
	}
	if c.Op == "close" {
		if x := a.children[c.ID]; x != nil {
			x.cancel()
		} else {
			report("closed", "", "")
		}
		return
	}
	if c.Op != "create" {
		report("failed", "unknown_operation", "")
		return
	}
	if _, ok := a.children[c.ID]; ok {
		return
	}
	if len(a.children) >= 8 {
		report("failed", "device_capacity", "")
		return
	}
	if e := c.Request.Normalize(); e != nil {
		report("failed", "invalid_request", "")
		return
	}
	q := c.Request
	v := Inventory(a.binary)
	if !v.Backend {
		report("failed", "gost_not_installed", "")
		return
	}
	source := ""
	var lock *os.File
	var e error
	if q.Kind == "lan" {
		source, e = Source(q, v)
		if e != nil {
			report("failed", e.Error(), "")
			return
		}
	} else {
		allowed := false
		for _, p := range v.Serials {
			if p == q.Serial {
				allowed = true
			}
		}
		if !allowed {
			report("failed", "serial_missing_or_console", "")
			return
		}
		lock, e = os.OpenFile(filepath.Join(os.TempDir(), "router-forwarding-"+filepath.Base(q.Serial)+".lock"), os.O_CREATE|os.O_RDWR, 0600)
		if e != nil {
			report("failed", "serial_lock_failed", "")
			return
		}
		if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
			lock.Close()
			report("failed", "serial_busy", "")
			return
		}
	}
	cleanupLock := func() {
		if lock != nil {
			lock.Close()
		}
	}
	if _, _, e = net.SplitHostPort(c.Relay); e != nil || len(c.Certificate) > 16384 || len(c.Secret) != 64 {
		cleanupLock()
		report("failed", "invalid_relay", "")
		return
	}
	if _, _, e = net.SplitHostPort(c.Listen); e != nil {
		cleanupLock()
		report("failed", "invalid_listen", "")
		return
	}
	dir, e := forwarding.PrivateDir()
	if e != nil {
		cleanupLock()
		report("failed", "config_directory_failed", "")
		return
	}
	if e = os.WriteFile(filepath.Join(dir, "ca.pem"), []byte(c.Certificate), 0600); e != nil {
		os.RemoveAll(dir)
		cleanupLock()
		report("failed", "certificate_write_failed", "")
		return
	}
	node := map[string]any{"name": "relay", "addr": c.Relay, "connector": map[string]any{"type": "relay", "auth": map[string]any{"username": c.ID, "password": c.Secret}}, "dialer": map[string]any{"type": "tls", "tls": map[string]any{"caFile": filepath.Join(dir, "ca.pem"), "secure": true, "serverName": "router-forwarding"}}}
	chains := []any{map[string]any{"name": "reverse", "hops": []any{map[string]any{"name": "hop", "nodes": []any{node}}}}}
	listener := map[string]any{"type": "r" + q.Protocol, "chain": "reverse", "metadata": map[string]any{"readBufferSize": "65535"}}
	handler := map[string]any{"type": "r" + q.Protocol}
	target := net.JoinHostPort(q.TargetIP, strconv.Itoa(q.TargetPort))
	if q.Kind == "serial" {
		addr := fmt.Sprintf("%s,%d,%s", q.Serial, q.Baud, q.Parity)
		chains = append(chains, map[string]any{"name": "serial", "hops": []any{map[string]any{"name": "uart", "nodes": []any{map[string]any{"name": "uart", "addr": addr, "connector": map[string]any{"type": "forward"}, "dialer": map[string]any{"type": "serial"}}}}}})
		handler["chain"] = "serial"
		target = "127.0.0.1:1"
	}
	service := map[string]any{"name": c.ID, "addr": c.Listen, "listener": listener, "handler": handler, "forwarder": map[string]any{"nodes": []any{map[string]any{"name": "target", "addr": target}}}}
	if q.Kind == "lan" {
		service["interface"] = source + "!"
	}
	cfg := map[string]any{"log": map[string]any{"level": "info"}, "chains": chains, "services": []any{service}}
	ctx, cancel := context.WithCancel(context.Background())
	p, e := forwarding.StartGOST(ctx, a.binary, cfg, dir, nil)
	if e != nil {
		cancel()
		cleanupLock()
		os.RemoveAll(dir)
		report("failed", "gost_start_failed", "")
		return
	}
	x := &child{process: p, cancel: cancel, request: q, lock: lock}
	a.children[c.ID] = x
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		state, reason := "closed", ""
		timer := time.NewTimer(200 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-p.Done:
			state, reason = "failed", "gost_exited"
		case <-timer.C:
			report("running", "", source)
		}
		var expiry <-chan time.Time
		var et *time.Timer
		if *q.LeaseMinutes > 0 {
			et = time.NewTimer(time.Duration(*q.LeaseMinutes) * time.Minute)
			defer et.Stop()
			expiry = et.C
		}
		if state != "failed" {
			select {
			case <-ctx.Done():
			case <-expiry:
				reason = "expired"
			case <-p.Done:
				state, reason = "failed", "gost_exited"
			}
		}
		cancel()
		p.Close()
		cleanupLock()
		a.mu.Lock()
		delete(a.children, c.ID)
		a.mu.Unlock()
		report(state, reason, source)
	}()
}
func (a *Agent) Close() {
	a.mu.Lock()
	a.closed = true
	for _, x := range a.children {
		x.cancel()
	}
	a.mu.Unlock()
	a.wg.Wait()
}
