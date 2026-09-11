//go:build linux

package integration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"routerprobe/internal/api"
	"routerprobe/internal/device"
	"routerprobe/internal/devicelog"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/probetemplate"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

type identityPTY struct {
	fd       int
	path     string
	mu       sync.Mutex
	commands []string
	stop     chan struct{}
	done     chan struct{}
}

func makeIdentityPTY(t *testing.T) *identityPTY {
	t.Helper()
	fd, e := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
	if e != nil {
		t.Fatal(e)
	}
	var unlock uint32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); e != 0 {
		syscall.Close(fd)
		t.Fatal(e)
	}
	var number uint32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&number))); e != 0 {
		syscall.Close(fd)
		t.Fatal(e)
	}
	p := &identityPTY{fd: fd, path: fmt.Sprintf("/dev/pts/%d", number), stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(p.done)
		var line strings.Builder
		b := make([]byte, 256)
		for {
			select {
			case <-p.stop:
				return
			default:
			}
			n, e := syscall.Read(fd, b)
			if e != nil || n <= 0 {
				time.Sleep(2 * time.Millisecond)
				continue
			}
			for _, c := range b[:n] {
				if c == '\r' {
					command := line.String()
					line.Reset()
					p.mu.Lock()
					p.commands = append(p.commands, command)
					p.mu.Unlock()
					answer := "ERROR\r\n"
					switch command {
					case "AT":
						answer = "OK\r\n"
					case "ATI":
						answer = "+CREG: 1\r\nIntegration USB modem\r\nFirmware test-only\r\nOK\r\n"
					case "AT+CGSN":
						answer = "867123456789012\r\nOK\r\n"
					}
					wire := []byte(command + "\r\n" + answer)
					for len(wire) > 0 {
						n, e := syscall.Write(fd, wire)
						if e != nil || n <= 0 {
							break
						}
						wire = wire[n:]
					}
				} else if c != '\n' {
					line.WriteByte(c)
				}
			}
		}
	}()
	t.Cleanup(func() { close(p.stop); <-p.done; syscall.Close(fd) })
	return p
}
func (p *identityPTY) count() int { p.mu.Lock(); defer p.mu.Unlock(); return len(p.commands) }
func linkIdentityTTY(t *testing.T, root, name string, p *identityPTY) {
	t.Helper()
	parent := filepath.Join(root, "sys/devices/mock-usb/usb1/1-1/1-1:1.0")
	for _, d := range []string{parent, filepath.Join(root, "sys/class/tty", name), filepath.Join(root, "dev/pts")} {
		if e := os.MkdirAll(d, 0700); e != nil {
			t.Fatal(e)
		}
	}
	for n, s := range map[string]string{"idVendor": "1234", "idProduct": "5678"} {
		if e := os.WriteFile(filepath.Join(filepath.Dir(parent), n), []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.Symlink("/sys/devices/mock-usb/usb1/1-1/1-1:1.0", filepath.Join(root, "sys/class/tty", name, "device")); e != nil {
		t.Fatal(e)
	}
	info, e := os.Stat(p.path)
	if e != nil {
		t.Fatal(e)
	}
	rdev := uint64(info.Sys().(*syscall.Stat_t).Rdev)
	major := ((rdev >> 8) & 0xfff) | ((rdev >> 32) & 0xfffff000)
	minor := (rdev & 0xff) | ((rdev >> 12) & 0xffffff00)
	if e := os.WriteFile(filepath.Join(root, "sys/class/tty", name, "dev"), []byte(fmt.Sprintf("%d:%d", major, minor)), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(p.path, filepath.Join(root, "dev", name)); e != nil {
		t.Fatal(e)
	}
}
func TestCellularIdentityRealProbeAutomaticRenumber(t *testing.T) {
	binary := probeBinary(t)
	root := t.TempDir()
	busy := makeIdentityPTY(t)
	available := makeIdentityPTY(t)
	linkIdentityTTY(t, root, "ttyUSB0", busy)
	linkIdentityTTY(t, root, "ttyUSB2", available)
	held, e := os.OpenFile(busy.path, os.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer held.Close()
	for _, n := range []string{"null", "urandom"} {
		if e := os.WriteFile(filepath.Join(root, "dev", n), nil, 0600); e != nil {
			t.Fatal(e)
		}
	}
	app, e := management.New(management.Config{RepositoryDirectory: t.TempDir(), Gateway: gateway.Config{HeartbeatInterval: 10 * time.Second, Logger: log.New(io.Discard, "", 0)}})
	if e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(l) }()
	adapter, e := api.New(app, api.Config{})
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(adapter)
	defer func() { server.Close(); adapter.Close(); app.Close(); <-done }()
	tpl, e := app.ProbeTemplates().Put("", 0, probetemplate.Input{Name: "auto AT", Properties: map[string]probetemplate.Property{}, Monitoring: &probetemplate.Monitoring{}, CellularProbe: &probetemplate.CellularProbe{Interval: 10}})
	if e != nil {
		t.Fatal(e)
	}
	// Only this child sees synthetic sysfs/dev. No host /dev nodes or sysfs are changed.
	script := `set -eu
mount --make-rprivate /
mount --bind /dev/pts "$1/dev/pts"
mount --bind /dev/null "$1/dev/null"
mount --bind /dev/urandom "$1/dev/urandom"
mount --bind "$1/sys" /sys
mount --rbind "$1/dev" /dev
shift
exec "$@"`
	cmd := exec.Command("unshare", "-m", "sh", "-c", script, "at-fixture", root, binary, "--server", l.Addr().String(), "--device-id", "cellular-integration", "--boot-id", "cellular-test-boot")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	adoptProbe(t, app, "cellular-integration", tpl.ID, nil)
	wait := func(check func(device.Snapshot) bool) device.Snapshot {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			v, e := app.Devices().Get("cellular-integration")
			if e == nil && check(v) {
				return v
			}
			time.Sleep(30 * time.Millisecond)
		}
		v, _ := app.Devices().Get("cellular-integration")
		t.Fatalf("cellular deadline: state=%s config=%d snapshot=%+v", v.Status, v.LatestSession.ConfigRevision, v.LatestSession.Cellular)
		return v
	}
	first := wait(func(v device.Snapshot) bool {
		c := v.LatestSession.Cellular
		return c != nil && c.Status == "ok" && len(c.Ports) == 2
	})
	// The merged Probe must expose and serve logs on the same control session
	// while AT telemetry remains active, without dropping neighbor capabilities.
	for _, capability := range []string{"neighbors_v1", "neighbors_inspect_v1", "cellular_identity_v1", "device_logs_v1"} {
		if !slices.Contains(first.Registration.Capabilities, capability) {
			t.Fatalf("merged Probe missing %s", capability)
		}
	}
	logDirectory := t.TempDir()
	logName := "FF_BKDATA_2026-09-11.txt"
	if err := os.WriteFile(filepath.Join(logDirectory, logName), []byte("integration log\n"), 0600); err != nil {
		t.Fatal(err)
	}
	query := url.Values{"session_id": {first.LatestSession.ID}, "directory": {logDirectory}}
	logResponse, err := http.Get(server.URL + "/api/v1/devices/cellular-integration/logs/history?" + query.Encode())
	if err != nil {
		t.Fatal(err)
	}
	var logBody struct {
		Data struct {
			SessionID string            `json:"session_id"`
			Value     devicelog.History `json:"value"`
		} `json:"data"`
	}
	err = json.NewDecoder(logResponse.Body).Decode(&logBody)
	logResponse.Body.Close()
	if err != nil || logResponse.StatusCode != 200 || logBody.Data.SessionID != first.LatestSession.ID || len(logBody.Data.Value.Files) != 1 || logBody.Data.Value.Files[0].Name != logName {
		t.Fatalf("log EVENT alongside AT telemetry: status=%d body=%+v error=%v", logResponse.StatusCode, logBody, err)
	}
	var chosen device.CellularPort
	for _, p := range first.LatestSession.Cellular.Ports {
		if p.Selected {
			chosen = p
		}
		if p.Path == "/dev/ttyUSB0" && p.Status != "busy" {
			t.Fatal("busy port was probed")
		}
	}
	if chosen.Path != "/dev/ttyUSB2" || chosen.IMEI.Value != "867123456789012" || busy.count() != 0 {
		t.Fatal("identity/occupancy")
	}
	response, e := http.Get(server.URL + "/api/v1/devices/cellular-integration/cellular")
	if e != nil {
		t.Fatal(e)
	}
	var body struct {
		Data struct {
			Snapshot *device.Cellular `json:"snapshot"`
		} `json:"data"`
	}
	e = json.NewDecoder(bufio.NewReader(response.Body)).Decode(&body)
	response.Body.Close()
	if e != nil || body.Data.Snapshot == nil || body.Data.Snapshot.Status != "ok" {
		t.Fatal("HTTP snapshot", e)
	}
	// Simulate USB re-enumeration without touching the Probe or replacing its config.
	if e := os.Rename(filepath.Join(root, "sys/class/tty/ttyUSB2"), filepath.Join(root, "sys/class/tty/ttyUSB9")); e != nil {
		t.Fatal(e)
	}
	if e := os.Rename(filepath.Join(root, "dev/ttyUSB2"), filepath.Join(root, "dev/ttyUSB9")); e != nil {
		t.Fatal(e)
	}
	renamed := wait(func(v device.Snapshot) bool {
		if c := v.LatestSession.Cellular; c != nil {
			for _, p := range c.Ports {
				if p.Selected && p.Path == "/dev/ttyUSB9" && p.IMEI.Status == "ok" {
					return true
				}
			}
		}
		return false
	})
	if renamed.CurrentSession == nil || renamed.CurrentSession.ID != first.CurrentSession.ID || !renamed.CurrentSession.LastSeenAt.After(first.CurrentSession.LastSeenAt) {
		t.Fatal("heartbeat/session interrupted")
	}
	for _, p := range renamed.LatestSession.Cellular.Ports {
		if p.Selected && p.DeviceKey != chosen.DeviceKey {
			t.Fatal("unstable physical identity")
		}
	}
	// A later dialing process takes the former free endpoint: never force it away.
	occupied, e := os.OpenFile(available.path, os.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
	if e != nil {
		t.Fatal(e)
	}
	unavailable := wait(func(v device.Snapshot) bool {
		return v.LatestSession.Cellular != nil && v.LatestSession.Cellular.Status == "unavailable"
	})
	occupied.Close()
	if unavailable.Status != device.Online {
		t.Fatal("AT failure dropped control session")
	}
	wait(func(v device.Snapshot) bool {
		return v.LatestSession.Cellular != nil && v.LatestSession.Cellular.Status == "ok"
	})
	// Disabling through the actual configuration chain cancels collection and clears the snapshot.
	_, e = app.ProbeTemplates().Put(tpl.ID, tpl.Version, probetemplate.Input{Name: tpl.Name, Properties: map[string]probetemplate.Property{}, Monitoring: &probetemplate.Monitoring{}})
	if e != nil {
		t.Fatal(e)
	}
	adoptProbe(t, app, "cellular-integration", tpl.ID, nil)
	wait(func(v device.Snapshot) bool {
		return v.LatestSession.ConfigRevision > first.LatestSession.ConfigRevision && v.LatestSession.ConfigTemplate.CellularProbe == nil && v.LatestSession.Cellular == nil
	})
	before := available.count()
	time.Sleep(11 * time.Second)
	if available.count() != before {
		t.Fatal("disabled collector still issuing commands")
	}
}
