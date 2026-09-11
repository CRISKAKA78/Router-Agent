package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
)

// Counters are test-fixture evidence, not product telemetry. Retain at most 128
// synthetic packet identifiers; never copy whole datagram payloads into logs.
type udpPacket struct {
	Prefix  string `json:"prefix"`
	Length  int    `json:"length"`
	Source  string `json:"source"`
	Written int    `json:"written"`
	Error   string `json:"error,omitempty"`
}
type udpMetrics struct {
	mu                                sync.Mutex
	packets, bytes, written, failures uint64
	recent                            []udpPacket
	inode                             string
	recvBuffer                        int
}

func newUDPMetrics(c *net.UDPConn) *udpMetrics {
	m := &udpMetrics{}
	raw, e := c.SyscallConn()
	must(e)
	must(raw.Control(func(fd uintptr) {
		link, e := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd))
		must(e)
		m.inode = strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
		m.recvBuffer, e = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
		must(e)
	}))
	return m
}
func (m *udpMetrics) record(b []byte, a net.Addr, written int, err error) {
	length := len(b)
	if len(b) > 12 {
		b = b[:12]
	}
	p := udpPacket{Prefix: hex.EncodeToString(b), Length: length, Source: a.String(), Written: written}
	if err != nil {
		p.Error = err.Error()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.packets++
	m.bytes += uint64(length)
	m.written += uint64(written)
	if err != nil {
		m.failures++
	}
	if len(m.recent) == 128 {
		copy(m.recent, m.recent[1:])
		m.recent = m.recent[:127]
	}
	m.recent = append(m.recent, p)
}
func (m *udpMetrics) serveHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	v := map[string]any{"packets": m.packets, "bytes": m.bytes, "written": m.written, "write_errors": m.failures, "recent": append([]udpPacket(nil), m.recent...), "inode": m.inode, "recv_buffer": m.recvBuffer}
	m.mu.Unlock()
	// Only the fixture's own socket row, not unrelated device peers.
	if b, e := os.ReadFile("/proc/net/udp"); e == nil {
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) > 9 && f[9] == m.inode {
				v["socket_row"] = line
				break
			}
		}
	}
	json.NewEncoder(w).Encode(v)
}
