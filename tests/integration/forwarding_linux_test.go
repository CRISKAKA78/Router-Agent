package integration

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"routerprobe/internal/forwarding"
	"routerprobe/internal/gateway"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Requires the isolated network namespace from the documented verification command.
func TestForwardingProduct(t *testing.T) {
	gost := os.Getenv("RMP_FORWARDING_GOST")
	agent := os.Getenv("RMP_FORWARDING_AGENT")
	if gost == "" || agent == "" || os.Getenv("RMP_FORWARDING_TEST_NET") != "1" {
		t.Skip("isolated product backend test requires GOST, agent and test network")
	}
	for _, args := range [][]string{{"link", "add", "fwdtest", "type", "dummy"}, {"addr", "add", "198.18.1.1/24", "dev", "fwdtest"}, {"addr", "add", "198.18.1.2/24", "dev", "fwdtest"}, {"link", "set", "fwdtest", "up"}} {
		if b, e := exec.Command("ip", args...).CombinedOutput(); e != nil {
			t.Fatal(string(b), e)
		}
	}
	defer exec.Command("ip", "link", "del", "fwdtest").Run()
	g, e := gateway.New(gateway.Config{HeartbeatInterval: 10 * time.Second, Logger: log.New(io.Discard, "", 0)})
	if e != nil {
		t.Fatal(e)
	}
	defer g.Close()
	s, e := forwarding.New(forwarding.Config{Binary: gost, BindHost: "127.0.0.1", Host: "127.0.0.1", PortFirst: 35200, PortLast: 35299}, g)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	g.SetForwardingStatus(s.Report)
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	go g.Serve(l)
	var logs lockedBuffer
	p := startProbe(t, probeBinary(t), l.Addr().String(), "forward-router", &logs)
	waitOnline(t, g.Events(), "forward-router", 5*time.Second)
	inv, e := s.Inventory(context.Background(), "forward-router")
	if e != nil || !inv.Backend {
		t.Fatalf("inventory %+v %v logs=%s", inv, e, logs.String())
	}
	t.Logf("product inventory: %+v", inv)
	for _, proto := range []string{"tcp", "udp"} {
		t.Run(proto, func(t *testing.T) {
			var port int
			var source = make(chan string, 8)
			if proto == "tcp" {
				target, e := net.Listen("tcp", "198.18.1.2:0")
				if e != nil {
					t.Fatal(e)
				}
				defer target.Close()
				port = target.Addr().(*net.TCPAddr).Port
				go func() {
					for {
						n, e := target.Accept()
						if e != nil {
							return
						}
						host, _, _ := net.SplitHostPort(n.RemoteAddr().String())
						source <- host
						go func() { defer n.Close(); io.Copy(n, n) }()
					}
				}()
			} else {
				target, e := net.ListenPacket("udp", "198.18.1.2:0")
				if e != nil {
					t.Fatal(e)
				}
				defer target.Close()
				port = target.LocalAddr().(*net.UDPAddr).Port
				go func() {
					b := make([]byte, 65536)
					for {
						n, a, e := target.ReadFrom(b)
						if e != nil {
							return
						}
						host, _, _ := net.SplitHostPort(a.String())
						select {
						case source <- host:
						default:
						}
						target.WriteTo(b[:n], a)
					}
				}()
			}
			zero := 0
			v, e := s.Create(context.Background(), forwarding.Request{DeviceID: "forward-router", Kind: "lan", Protocol: proto, Interface: "fwdtest", TargetIP: "198.18.1.2", TargetPort: port, LeaseMinutes: &zero})
			if e != nil {
				t.Fatal(e)
			}
			deadline := time.Now().Add(15 * time.Second)
			for {
				v.Mapping, _ = s.Get(v.Mapping.ID)
				if v.Mapping.State == "active" {
					break
				}
				if v.Mapping.State == "failed" || time.Now().After(deadline) {
					t.Fatalf("mapping %+v logs=%s", v.Mapping, logs.String())
				}
				time.Sleep(30 * time.Millisecond)
			}
			n, e := net.Dial(proto, net.JoinHostPort(v.Mapping.Host, strconv.Itoa(v.Mapping.Port)))
			if e != nil {
				t.Fatal(e)
			}
			defer n.Close()
			n.SetDeadline(time.Now().Add(5 * time.Second))
			data := []byte("product-forward\x00\xff")
			n.Write(data)
			out := make([]byte, len(data))
			if _, e = io.ReadFull(n, out); e != nil || string(out) != string(data) {
				t.Fatal(out, e)
			}
			select {
			case src := <-source:
				if src != "198.18.1.1" {
					t.Fatal("incorrect proxy source", src)
				}
			case <-time.After(time.Second):
				t.Fatal("no source evidence")
			}
			if proto == "udp" {
				n.Write([]byte{})
				b := make([]byte, 1)
				if count, e := n.Read(b); e != nil || count != 0 {
					t.Fatal("empty UDP lost", count, e)
				}
			}
			if e = s.CloseMapping(v.Mapping.ID); e != nil {
				t.Fatal(e)
			}
			deadline = time.Now().Add(4 * time.Second)
			for {
				v.Mapping, _ = s.Get(v.Mapping.ID)
				if v.Mapping.DeviceReleased {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("device close unconfirmed", v.Mapping)
				}
				time.Sleep(20 * time.Millisecond)
			}
		})
	}

	t.Run("serial_registration_pty", func(t *testing.T) {
		fd, e := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
		if e != nil {
			t.Fatal(e)
		}
		master := os.NewFile(uintptr(fd), "test-pty")
		defer master.Close()
		var zero, number uint32
		if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x40045431, uintptr(unsafe.Pointer(&zero))); e != 0 {
			t.Fatal(e)
		}
		if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x80045430, uintptr(unsafe.Pointer(&number))); e != 0 {
			t.Fatal(e)
		}
		node := "/dev/ttyUSBforwardingtest"
		f, e := os.OpenFile(node, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if e != nil {
			t.Fatal(e)
		}
		f.Close()
		defer os.Remove(node)
		if e = syscall.Mount(fmt.Sprintf("/dev/pts/%d", number), node, "", syscall.MS_BIND, ""); e != nil {
			t.Fatal(e)
		}
		defer syscall.Unmount(node, 0)
		lease := 0
		v, e := s.Create(context.Background(), forwarding.Request{DeviceID: "forward-router", Kind: "serial", Protocol: "tcp", Serial: node, Baud: 115200, LeaseMinutes: &lease})
		if e != nil {
			t.Fatal(e)
		}
		deadline := time.Now().Add(15 * time.Second)
		for {
			v.Mapping, _ = s.Get(v.Mapping.ID)
			if v.Mapping.State == "active" {
				break
			}
			if v.Mapping.State == "failed" || time.Now().After(deadline) {
				t.Fatal(v.Mapping)
			}
			time.Sleep(30 * time.Millisecond)
		}
		addr := net.JoinHostPort(v.Mapping.Host, strconv.Itoa(v.Mapping.Port))
		bad, e := net.Dial("tcp", addr)
		if e != nil {
			t.Fatal(e)
		}
		bad.SetDeadline(time.Now().Add(time.Second))
		io.WriteString(bad, "AUTH "+strings.Repeat("b", 64)+"\r\nGARBAGE")
		line, _ := bufio.NewReader(bad).ReadString('\n')
		bad.Close()
		if line != "ERR AUTH\r\n" {
			t.Fatal(line)
		}
		master.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		buf := make([]byte, 128)
		if n, _ := master.Read(buf); n != 0 {
			t.Fatal("unauthorized serial write", buf[:n])
		}
		good, e := net.Dial("tcp", addr)
		if e != nil {
			t.Fatal(e)
		}
		defer good.Close()
		good.SetDeadline(time.Now().Add(4 * time.Second))
		reader := bufio.NewReader(good)
		io.WriteString(good, v.Registration+"product-serial")
		line, e = reader.ReadString('\n')
		if e != nil || line != "OK\r\n" {
			t.Fatal(line, e)
		}
		master.SetReadDeadline(time.Now().Add(4 * time.Second))
		want := make([]byte, len("product-serial"))
		if _, e = io.ReadFull(master, want); e != nil || string(want) != "product-serial" {
			t.Fatal(string(want), e)
		}
		master.Write([]byte("reply"))
		reply := make([]byte, 5)
		if _, e = io.ReadFull(reader, reply); e != nil || string(reply) != "reply" {
			t.Fatal(string(reply), e)
		}
		if e = s.CloseMapping(v.Mapping.ID); e != nil {
			t.Fatal(e)
		}
		deadline = time.Now().Add(4 * time.Second)
		for {
			v.Mapping, _ = s.Get(v.Mapping.ID)
			if v.Mapping.DeviceReleased {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("serial not released", v.Mapping)
			}
			time.Sleep(20 * time.Millisecond)
		}
		v, e = s.Create(context.Background(), forwarding.Request{DeviceID: "forward-router", Kind: "serial", Protocol: "tcp", Serial: node, LeaseMinutes: &lease})
		if e != nil {
			t.Fatal(e)
		}
		p.Process.Kill()
		p.Wait()
		deadline = time.Now().Add(4 * time.Second)
		for {
			v.Mapping, _ = s.Get(v.Mapping.ID)
			if v.Mapping.Released {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("unlimited mapping survived session death", v.Mapping)
			}
			time.Sleep(20 * time.Millisecond)
		}
	})

}
