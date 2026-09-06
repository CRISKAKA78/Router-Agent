package integration

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"routerprobe/internal/filetransfer"
	"routerprobe/internal/gateway"
	"routerprobe/internal/task"
	"routerprobe/internal/tunnel"
)

type phase4SilentClose struct{ *gateway.Server }

func (g phase4SilentClose) BindTunnel(id string) (tunnel.Binding, error) {
	b, e := g.Server.BindTunnel(id)
	if e != nil {
		return b, e
	}
	enqueue := b.Enqueue
	b.Enqueue = func(ctx context.Context, closing bool, c tunnel.Command) error {
		if closing {
			return nil
		}
		return enqueue(ctx, closing, c)
	}
	return b, nil
}

func phase4Server(t *testing.T, c tunnel.Config, silentClose ...bool) (*gateway.Server, *tunnel.Service, string) {
	t.Helper()
	probeBinary(t)
	g, e := gateway.New(gateway.Config{HeartbeatInterval: 10 * time.Second, Logger: log.New(io.Discard, "", 0)})
	if e != nil {
		t.Fatal(e)
	}
	if c.DataListen == "" {
		c.DataListen = "127.0.0.1:0"
	}
	c.PortFirst = 26000
	c.PortLast = 26999
	var control tunnel.Control = g
	if len(silentClose) > 0 && silentClose[0] {
		control = phase4SilentClose{g}
	}
	s, e := tunnel.New(c, control)
	if e != nil {
		g.Close()
		t.Fatal(e)
	}
	g.SetTunnelStatus(s.Report)
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- g.Serve(l) }()
	t.Cleanup(func() {
		s.Close()
		g.Close()
		if e := <-done; e != nil {
			t.Error(e)
		}
	})
	return g, s, l.Addr().String()
}

func TestTunnelRealDefaultCapacityAndResetAfterHalfClose(t *testing.T) {
	binary := probeBinary(t)
	l, e := net.Listen("tcp", "127.0.0.1:80")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	accepted := make(chan net.Conn, 16)
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			accepted <- c
		}
	}()
	defer func() {
		l.Close()
		<-acceptDone
		close(accepted)
		for c := range accepted {
			c.Close()
		}
	}()
	g, s, address := phase4Server(t, tunnel.Config{PerMaintenance: 16, PerDevice: 16}, true)
	var logs lockedBuffer
	p := startProbe(t, binary, address, "small-router", &logs)
	waitOnline(t, g.Events(), "small-router", 5*time.Second)
	time.Sleep(100 * time.Millisecond)
	fds, threads := phase4Counts(p.Process.Pid)
	m := phase4Create(t, s, "small-router", 0)
	var clients, targets []net.Conn
	for i := 0; i < 8; i++ {
		c := phase4Dial(t, m.Endpoints[0])
		clients = append(clients, c)
		select {
		case target := <-accepted:
			targets = append(targets, target)
			t.Cleanup(func() { target.Close() })
		case <-time.After(time.Second):
			t.Fatal("missing local target")
		}
	}
	// Probe default is eight, even when Server explicitly permits more.
	extra := phase4Dial(t, m.Endpoints[0])
	phase4Closed(t, extra)
	_, activeThreads := phase4Counts(p.Process.Pid)
	if activeThreads > threads+8 {
		t.Fatal("unbounded Probe threads", threads, activeThreads)
	}
	clients[0].(*net.TCPConn).CloseWrite()
	targets[0].SetReadDeadline(time.Now().Add(time.Second))
	var one [1]byte
	if _, e := targets[0].Read(one[:]); e != io.EOF {
		t.Fatal("missing normal EOF", e)
	}
	// Keep all local applications open and idle. Drop every control CLOSE to
	// prove data reset alone releases Probe workers, including after read EOF.
	s.CloseMaintenance(m.ID)
	for _, c := range clients {
		phase4Closed(t, c)
	}
	until := time.Now().Add(3 * time.Second)
	for {
		f, n := phase4Counts(p.Process.Pid)
		if f <= fds && n <= threads {
			break
		}
		if time.Now().After(until) {
			t.Fatalf("reset left idle local workers: fd %d/%d threads %d/%d\n%s", f, fds, n, threads, logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	r := runExec(t, g, "small-router", task.ExecRequest{Command: "printf control-alive", Timeout: time.Second})
	if r.Stdout != "control-alive" {
		t.Fatal(r)
	}
}

func TestTunnelRealOneWayIdleAndBufferedEOF(t *testing.T) {
	binary := probeBinary(t)
	l, e := net.Listen("tcp", "127.0.0.1:80")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	done := make(chan error, 1)
	go func() {
		c, e := l.Accept()
		if e != nil {
			done <- e
			return
		}
		defer c.Close()
		if _, e = io.Copy(io.Discard, c); e != nil {
			done <- e
			return
		}
		for i := 0; i < 12; i++ {
			time.Sleep(100 * time.Millisecond)
			if _, e = c.Write([]byte("x")); e != nil {
				done <- e
				return
			}
		}
		_, e = io.CopyN(c, bytes.NewReader(bytes.Repeat([]byte("z"), 512*1024)), 512*1024)
		done <- e // normal full local close with unread/buffered bytes
	}()
	g, s, address := phase4Server(t, tunnel.Config{IdleTimeout: 500 * time.Millisecond})
	var logs lockedBuffer
	startProbe(t, binary, address, "one-way", &logs)
	waitOnline(t, g.Events(), "one-way", 5*time.Second)
	m := phase4Create(t, s, "one-way", 0)
	c := phase4Dial(t, m.Endpoints[0])
	c.(*net.TCPConn).CloseWrite()
	var got bytes.Buffer
	buffer := make([]byte, 8192)
	for {
		n, e := c.Read(buffer)
		got.Write(buffer[:n])
		if e != nil {
			if e != io.EOF {
				t.Fatal(e)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got.Len() != 12+512*1024 || !bytes.Equal(got.Bytes()[:12], bytes.Repeat([]byte("x"), 12)) || !bytes.Equal(got.Bytes()[12:], bytes.Repeat([]byte("z"), 512*1024)) {
		t.Fatal("EOF dropped buffered bytes", got.Len())
	}
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}
func phase4Create(t *testing.T, s *tunnel.Service, id string, lease time.Duration) tunnel.Snapshot {
	t.Helper()
	m, e := s.Create(context.Background(), id, lease)
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func phase4Dial(t *testing.T, e tunnel.Endpoint) net.Conn {
	t.Helper()
	c, err := net.DialTimeout("tcp", e.Address(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	c.SetDeadline(time.Now().Add(5 * time.Second))
	return c
}
func phase4Echo(t *testing.T, c net.Conn) {
	t.Helper()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	data := []byte{0, 255, 'w', 'e', 'b', '\r', '\n'}
	if _, e := c.Write(data); e != nil {
		t.Fatal(e)
	}
	b := make([]byte, len(data))
	if _, e := io.ReadFull(c, b); e != nil || !bytes.Equal(b, data) {
		t.Fatalf("echo %x %v", b, e)
	}
}
func phase4Closed(t *testing.T, c net.Conn) {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var b [65536]byte
	for {
		_, e := c.Read(b[:])
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Timeout() {
				t.Fatal("data connection not terminated")
			}
			return
		}
	}
}
func phase4Released(t *testing.T, s *tunnel.Service, id string) {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		m, e := s.Get(id)
		if e == nil && m.Released {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("maintenance not released")
}
func phase4EchoTarget(t *testing.T, port int) {
	t.Helper()
	l, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	cs := map[net.Conn]bool{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			mu.Lock()
			cs[c] = true
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer c.Close()
				defer func() { mu.Lock(); delete(cs, c); mu.Unlock() }()
				io.Copy(c, c)
				if tcp, ok := c.(*net.TCPConn); ok {
					tcp.CloseWrite()
				}
			}()
		}
	}()
	t.Cleanup(func() {
		l.Close()
		mu.Lock()
		for c := range cs {
			c.Close()
		}
		mu.Unlock()
		wg.Wait()
	})
}
func phase4Counts(pid int) (int, int) {
	fds, _ := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	threads, _ := os.ReadDir(fmt.Sprintf("/proc/%d/task", pid))
	return len(fds), len(threads)
}
func TestTunnelRealProbeLifecycleConcurrencyAndControlLoad(t *testing.T) {
	binary := probeBinary(t)
	for _, p := range []int{80, 22, 23} {
		phase4EchoTarget(t, p)
	}
	g, s, address := phase4Server(t, tunnel.Config{})
	var logs lockedBuffer
	a := startProbe(t, binary, address, "tunnel-a", &logs)
	bProcess := startProbe(t, binary, address, "tunnel-b", &logs)
	waitOnline(t, g.Events(), "tunnel-a", 5*time.Second)
	// Query inventory rather than consuming another device's online event.
	until := time.Now().Add(5 * time.Second)
	for {
		d, e := g.Devices().Get("tunnel-b")
		if e == nil && d.CurrentSession != nil {
			break
		}
		if time.Now().After(until) {
			t.Fatal("second device offline")
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	fds, threads := phase4Counts(a.Process.Pid)
	m := phase4Create(t, s, "tunnel-a", 0)
	if m.ExpiresAt.Sub(m.CreatedAt) != 240*time.Minute {
		t.Fatal(m)
	}
	b := phase4Create(t, s, "tunnel-b", time.Minute)
	var cs []net.Conn
	for i := 0; i < 6; i++ {
		c := phase4Dial(t, m.Endpoints[i%3])
		phase4Echo(t, c)
		cs = append(cs, c)
	}
	bconn := phase4Dial(t, b.Endpoints[0])
	phase4Echo(t, bconn)
	cs[0].(*net.TCPConn).CloseWrite()
	phase4Closed(t, cs[0])
	phase4Echo(t, cs[1])
	phase4Echo(t, bconn)
	// Continuous independent data flow overlaps heartbeats, exec, upload, download.
	load := phase4Dial(t, m.Endpoints[0])
	load.SetDeadline(time.Time{})
	var transferred atomic.Int64
	var loadWG sync.WaitGroup
	loadWG.Add(2)
	go func() {
		defer loadWG.Done()
		chunk := make([]byte, 65536)
		for {
			if _, e := load.Write(chunk); e != nil {
				return
			}
		}
	}()
	go func() {
		defer loadWG.Done()
		chunk := make([]byte, 65536)
		for {
			n, e := load.Read(chunk)
			transferred.Add(int64(n))
			if e != nil {
				return
			}
		}
	}()
	start := time.Now()
	r := runExec(t, g, "tunnel-a", task.ExecRequest{Command: "printf tunnel-control-ok", Timeout: 5 * time.Second})
	if r.Stdout != "tunnel-control-ok" || r.Status != "success" {
		t.Fatal(r)
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	remote := filepath.Join(dir, "remote")
	download := filepath.Join(dir, "download")
	payload := bytes.Repeat([]byte{0, 255, 3, 7}, 50000)
	os.WriteFile(source, payload, 0600)
	id, e := g.CreateUpload(context.Background(), "tunnel-a", filetransfer.UploadRequest{SourcePath: source, RemotePath: remote, Mode: "0600", Overwrite: true, Timeout: 10 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	if r := fileResult(t, g, id); r.Status != "success" {
		t.Fatal(r)
	}
	id, e = g.CreateDownload(context.Background(), "tunnel-a", filetransfer.DownloadRequest{RemotePath: remote, ResultName: "download", TargetPath: download, Overwrite: true, Timeout: 10 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	if r := fileResult(t, g, id); r.Status != "success" {
		t.Fatal(r)
	}
	got, e := os.ReadFile(download)
	if e != nil || !bytes.Equal(got, payload) {
		t.Fatal("load download mismatch", e)
	}
	for time.Since(start) < 11*time.Second {
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(logs.String(), "received=HEARTBEAT_ACK") || transferred.Load() < 1024*1024 {
		t.Fatal("control/data load did not run", transferred.Load(), logs.String())
	}
	seen, e := g.Devices().Get("tunnel-a")
	if e != nil || seen.LastSeenAt.Before(start.Add(8*time.Second)) {
		t.Fatal("loaded device heartbeat did not progress", seen.LastSeenAt, e)
	}
	if e := s.CloseMaintenance(m.ID); e != nil {
		t.Fatal(e)
	}
	load.Close()
	loadWG.Wait()
	for _, c := range cs {
		phase4Closed(t, c)
	}
	phase4Echo(t, bconn)
	// Repeated creation and active revoke returns actual Probe FD/thread counts.
	for i := 0; i < 5; i++ {
		next := phase4Create(t, s, "tunnel-a", time.Minute)
		if next.Endpoints[0].Port == m.Endpoints[0].Port {
			t.Fatal("quarantined port reused")
		}
		c := phase4Dial(t, next.Endpoints[0])
		phase4Echo(t, c)
		s.CloseMaintenance(next.ID)
		phase4Closed(t, c)
	}
	until = time.Now().Add(5 * time.Second)
	for {
		f, n := phase4Counts(a.Process.Pid)
		if f <= fds && n <= threads {
			break
		}
		if time.Now().After(until) {
			t.Fatalf("Probe leak fd %d->%d threads %d->%d", fds, f, threads, n)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Replacement on the same identity revokes the old entries and active flow.
	old := phase4Create(t, s, "tunnel-a", time.Minute)
	active := phase4Dial(t, old.Endpoints[1])
	phase4Echo(t, active)
	a.Process.Kill() // real disconnection; restart uses a new Device Session
	phase4Released(t, s, old.ID)
	phase4Closed(t, active)
	startProbe(t, binary, address, "tunnel-a", &logs)
	waitOnline(t, g.Events(), "tunnel-a", 5*time.Second)
	exp := phase4Create(t, s, "tunnel-a", 200*time.Millisecond)
	ec := phase4Dial(t, exp.Endpoints[2])
	phase4Echo(t, ec)
	phase4Released(t, s, exp.ID)
	phase4Closed(t, ec)
	startProbe(t, binary, address, "tunnel-b", &logs)
	phase4Released(t, s, b.ID)
	bProcess.Process.Kill()
	phase4Closed(t, bconn)
	replacement := phase4Create(t, s, "tunnel-b", time.Minute)
	if replacement.SessionID == b.SessionID {
		t.Fatal("replacement inherited session")
	}
	rc := phase4Dial(t, replacement.Endpoints[0])
	phase4Echo(t, rc)
}

func TestTunnelRealProbeMissingLocalAndDataFailure(t *testing.T) {
	binary := probeBinary(t)
	g, s, address := phase4Server(t, tunnel.Config{DataHost: "127.0.0.2", PendingTimeout: 500 * time.Millisecond})
	phase4EchoTarget(t, 80)
	var output lockedBuffer
	startProbe(t, binary, address, "tunnel-failure", &output)
	waitOnline(t, g.Events(), "tunnel-failure", 5*time.Second)
	m := phase4Create(t, s, "tunnel-failure", 0)
	c := phase4Dial(t, m.Endpoints[0])
	phase4Closed(t, c)
	absent := phase4Dial(t, m.Endpoints[2])
	phase4Closed(t, absent)
	state, _ := s.Get(m.ID)
	if state.Endpoints[2].State != "unavailable" || state.Endpoints[0].State != "ready" {
		t.Fatal(state)
	}
	r := runExec(t, g, "tunnel-failure", task.ExecRequest{Command: "printf still-online", Timeout: time.Second})
	if r.Stdout != "still-online" {
		t.Fatal(r)
	}
}

func TestTunnelRealBackpressureHalfCloseAndRevoke(t *testing.T) {
	binary := probeBinary(t)
	l, e := net.Listen("tcp", "127.0.0.1:80")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { l.Close() })
	targetDone := make(chan struct{})
	go func() {
		defer close(targetDone)
		first, e := l.Accept()
		if e != nil {
			return
		}
		body, e := io.ReadAll(first)
		if e == nil {
			first.Write(append(body, []byte("-after-eof")...))
			first.(*net.TCPConn).CloseWrite()
		}
		first.Close()
		second, e := l.Accept()
		if e != nil {
			return
		}
		defer second.Close()
		second.SetWriteDeadline(time.Now().Add(10 * time.Second))
		chunk := make([]byte, 65536)
		for i := 0; i < 1024; i++ {
			if _, e := second.Write(chunk); e != nil {
				return
			}
		}
	}()
	g, s, address := phase4Server(t, tunnel.Config{})
	var output lockedBuffer
	process := startProbe(t, binary, address, "backpressure", &output)
	waitOnline(t, g.Events(), "backpressure", 5*time.Second)
	time.Sleep(60 * time.Millisecond)
	fds, threads := phase4Counts(process.Process.Pid)
	m := phase4Create(t, s, "backpressure", time.Minute)
	c := phase4Dial(t, m.Endpoints[0])
	c.Write([]byte("half-close"))
	c.(*net.TCPConn).CloseWrite()
	got, e := io.ReadAll(c)
	if e != nil || string(got) != "half-close-after-eof" {
		t.Fatal(string(got), e)
	}
	slow := phase4Dial(t, m.Endpoints[0])
	slow.(*net.TCPConn).SetReadBuffer(1024)
	time.Sleep(200 * time.Millisecond)
	result := runExec(t, g, "backpressure", task.ExecRequest{Command: "printf alive-under-backpressure", Timeout: time.Second})
	if result.Stdout != "alive-under-backpressure" {
		t.Fatal(result)
	}
	s.CloseMaintenance(m.ID)
	slow.Close()
	select {
	case <-targetDone:
	case <-time.After(3 * time.Second):
		t.Fatal("local service write was not terminated")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		f, n := phase4Counts(process.Process.Pid)
		if f <= fds && n <= threads {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("backpressure leak: fd=%d/%d threads=%d/%d", f, fds, n, threads)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestTunnelRealHTTPSSHAndTelnet(t *testing.T) {
	binary := probeBinary(t)
	for _, exe := range []string{"sshd", "ssh", "ssh-keygen", "busybox-extras"} {
		if _, e := exec.LookPath(exe); e != nil {
			t.Fatalf("Phase 4 protocol verification requires %s: %v", exe, e)
		}
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "id")
	hostKey := filepath.Join(dir, "host")
	for _, path := range []string{key, hostKey} {
		if b, e := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", path).CombinedOutput(); e != nil {
			t.Fatal(e, string(b))
		}
	}
	public, e := os.ReadFile(key + ".pub")
	if e != nil {
		t.Fatal(e)
	}
	authorized := filepath.Join(dir, "authorized")
	os.WriteFile(authorized, public, 0600)
	cfg := fmt.Sprintf("ListenAddress 127.0.0.1\nPort 22\nHostKey %s\nPidFile %s\nAuthorizedKeysFile %s\nPermitRootLogin prohibit-password\nPasswordAuthentication no\nStrictModes no\n", hostKey, filepath.Join(dir, "pid"), authorized)
	config := filepath.Join(dir, "sshd_config")
	os.WriteFile(config, []byte(cfg), 0600)
	var daemonLog lockedBuffer
	sshd := exec.Command("/usr/sbin/sshd", "-D", "-e", "-f", config)
	sshd.Stdout = &daemonLog
	sshd.Stderr = &daemonLog
	if e := sshd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { sshd.Process.Kill(); sshd.Wait() })
	// Telnet daemon owns protocol negotiation and a test shell; Tunnel remains raw TCP.
	login := filepath.Join(dir, "login")
	os.WriteFile(login, []byte("#!/bin/sh\nexec /bin/sh -i\n"), 0700)
	telnetd := exec.Command("busybox-extras", "telnetd", "-F", "-b", "127.0.0.1", "-p", "23", "-l", login)
	telnetd.Stdout = &daemonLog
	telnetd.Stderr = &daemonLog
	if e := telnetd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { telnetd.Process.Kill(); telnetd.Wait() })
	httpListener, e := net.Listen("tcp", "127.0.0.1:80")
	if e != nil {
		t.Fatal(e)
	}
	httpServer := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "device-web-ok") })}
	go httpServer.Serve(httpListener)
	t.Cleanup(func() { httpServer.Close() })
	time.Sleep(200 * time.Millisecond)
	g, s, address := phase4Server(t, tunnel.Config{PerMaintenance: 16, PerDevice: 16, DataHost: "localhost"})
	var output lockedBuffer
	startProbe(t, binary, address, "protocols", &output, "--tunnel-connections", "16")
	waitOnline(t, g.Events(), "protocols", 5*time.Second)
	m := phase4Create(t, s, "protocols", 0)
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, e := client.Get("http://" + m.Endpoints[0].Address() + "/")
			if e != nil {
				t.Error(e)
				return
			}
			defer response.Body.Close()
			b, e := io.ReadAll(response.Body)
			if e != nil || string(b) != "device-web-ok" {
				t.Error(string(b), e)
			}
		}()
	}
	ssh := exec.Command("ssh", "-F", "/dev/null", "-i", key, "-p", strconv.Itoa(m.Endpoints[1].Port), "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-o", "ConnectTimeout=5", "root@127.0.0.1", "printf real-ssh-command-ok")
	sshBytes, sshErr := ssh.CombinedOutput()
	telnet := phase4Dial(t, m.Endpoints[2])
	telnet.Write([]byte("printf 'real-telnet-command-ok\\n'\r\n"))
	reader := bufio.NewReader(telnet)
	found := false
	for i := 0; i < 20; i++ {
		line, e := reader.ReadString('\n')
		if strings.Contains(line, "real-telnet-command-ok") && !strings.Contains(line, "printf") {
			found = true
			break
		}
		if e != nil {
			break
		}
	}
	wg.Wait()
	if sshErr != nil || !strings.Contains(string(sshBytes), "real-ssh-command-ok") {
		t.Fatal(sshErr, string(sshBytes), daemonLog.String())
	}
	if !found {
		t.Fatal("no Telnet command output", daemonLog.String())
	}
	s.CloseMaintenance(m.ID)
	phase4Closed(t, telnet)
}
