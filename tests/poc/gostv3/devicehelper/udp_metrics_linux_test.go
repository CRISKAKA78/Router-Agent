package main

import (
	"encoding/json"
	"errors"
	"net"
	"net/http/httptest"
	"testing"
)

func TestUDPMetricsBoundedEvidence(t *testing.T) {
	m := &udpMetrics{}
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}
	payload := make([]byte, 32000)
	for i := 0; i < 130; i++ {
		payload[0] = byte(i)
		m.record(payload, addr, len(payload), nil)
	}
	m.record(nil, addr, 0, errors.New("synthetic write error"))
	if m.packets != 131 || m.bytes != 130*32000 || m.written != 130*32000 || m.failures != 1 {
		t.Fatalf("bad totals: %+v", m)
	}
	if len(m.recent) != 128 || m.recent[0].Prefix[:2] != "03" || len(m.recent[0].Prefix) != 24 {
		t.Fatal("bounded identifiers incorrect")
	}
	last := m.recent[len(m.recent)-1]
	if last.Length != 0 || last.Prefix != "" || last.Error != "synthetic write error" {
		t.Fatalf("zero-byte/error evidence lost: %+v", last)
	}
}

func TestUDPMetricsOwnSocket(t *testing.T) {
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	m := newUDPMetrics(c)
	if m.inode == "" || m.recvBuffer <= 0 {
		t.Fatal("missing socket metadata")
	}
	w := httptest.NewRecorder()
	m.serveHTTP(w, httptest.NewRequest("GET", "/udp/stats", nil))
	var v map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v["inode"] != m.inode || v["socket_row"] == nil {
		t.Fatal("own socket evidence missing")
	}
}
