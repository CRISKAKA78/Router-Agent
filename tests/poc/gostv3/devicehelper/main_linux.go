// Device-only bounded echo/PTY fixture. No physical serial, routes or product APIs.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	// Fail closed if the orchestrator disappears; this fixture is never a daemon.
	time.AfterFunc(12*time.Minute, func() { os.Exit(0) })
	tcp, err := net.Listen("tcp4", "127.0.0.1:19403")
	must(err)
	udp, err := net.ListenPacket("udp4", "127.0.0.1:19404")
	must(err)
	go func() {
		for {
			c, e := tcp.Accept()
			if e != nil {
				return
			}
			go func() { defer c.Close(); c.SetDeadline(time.Now().Add(2 * time.Minute)); io.Copy(c, c) }()
		}
	}()
	udpMetrics := newUDPMetrics(udp.(*net.UDPConn))
	http.HandleFunc("/udp/stats", udpMetrics.serveHTTP)
	go func() {
		b := make([]byte, 65536)
		for {
			n, a, e := udp.ReadFrom(b)
			if e != nil {
				return
			}
			written, err := udp.WriteTo(b[:n], a)
			udpMetrics.record(b[:n], a, written, err)
		}
	}()
	var mu sync.Mutex
	fds := make([]int, 3)
	paths := make([]string, 3)
	for i := range fds {
		fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
		must(err)
		defer syscall.Close(fd)
		var zero, number uint32
		_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x40045431, uintptr(unsafe.Pointer(&zero)))
		if e != 0 {
			panic(e)
		}
		_, _, e = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x80045430, uintptr(unsafe.Pointer(&number)))
		if e != 0 {
			panic(e)
		}
		fds[i], paths[i] = fd, fmt.Sprintf("/dev/pts/%d", number)
	}
	selectPTY := func(r *http.Request) (int, string) {
		i, _ := strconv.Atoi(r.URL.Query().Get("slot"))
		if i < 0 || i >= len(fds) {
			i = 0
		}
		return fds[i], paths[i]
	}
	http.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"ptys": paths, "pid": os.Getpid()})
	})
	http.HandleFunc("/pty/read", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		fd, _ := selectPTY(r)
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		if n < 1 || n > 65536 {
			http.Error(w, "invalid n", 400)
			return
		}
		ms, _ := strconv.Atoi(r.URL.Query().Get("ms"))
		if ms < 1 || ms > 2000 {
			ms = 1000
		}
		b := make([]byte, 0, n)
		until := time.Now().Add(time.Duration(ms) * time.Millisecond)
		for len(b) < n && time.Now().Before(until) {
			tmp := make([]byte, n-len(b))
			k, e := syscall.Read(fd, tmp)
			if k > 0 {
				b = append(b, tmp[:k]...)
			}
			if e != nil && e != syscall.EAGAIN && e != syscall.EINTR && e != syscall.EIO {
				http.Error(w, e.Error(), 500)
				return
			}
			if k <= 0 {
				time.Sleep(5 * time.Millisecond)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"data": b, "length": len(b)})
	})
	http.HandleFunc("/pty/write", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST only", 405)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		var v struct {
			Data []byte `json:"data"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 90000)).Decode(&v) != nil || len(v.Data) > 65536 {
			http.Error(w, "invalid data", 400)
			return
		}
		fd, _ := selectPTY(r)
		n, e := syscall.Write(fd, v.Data)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]int{"written": n})
	})
	http.HandleFunc("/pty/termios", func(w http.ResponseWriter, r *http.Request) {
		_, path := selectPTY(r)
		f, e := syscall.Open(path, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		defer syscall.Close(f)
		var t syscall.Termios
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(f), syscall.TCGETS, uintptr(unsafe.Pointer(&t)))
		if errno != 0 {
			http.Error(w, errno.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]uint32{"cflag": t.Cflag, "speed_bits": t.Cflag & 0x100f})
	})
	s := &http.Server{Addr: "127.0.0.1:19402", ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	must(s.ListenAndServe())
}
