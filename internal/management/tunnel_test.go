package management

import (
	"net"
	"routerprobe/internal/tunnel"
	"testing"
	"time"
)

func TestManagementTunnelCompositionAndRestart(t *testing.T) {
	dir := t.TempDir()
	c := Config{RepositoryDirectory: dir, Tunnel: &tunnel.Config{DataListen: "127.0.0.1:0"}}
	s, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	if s.Maintenance() == nil {
		t.Fatal("missing maintenance service")
	}
	address := s.Maintenance().DataAddress()
	data, e := net.Dial("tcp", address)
	if e != nil {
		t.Fatal(e)
	}
	defer data.Close()
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	data.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	if _, e = data.Read(b[:]); e == nil {
		t.Fatal("unpaired data socket survived close")
	}
	s, e = New(c)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if len(s.Maintenance().List()) != 0 {
		t.Fatal("maintenance restored")
	}
	if e = s.Maintenance().CloseMaintenance("old-evicted-id"); e != nil {
		t.Fatal("close not idempotent after restart", e)
	}
}
