package gateway

import (
	"routerprobe/internal/protocol"
	"testing"
	"time"
)

func TestHeartbeatValidity(t *testing.T) {
	for _, tc := range []struct {
		payload    string
		value      uint64
		known, bad bool
	}{
		{`{"uptime":123,"running_tasks":0}`, 0, false, true},
		{`{"uptime":0,"running_tasks":0}`, 0, false, true},
		{`{"uptime":0,"uptime_valid":true,"running_tasks":0}`, 0, true, false},
		{`{"uptime":0,"uptime_valid":false,"running_tasks":0}`, 0, false, false},
		{`{"uptime":1,"uptime_valid":false,"running_tasks":0}`, 0, false, true},
		{`{"uptime":0,"uptime_valid":null,"running_tasks":0}`, 0, false, true},
		{`{"uptime":0,"uptime_valid":1,"running_tasks":0}`, 0, false, true},
		{`{"uptime":0,"uptime_valid":"true","running_tasks":0}`, 0, false, true},
		{`{"uptime":-1,"running_tasks":0}`, 0, false, true},
		{`{"uptime":1.5,"running_tasks":0}`, 0, false, true},
		{`{"uptime":9223372036854775807,"uptime_valid":true,"running_tasks":65535,"future":true}`, 9223372036854775807, true, false},
		{`{"uptime":9223372036854775808,"running_tasks":0}`, 0, false, true},
		{`{"uptime":1,"uptime_valid":true,"uptime_valid":false,"running_tasks":0}`, 0, false, true},
	} {
		value, err := parseHeartbeat([]byte(tc.payload))
		if (err != nil) != tc.bad || (err == nil && ((value != nil) != tc.known || (value != nil && *value != tc.value))) {
			t.Fatalf("%s => %v %v", tc.payload, value, err)
		}
	}
}

func TestGatewayRetainsHeartbeatAndRejectsStaleReader(t *testing.T) {
	s, address := startTestServer(t)
	c := deviceConnect(t, s, address, validRegister("uptime"))
	s.mu.Lock()
	old := s.sessions["uptime"]
	s.mu.Unlock()
	writeJSONFrame(t, c, protocol.TypeHeartbeat, 2, `{"uptime":60,"uptime_valid":true,"running_tasks":0}`)
	if readFrame(t, c).Header.Type != protocol.TypeHeartbeatAck {
		t.Fatal("heartbeat not ACKed")
	}
	v := deviceGet(t, s, "uptime")
	if v.CurrentSession.Runtime == nil || *v.CurrentSession.Runtime.UptimeSeconds != 60 || v.CurrentSession.Runtime.ReportedAt.IsZero() {
		t.Fatal("sample discarded")
	}
	deviceConnect(t, s, address, validRegister("uptime"))
	seconds := uint64(888)
	s.recordHeartbeat(old, &seconds)
	v = deviceGet(t, s, "uptime")
	if v.CurrentSession.Runtime != nil || v.LastSeenAt.After(time.Now()) {
		t.Fatal("stale reader changed current runtime")
	}
}
