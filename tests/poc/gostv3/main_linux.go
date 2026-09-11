// Isolated GOST v3 evaluation only. Not a Router-Agent product backend.
// Run as root inside unshare -mnpf --mount-proc with a private network/devpts.
package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type result struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail any    `json:"detail"`
}

var results []result
var root string
var procs []*exec.Cmd
var logs []*os.File

func record(name string, pass bool, detail any) {
	r := result{name, pass, detail}
	results = append(results, r)
	b, _ := json.Marshal(r)
	fmt.Println(string(b))
	save()
}
func save() {
	b, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(filepath.Join(root, "results.json"), b, 0600)
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
func command(args ...string) string {
	b, e := exec.Command(args[0], args[1:]...).CombinedOutput()
	if e != nil {
		panic(fmt.Sprintf("%v: %v %s", args, e, b))
	}
	return string(b)
}
func start(name string, args ...string) *exec.Cmd {
	f, e := os.Create(filepath.Join(root, name+".log"))
	must(e)
	logs = append(logs, f)
	c := exec.Command(args[0], args[1:]...)
	c.Stdout = f
	c.Stderr = f
	must(c.Start())
	procs = append(procs, c)
	return c
}
func stop(c *exec.Cmd) {
	if c == nil || c.Process == nil {
		return
	}
	_ = c.Process.Kill()
	_ = c.Wait()
}
func waitPort(addr string) bool {
	until := time.Now().Add(8 * time.Second)
	for time.Now().Before(until) {
		c, e := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if e == nil {
			c.Close()
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
func waitListening(port int) bool {
	needle := fmt.Sprintf(":%04X", port)
	end := time.Now().Add(8 * time.Second)
	for time.Now().Before(end) {
		b, _ := os.ReadFile("/proc/net/tcp")
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) > 3 && strings.HasSuffix(f[1], needle) && f[3] == "0A" {
				return true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
func connect(addr string) (net.Conn, error) {
	c, e := net.DialTimeout("tcp", addr, 2*time.Second)
	if e == nil {
		c.SetDeadline(time.Now().Add(3 * time.Second))
	}
	return c, e
}
func exchange(c net.Conn, data []byte) (bool, string) {
	c.SetDeadline(time.Now().Add(3 * time.Second))
	if _, e := c.Write(data); e != nil {
		return false, e.Error()
	}
	b := make([]byte, len(data))
	_, e := io.ReadFull(c, b)
	return e == nil && bytes.Equal(b, data), fmt.Sprintf("bytes=%d error=%v", len(data), e)
}
func tcp(addr string, data []byte) (bool, string) {
	c, e := connect(addr)
	if e != nil {
		return false, e.Error()
	}
	defer c.Close()
	return exchange(c, data)
}
func udp(addr string, data []byte) (bool, string) {
	c, e := net.DialTimeout("udp", addr, time.Second)
	if e != nil {
		return false, e.Error()
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(4 * time.Second))
	_, e = c.Write(data)
	if e != nil {
		return false, e.Error()
	}
	b := make([]byte, 65536)
	n, e := c.Read(b)
	return e == nil && bytes.Equal(b[:n], data), fmt.Sprintf("sent=%d received=%d error=%v", len(data), n, e)
}
func api(method, path string, body any) (int, []byte) {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	req, e := http.NewRequest(method, "http://127.0.0.1:39002"+path, bytes.NewReader(b))
	must(e)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 5 * time.Second}
	r, e := client.Do(req)
	if e != nil {
		return 0, []byte(e.Error())
	}
	defer r.Body.Close()
	b, _ = io.ReadAll(r.Body)
	return r.StatusCode, b
}
func sample(pid int) map[string]any {
	m := map[string]any{}
	b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	for _, s := range strings.Split(string(b), "\n") {
		for _, k := range []string{"VmRSS:", "VmHWM:", "Threads:"} {
			if strings.HasPrefix(s, k) {
				m[strings.TrimSuffix(k, ":")] = strings.TrimSpace(strings.TrimPrefix(s, k))
			}
		}
	}
	fds, _ := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	m["fds"] = len(fds)
	stat, _ := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	m["stat"] = string(stat)
	return m
}
func echo() {
	l, e := net.Listen("tcp", ":29080")
	must(e)
	go func() {
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			fmt.Println("TCP_SOURCE", c.RemoteAddr())
			go func() { defer c.Close(); io.Copy(c, c) }()
		}
	}()
	u, e := net.ListenPacket("udp", ":29081")
	must(e)
	b := make([]byte, 65536)
	for {
		n, a, e := u.ReadFrom(b)
		if e != nil {
			return
		}
		fmt.Println("UDP_SOURCE", a)
		u.WriteTo(b[:n], a)
	}
}
func pty() (int, string) {
	f, e := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
	must(e)
	var zero, number uint32
	_, _, er := syscall.Syscall(syscall.SYS_IOCTL, uintptr(f), 0x40045431, uintptr(unsafe.Pointer(&zero)))
	if er != 0 {
		panic(er)
	}
	_, _, er = syscall.Syscall(syscall.SYS_IOCTL, uintptr(f), 0x80045430, uintptr(unsafe.Pointer(&number)))
	if er != 0 {
		panic(er)
	}
	return f, fmt.Sprintf("/dev/pts/%d", number)
}
func ptyRead(fd int, count int, wait time.Duration) ([]byte, error) {
	var out []byte
	until := time.Now().Add(wait)
	for time.Now().Before(until) {
		b := make([]byte, 4096)
		n, e := syscall.Read(fd, b)
		if n > 0 {
			out = append(out, b[:n]...)
			if len(out) >= count {
				return out, nil
			}
		}
		if e != nil && e != syscall.EAGAIN && e != syscall.EINTR && e != syscall.EIO {
			return out, e
		}
		time.Sleep(5 * time.Millisecond)
	}
	return out, errors.New("PTY read timeout")
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "echo" {
		echo()
		return
	}
	root = "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	must(os.MkdirAll(root, 0700))
	must(os.WriteFile("/proc/sys/net/ipv4/ip_local_port_range", []byte("45000 60999"), 0600))
	defer func() {
		for i := len(procs) - 1; i >= 0; i-- {
			stop(procs[i])
		}
		for _, f := range logs {
			f.Close()
		}
		if r := recover(); r != nil {
			record("harness_setup", false, fmt.Sprint(r))
			os.Exit(1)
		}
	}()
	gost := os.Getenv("GOST_BIN")
	if gost == "" {
		gost = filepath.Join(root, "gost")
	}
	self, e := os.Executable()
	must(e)
	target := start("target", "unshare", "-n", self, "echo")
	time.Sleep(150 * time.Millisecond)
	command("ip", "link", "add", "gp", "type", "veth") // BusyBox assigns the peer veth0.
	command("ip", "link", "set", "veth0", "netns", strconv.Itoa(target.Process.Pid))
	command("ip", "addr", "add", "198.18.10.1/24", "dev", "gp")
	command("ip", "link", "set", "gp", "up")
	ns := func(args ...string) string {
		a := []string{"nsenter", "-t", strconv.Itoa(target.Process.Pid), "-n", "--"}
		a = append(a, args...)
		return command(a...)
	}
	ns("ip", "link", "set", "lo", "up")
	ns("ip", "addr", "add", "198.18.10.2/24", "dev", "veth0")
	ns("ip", "link", "set", "veth0", "up")
	empty := start("no-mappings", gost, "-api", "127.0.0.1:39501")
	waitPort("127.0.0.1:39501")
	record("resource_no_mappings", true, sample(empty.Process.Pid))
	stop(empty)
	record("target_no_default_route", !strings.Contains(ns("ip", "route", "show"), "default"), ns("ip", "route", "show"))
	must(os.WriteFile(filepath.Join(root, "topology.txt"), []byte(command("ip", "addr", "show")+ns("ip", "addr", "show")), 0600))
	server := start("server", gost, "-L", "relay://127.0.0.1:39000?bind=true", "-api", "127.0.0.1:39001")
	if !waitPort("127.0.0.1:39000") {
		panic("relay server did not start")
	}
	if os.Getenv("GOST_POC_FOCUS") == "1" {
		serialBridgeTests(gost)
		parentDeathTest(gost)
		record("focused_checks_finished", true, "configuration-only serial ownership workaround and orphan lifecycle")
		return
	}
	client := start("client", gost, "-L", "rtcp://127.0.0.1:39101/198.18.10.2:29080", "-L", "rudp://127.0.0.1:39102/198.18.10.2:29081", "-F", "relay://127.0.0.1:39000", "-api", "127.0.0.1:39002")
	if !waitPort("127.0.0.1:39101") {
		panic("reverse TCP did not start")
	}
	waitPort("127.0.0.1:39002")
	payload := bytes.Repeat([]byte{0, 255, 128, 13, 10, 27, 65}, 8192)
	ok, d := tcp("127.0.0.1:39101", payload)
	record("reverse_tcp_binary_no_gateway", ok, d)
	ok, d = udp("127.0.0.1:39102", payload[:1200])
	record("reverse_udp_no_gateway", ok, d)
	ns("ip", "route", "add", "default", "via", "198.18.10.254")
	ok, d = tcp("127.0.0.1:39101", payload)
	record("reverse_tcp_other_gateway", ok, d)
	ok, d = udp("127.0.0.1:39102", payload[:1200])
	record("reverse_udp_other_gateway", ok, d)
	ns("ip", "route", "del", "default")
	time.Sleep(time.Second)
	record("client_resource_zero_active", true, sample(client.Process.Pid))
	idleBefore := sample(client.Process.Pid)
	time.Sleep(3 * time.Second)
	record("idle_cpu_three_second_samples", true, map[string]any{"before": idleBefore, "after": sample(client.Process.Pid), "seconds": 3})
	var conns []net.Conn
	for i := 0; i < 8; i++ {
		c, e := connect("127.0.0.1:39101")
		must(e)
		conns = append(conns, c)
		ok, _ := exchange(c, []byte{byte(i)})
		if !ok {
			panic("8 stream setup failed")
		}
		if i == 0 {
			record("client_resource_one_active", true, sample(client.Process.Pid))
		}
	}
	record("client_resource_eight_active", true, sample(client.Process.Pid))
	for _, c := range conns {
		c.Close()
	}
	// Concurrent UDP origins must each receive their own payload.
	var wg sync.WaitGroup
	out := make(chan bool, 8)
	var udpDetailsMu sync.Mutex
	var udpDetails []string
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, detail := udp("127.0.0.1:39102", bytes.Repeat([]byte{byte(i + 1)}, 600))
			udpDetailsMu.Lock()
			udpDetails = append(udpDetails, fmt.Sprintf("origin %d: %s", i, detail))
			udpDetailsMu.Unlock()
			out <- ok
		}(i)
	}
	wg.Wait()
	close(out)
	all := true
	for v := range out {
		all = all && v
	}
	record("udp_eight_origin_isolation", all, udpDetails)
	for _, n := range []int{1, 1200, 8192, 65000, 0} {
		ok, d = udp("127.0.0.1:39102", make([]byte, n))
		record(fmt.Sprintf("udp_datagram_%d", n), ok, d)
	}
	_, cfgBytes := api("GET", "/config", nil)
	os.WriteFile(filepath.Join(root, "client-config.json"), cfgBytes, 0600)
	var cfg map[string]any
	must(json.Unmarshal(cfgBytes, &cfg))
	services, valid := cfg["services"].([]any)
	if !valid {
		panic("unexpected API config: " + string(cfgBytes))
	}
	original := services[0].(map[string]any)
	oldName := original["name"].(string)
	clone := func(src map[string]any) map[string]any {
		b, _ := json.Marshal(src)
		var d map[string]any
		json.Unmarshal(b, &d)
		return d
	}
	live, e := connect("127.0.0.1:39101")
	must(e)
	defer live.Close()
	ok, _ = exchange(live, []byte("before"))
	mustBool(ok, "live before deletion")
	extra := clone(original)
	extra["name"] = "extra"
	extra["addr"] = "127.0.0.1:39103"
	status, b := api("POST", "/config/services", extra)
	record("dynamic_add", status == 200, map[string]any{"status": status, "body": string(b)})
	waitPort("127.0.0.1:39103")
	ok, d = exchange(live, []byte("unaffected"))
	record("add_preserves_other_live_connection", ok, d)
	status, b = api("DELETE", "/config/services/extra", nil)
	ok, d = exchange(live, []byte("unaffected2"))
	record("delete_other_preserves_live_connection", status == 200 && ok, d)
	// This assertion deliberately fails if upstream deletion leaves active streams alive.
	startTime := time.Now()
	status, b = api("DELETE", "/config/services/"+oldName, nil)
	time.Sleep(300 * time.Millisecond)
	stillWorks, detail := exchange(live, []byte("after-delete"))
	newWorks, newDetail := tcp("127.0.0.1:39101", []byte("new-after-delete"))
	record("delete_revokes_existing_and_listener", status == 200 && !stillWorks && !newWorks, map[string]any{"status": status, "existing_still_echoes": stillWorks, "existing": detail, "new_still_echoes": newWorks, "new": newDetail, "elapsed_ms": time.Since(startTime).Milliseconds()})
	api("POST", "/config/services", original)
	waitPort("127.0.0.1:39101")
	// Preserve precise upstream behavior for port collisions and target failures.
	duplicate := clone(original)
	duplicate["name"] = "collision"
	status, b = api("POST", "/config/services", duplicate)
	record("collision_response_observation", true, map[string]any{"status": status, "body": string(b)})
	api("DELETE", "/config/services/collision", nil)
	for i := 0; i < 20; i++ {
		extra["name"] = "cycle"
		api("POST", "/config/services", extra)
		waitPort("127.0.0.1:39103")
		tcp("127.0.0.1:39103", []byte("cycle"))
		api("DELETE", "/config/services/cycle", nil)
	}
	time.Sleep(7 * time.Second)
	record("resources_after_20_add_close_cycles", true, sample(client.Process.Pid))
	// Negative target and wrong subnet: a ready listener is not target reachability.
	bad := clone(original)
	bad["name"] = "unreachable"
	bad["addr"] = "127.0.0.1:39104"
	bad["forwarder"] = map[string]any{"nodes": []any{map[string]any{"name": "bad", "addr": "198.18.11.2:29080"}}}
	status, _ = api("POST", "/config/services", bad)
	waitPort("127.0.0.1:39104")
	badWorks, badDetail := tcp("127.0.0.1:39104", []byte("unreachable"))
	record("wrong_subnet_is_not_usable", !badWorks, map[string]any{"api_status": status, "error": badDetail})
	api("DELETE", "/config/services/unreachable", nil)
	// Same public port can be re-bound immediately: upstream does not provide platform quarantine.
	q := clone(original)
	q["name"] = "reuse"
	q["addr"] = "127.0.0.1:39103"
	status, _ = api("POST", "/config/services", q)
	waitPort("127.0.0.1:39103")
	reuse, _ := tcp("127.0.0.1:39103", []byte("reused"))
	record("native_port_quarantine_present", !reuse, map[string]any{"rebind_status": status, "immediate_reuse": reuse})
	api("DELETE", "/config/services/reuse", nil)
	// Increase documented UDP buffers; do not change upstream source or expected datagram bytes.
	stop(server)
	server = start("server-large-buffer", gost, "-L", "relay://127.0.0.1:39000?bind=true&udp.bufferSize=65535", "-api", "127.0.0.1:39001")
	waitPort("127.0.0.1:39000")
	waitPort("127.0.0.1:39101")
	udpService := clone(services[1].(map[string]any))
	udpService["listener"].(map[string]any)["metadata"] = map[string]any{"readBufferSize": 65535}
	us, ub := api("PUT", "/config/services/"+udpService["name"].(string), udpService)
	record("udp_tuning_api_response", us == 200, string(ub))
	_, uc := api("GET", "/config", nil)
	os.WriteFile(filepath.Join(root, "tuned-config.json"), uc, 0600)
	time.Sleep(time.Second)
	for _, n := range []int{1, 8192, 32000, 65000, 0} {
		ok, d = udp("127.0.0.1:39102", bytes.Repeat([]byte{0x7f}, n))
		record(fmt.Sprintf("udp_tuned_datagram_%d", n), ok, d)
	}
	// Independent process restart demonstrates upstream reconnect, not platform auto-recovery.
	ok, d = tcp("127.0.0.1:39101", []byte("after-relay-restart"))
	record("old_client_auto_reconnects_after_relay_restart", ok, d)

	// A live relay cannot observe the platform control session unless integrated.
	c, e := connect("127.0.0.1:39101")
	must(e)
	ok, _ = exchange(c, []byte("before-crash"))
	mustBool(ok, "before crash")
	stop(client)
	time.Sleep(300 * time.Millisecond)
	stillWorks, _ = exchange(c, []byte("after-client-crash"))
	c.Close()
	record("client_crash_revokes_tcp", !stillWorks, "standalone client process killed; not a Router-Agent session test")
	// Serial tests use a PTY only, never physical console or vendor serial devices.
	serialTests(gost, server, payload)
	serialLimitedTests(gost)
	securityTests(gost)
	parentDeathTest(gost)
	record("end_of_finite_probe", true, "No product integration, real UART, ARM execution or one-hour soak claimed")
}
func mustBool(ok bool, s string) {
	if !ok {
		panic(s)
	}
}

// Both listener and handler chains are explicit to keep the local serial node address intact.
func serialConfig(name, port, path string, baud int, transport string) map[string]any {
	return map[string]any{"services": []any{map[string]any{"name": name, "addr": "127.0.0.1:" + port, "listener": map[string]any{"type": "r" + transport, "chain": "reverse"}, "handler": map[string]any{"type": "r" + transport, "chain": "serial"}, "forwarder": map[string]any{"nodes": []any{map[string]any{"name": "uart", "addr": "127.0.0.1:1"}}}}}, "chains": []any{map[string]any{"name": "reverse", "hops": []any{map[string]any{"name": "reverse-hop", "nodes": []any{map[string]any{"name": "relay", "addr": "127.0.0.1:39000", "connector": map[string]any{"type": "relay"}, "dialer": map[string]any{"type": "tcp"}}}}}}, map[string]any{"name": "serial", "hops": []any{map[string]any{"name": "serial-hop", "nodes": []any{map[string]any{"name": "serial-node", "addr": fmt.Sprintf("%s,%d", path, baud), "connector": map[string]any{"type": "forward"}, "dialer": map[string]any{"type": "serial"}}}}}}}}
}
func configProcess(gost, name string, cfg map[string]any) *exec.Cmd {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	path := filepath.Join(root, name+".json")
	must(os.WriteFile(path, b, 0600))
	return start(name, gost, "-C", path)
}
func serialTests(gost string, server *exec.Cmd, payload []byte) {
	for _, baud := range []int{9600, 115200} {
		fd, path := pty()
		port := "39201"
		cfg := serialConfig("serial", port, path, baud, "tcp")
		cfg["api"] = map[string]any{"addr": "127.0.0.1:39002"}
		proc := configProcess(gost, fmt.Sprintf("serial-%d", baud), cfg)
		if !waitListening(39201) {
			record("serial_config_start", false, "consult serial log")
			stop(proc)
			syscall.Close(fd)
			continue
		}
		time.Sleep(100 * time.Millisecond)
		c, e := connect("127.0.0.1:" + port)
		if e != nil {
			record("serial_connect", false, e.Error())
			stop(proc)
			syscall.Close(fd)
			continue
		}
		c.SetDeadline(time.Now().Add(2 * time.Second))
		_, e = c.Write(payload[:1024])
		got, re := ptyRead(fd, 1024, 2*time.Second)
		record(fmt.Sprintf("tcp_to_pty_binary_%d", baud), e == nil && re == nil && bytes.Equal(got, payload[:1024]), fmt.Sprintf("write=%v read=%v count=%d", e, re, len(got)))
		if re == nil {
			_, e = syscall.Write(fd, payload[:1024])
			b := make([]byte, 1024)
			c.SetDeadline(time.Now().Add(2 * time.Second))
			_, re = io.ReadFull(c, b)
			record(fmt.Sprintf("pty_to_tcp_binary_%d", baud), e == nil && re == nil && bytes.Equal(b, payload[:1024]), fmt.Sprintf("write=%v read=%v", e, re))
			var term syscall.Termios
			_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TCGETS, uintptr(unsafe.Pointer(&term)))
			expected := uint32(syscall.B9600)
			if baud == 115200 {
				expected = syscall.B115200
			}
			record(fmt.Sprintf("pty_termios_%d", baud), errno == 0 && (term.Cflag&0x100f) == expected, map[string]any{"cflag": term.Cflag, "speed_bits": term.Cflag & 0x100f, "expected": expected})
			c2, e := connect("127.0.0.1:" + port)
			if e == nil {
				c2.Write([]byte("second-writer"))
				got, re = ptyRead(fd, len("second-writer"), time.Second)
				record(fmt.Sprintf("serial_exclusive_second_writer_rejected_%d", baud), re != nil || !bytes.Contains(got, []byte("second-writer")), fmt.Sprintf("second client bytes=%q error=%v", got, re))
				c2.Close()
			}
			status, _ := api("DELETE", "/config/services/serial", nil)
			time.Sleep(300 * time.Millisecond)
			c.SetDeadline(time.Now().Add(time.Second))
			c.Write([]byte("post-close"))
			got, re = ptyRead(fd, len("post-close"), time.Second)
			record(fmt.Sprintf("serial_delete_stops_writes_%d", baud), status == 200 && !bytes.Contains(got, []byte("post-close")), fmt.Sprintf("api=%d received=%q error=%v", status, got, re))
		}
		c.Close()
		stop(proc)
		syscall.Close(fd)
	}
	// UDP raw-to-serial wiring: probe replies and ownership separately, no fake echo server.
	fd, path := pty()
	defer syscall.Close(fd)
	proc := configProcess(gost, "serial-udp", serialConfig("serial-udp", "39202", path, 9600, "udp"))
	defer stop(proc)
	time.Sleep(time.Second)
	u, e := net.Dial("udp", "127.0.0.1:39202")
	must(e)
	defer u.Close()
	u.Write([]byte("serial-udp-A"))
	got, re := ptyRead(fd, 12, 2*time.Second)
	record("udp_to_pty_binary", re == nil && bytes.Equal(got, []byte("serial-udp-A")), fmt.Sprintf("got=%q error=%v", got, re))
	if re == nil {
		syscall.Write(fd, []byte("reply-A"))
		u.SetDeadline(time.Now().Add(2 * time.Second))
		b := make([]byte, 100)
		n, e := u.Read(b)
		record("pty_to_udp_reply", e == nil && bytes.Equal(b[:n], []byte("reply-A")), fmt.Sprintf("got=%q error=%v", b[:n], e))
		u2, e := net.Dial("udp", "127.0.0.1:39202")
		must(e)
		defer u2.Close()
		u2.Write([]byte("serial-udp-B"))
		got, re = ptyRead(fd, 12, time.Second)
		record("serial_udp_first_origin_exclusive", re != nil || !bytes.Contains(got, []byte("serial-udp-B")), fmt.Sprintf("second origin bytes=%q error=%v", got, re))
	}
	_ = server
}

func limited(cfg map[string]any) {
	cfg["climiters"] = []any{map[string]any{"name": "exclusive", "limits": []string{"$ 1"}}}
	cfg["services"].([]any)[0].(map[string]any)["climiter"] = "exclusive"
}
func serialLimitedTests(gost string) {
	for _, transport := range []string{"tcp", "udp"} {
		fd, path := pty()
		cfg := serialConfig("limited", "39203", path, 115200, transport)
		limited(cfg)
		proc := configProcess(gost, "serial-limited-"+transport, cfg)
		if transport == "tcp" {
			waitListening(39203)
		} else {
			time.Sleep(time.Second)
		}
		a, e := net.Dial(transport, "127.0.0.1:39203")
		must(e)
		a.Write([]byte("owner"))
		got, re := ptyRead(fd, 5, 2*time.Second)
		record("serial_"+transport+"_limited_first_owner", re == nil && bytes.Equal(got, []byte("owner")), fmt.Sprintf("bytes=%q error=%v", got, re))
		b, e := net.Dial(transport, "127.0.0.1:39203")
		must(e)
		b.Write([]byte("intruder"))
		got, re = ptyRead(fd, 8, time.Second)
		record("serial_"+transport+"_configured_exclusive", !bytes.Contains(got, []byte("intruder")), fmt.Sprintf("bytes=%q error=%v", got, re))
		syscall.Write(fd, []byte("to-owner"))
		a.SetDeadline(time.Now().Add(time.Second))
		buf := make([]byte, 100)
		n, err := a.Read(buf)
		record("serial_"+transport+"_owner_keeps_reply", err == nil && bytes.Equal(buf[:n], []byte("to-owner")), fmt.Sprintf("bytes=%q error=%v", buf[:n], err))
		a.Close()
		b.Close()
		stop(proc)
		syscall.Close(fd)
	}
}
func securityTests(gost string) {
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(e)
	now := time.Now()
	tmpl := x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "gost-poc.test"}, DNSNames: []string{"gost-poc.test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, e := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	must(e)
	certPath := filepath.Join(root, "test-ca.pem")
	keyPath := filepath.Join(root, "test-key.pem")
	os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600)
	kb, e := x509.MarshalECPrivateKey(key)
	must(e)
	os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0600)
	auth := map[string]any{"username": "test-only", "password": "ephemeral-test-credential"}
	serverCfg := map[string]any{"services": []any{map[string]any{"name": "secure-relay", "addr": "127.0.0.1:39300", "listener": map[string]any{"type": "tls", "tls": map[string]any{"certFile": certPath, "keyFile": keyPath}}, "handler": map[string]any{"type": "relay", "auth": auth, "metadata": map[string]any{"bind": true}}}}}
	server := configProcess(gost, "tls-server", serverCfg)
	defer stop(server)
	waitListening(39300)
	for _, mode := range []string{"trusted", "wrong-name", "untrusted", "wrong-credential"} {
		tls := map[string]any{"serverName": "gost-poc.test", "secure": true, "caFile": certPath}
		a := map[string]any{"username": "test-only", "password": "ephemeral-test-credential"}
		if mode == "wrong-name" {
			tls["serverName"] = "wrong.test"
		}
		if mode == "untrusted" {
			delete(tls, "caFile")
		}
		if mode == "wrong-credential" {
			a["password"] = "invalid"
		}
		cfg := map[string]any{"services": []any{map[string]any{"name": "secure-test", "addr": "127.0.0.1:39301", "listener": map[string]any{"type": "rtcp", "chain": "secure"}, "handler": map[string]any{"type": "rtcp"}, "forwarder": map[string]any{"nodes": []any{map[string]any{"name": "echo", "addr": "198.18.10.2:29080"}}}}}, "chains": []any{map[string]any{"name": "secure", "hops": []any{map[string]any{"name": "tls", "nodes": []any{map[string]any{"name": "relay", "addr": "127.0.0.1:39300", "connector": map[string]any{"type": "relay", "auth": a}, "dialer": map[string]any{"type": "tls", "tls": tls}}}}}}}}
		c := configProcess(gost, "tls-"+mode, cfg)
		if mode == "trusted" {
			waitListening(39301)
		} else {
			time.Sleep(1200 * time.Millisecond)
		}
		ok, d := tcp("127.0.0.1:39301", []byte("secret-test-bytes"))
		expected := mode == "trusted"
		record("tls_"+mode, ok == expected, map[string]any{"forwarding_works": ok, "detail": d})
		stop(c)
		time.Sleep(100 * time.Millisecond)
	}
}
func parentDeathTest(gost string) {
	// Only this test's own spawned child is identified and killed. No host process scan/kill.
	pidfile := filepath.Join(root, "orphan.pid")
	parent := start("parent", "sh", "-c", `"$1" -api 127.0.0.1:15000 & echo $! > "$2"; wait`, "poc-parent", gost, pidfile)
	if !waitListening(15000) {
		record("parent_death_setup", false, "listener missing")
		return
	}
	stop(parent)
	time.Sleep(300 * time.Millisecond)
	b, e := os.ReadFile(pidfile)
	must(e)
	pid, e := strconv.Atoi(strings.TrimSpace(string(b)))
	must(e)
	path, e := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	alive := e == nil && path == gost
	record("native_parent_death_cleanup", !alive, map[string]any{"child_alive_after_parent_kill": alive})
	if alive && pid > 1 {
		p, e := os.FindProcess(pid)
		if e == nil {
			p.Kill()
		}
	}
}

// An extra local GOST service moves the connection limiter off the reverse listener.
// This is a configuration-only candidate workaround, not a new custom backend.
func serialBridgeTests(gost string) {
	for _, transport := range []string{"tcp", "udp"} {
		fd, path := pty()
		cfg := serialConfig("remote", "39203", path, 115200, transport)
		remote := cfg["services"].([]any)[0].(map[string]any)
		delete(remote["handler"].(map[string]any), "chain")
		remote["forwarder"] = map[string]any{"nodes": []any{map[string]any{"name": "local-serial", "addr": "127.0.0.1:39204"}}}
		local := map[string]any{"name": "serial-bridge", "addr": "127.0.0.1:39204", "listener": map[string]any{"type": transport, "metadata": map[string]any{"ttl": "60s"}}, "handler": map[string]any{"type": transport, "chain": "serial"}, "forwarder": map[string]any{"nodes": []any{map[string]any{"name": "uart", "addr": "127.0.0.1:1"}}}, "climiter": "exclusive"}
		cfg["services"] = []any{remote, local}
		cfg["climiters"] = []any{map[string]any{"name": "exclusive", "limits": []string{"$ 1"}}}
		proc := configProcess(gost, "serial-local-bridge-"+transport, cfg)
		if transport == "tcp" {
			waitListening(39203)
		} else {
			time.Sleep(time.Second)
		}
		a, e := net.Dial(transport, "127.0.0.1:39203")
		must(e)
		a.Write([]byte("owner"))
		got, re := ptyRead(fd, 5, 2*time.Second)
		record("bridge_"+transport+"_first_owner", re == nil && bytes.Equal(got, []byte("owner")), fmt.Sprintf("bytes=%q error=%v", got, re))
		b, e := net.Dial(transport, "127.0.0.1:39203")
		must(e)
		b.Write([]byte("intruder"))
		got, re = ptyRead(fd, 8, time.Second)
		record("bridge_"+transport+"_second_writer_rejected", !bytes.Contains(got, []byte("intruder")), fmt.Sprintf("bytes=%q error=%v", got, re))
		syscall.Write(fd, []byte("to-owner"))
		a.SetDeadline(time.Now().Add(time.Second))
		buf := make([]byte, 100)
		n, err := a.Read(buf)
		record("bridge_"+transport+"_owner_reply_preserved", err == nil && bytes.Equal(buf[:n], []byte("to-owner")), fmt.Sprintf("bytes=%q error=%v", buf[:n], err))
		// Existing connection must stay usable after the second rejected connection.
		a.SetDeadline(time.Now().Add(time.Second))
		a.Write([]byte("owner-again"))
		got, re = ptyRead(fd, 11, time.Second)
		record("bridge_"+transport+"_owner_write_preserved", re == nil && bytes.Equal(got, []byte("owner-again")), fmt.Sprintf("bytes=%q error=%v", got, re))
		a.Close()
		b.Close()
		stop(proc)
		syscall.Close(fd)
	}
}
