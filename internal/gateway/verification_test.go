package gateway

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"routerprobe/internal/protocol"
)

func TestHeartbeatNumberType(t *testing.T) {
	for _, value := range []string{`"0.21"`, `null`, `true`, `[]`, `{}`, `-1`, `1e999`} {
		if _, err := parseHeartbeat([]byte(`{"uptime":0,"uptime_valid":true,"running_tasks":0,"load1":` + value + `}`)); err == nil {
			t.Errorf("accepted load1=%s", value)
		}
	}
	for _, value := range []string{`0`, `0.21`, `1e2`} {
		if _, err := parseHeartbeat([]byte(`{"uptime":0,"uptime_valid":true,"running_tasks":0,"load1":` + value + `}`)); err != nil {
			t.Errorf("load1=%s: %v", value, err)
		}
	}
}

func TestServerInvalidProtocolConverges(t *testing.T) {
	s, address := startTestServer(t)
	cases := []struct {
		name    string
		kind    uint8
		flags   uint16
		payload string
		mutate  func([]byte) []byte
	}{
		{"magic", 3, 0, `{}`, func(b []byte) []byte { b[0] = 'X'; return b }},
		{"version", 3, 0, `{}`, func(b []byte) []byte { b[4] = 2; return b }},
		{"zero-id", 3, 0, `{}`, func(b []byte) []byte { binary.BigEndian.PutUint64(b[12:], 0); return b }},
		{"duplicate-id", 3, 0, `{}`, func(b []byte) []byte { binary.BigEndian.PutUint64(b[12:], 1); return b }},
		{"gap-id", 3, 0, `{}`, func(b []byte) []byte { binary.BigEndian.PutUint64(b[12:], 3); return b }},
		{"limit-header-only", 3, 0, `{}`, func(b []byte) []byte { binary.BigEndian.PutUint32(b[8:], protocol.MaxControlPayload+1); return b[:20] }},
		{"invalid-json", 3, 0, `{`, nil},
		{"array", 3, 0, `[]`, nil},
		{"null", 3, 0, `null`, nil},
		{"invalid-utf8", 3, 0, "{\"future\":\"\xff\"}", nil},
		{"load-string", 3, 0, `{"uptime":0,"uptime_valid":true,"running_tasks":0,"load1":"0"}`, nil},
		{"ack-missing-response", 0x11, 0, `{}`, nil},
		{"result-response", 0x12, 1, `{}`, nil},
		{"chunk-no-binary", 0x31, 0, `{}`, nil},
		{"unknown-type", 0x13, 0, `{}`, nil},
	}
	for bit := 0; bit < 16; bit++ {
		cases = append(cases, struct {
			name    string
			kind    uint8
			flags   uint16
			payload string
			mutate  func([]byte) []byte
		}{fmt.Sprintf("heartbeat-flag-%d", bit), 3, 1 << bit, `{"uptime":0,"uptime_valid":true,"running_tasks":0}`, nil})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, e := net.Dial("tcp", address)
			if e != nil {
				t.Fatal(e)
			}
			defer c.Close()
			c.SetDeadline(time.Now().Add(2 * time.Second))
			writeJSONFrame(t, c, 1, 1, `{"device_id":"invalid","probe_version":"1","arch":"x86_64","boot_id":"boot","capabilities":["managed_config_v1","telemetry_v2"]}`)
			// Consume REGISTER_ACK before injecting an online violation.
			readFrame := func() (protocol.Frame, error) {
				var h [20]byte
				if _, e := io.ReadFull(c, h[:]); e != nil {
					return protocol.Frame{}, e
				}
				header, e := protocol.DecodeHeader(h[:], protocol.MaxControlPayload)
				if e != nil {
					return protocol.Frame{}, e
				}
				b := make([]byte, header.PayloadLen)
				_, e = io.ReadFull(c, b)
				return protocol.Frame{Header: header, Payload: b}, e
			}
			if f, e := readFrame(); e != nil || f.Header.Type != 2 {
				t.Fatal("register", e)
			}
			data, _ := protocol.EncodeFrame(protocol.Frame{Header: protocol.Header{Version: 1, Type: tc.kind, Flags: tc.flags, MessageID: 2}, Payload: []byte(tc.payload)})
			if tc.mutate != nil {
				data = tc.mutate(data)
			}
			if _, e = c.Write(data); e != nil {
				t.Fatal(e)
			}
			for {
				f, e := readFrame()
				if e != nil {
					if x, ok := e.(net.Error); ok && x.Timeout() {
						t.Fatal("connection did not close")
					}
					break
				}
				if f.Header.Type != protocol.TypeError || f.Header.Flags != protocol.FlagResponse {
					t.Fatalf("unexpected response %+v", f.Header)
				}
				var r struct {
					ReplyTo uint64 `json:"reply_to"`
				}
				if json.Unmarshal(f.Payload, &r) != nil || r.ReplyTo != 2 {
					t.Fatalf("invalid ERROR reply: %s", f.Payload)
				}
			}
		})
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		n := len(s.connections) + len(s.sessions)
		s.mu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("connection/session leak: %d", n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestControlUnicodeContract(t *testing.T) {
	base := `{"device_id":"DEVICE","probe_version":"1","arch":"x86_64","boot_id":"boot","capabilities":["managed_config_v1","telemetry_v2"]}`
	for _, value := range []string{`\ud800`, `\udc00`, `\ud800x`, `\ud800\u0041`, string([]byte{0xff})} {
		if _, err := parseRegister([]byte(strings.Replace(base, "DEVICE", value, 1))); err == nil {
			t.Errorf("accepted invalid Unicode %q", value)
		}
	}
	for _, value := range []string{`设备`, `\ud83d\ude80`, `\\ud800`, `\ufffd`} {
		if _, err := parseRegister([]byte(strings.Replace(base, "DEVICE", value, 1))); err != nil {
			t.Errorf("valid Unicode %q: %v", value, err)
		}
	}
	if _, err := decodeObject([]byte(`{"unknown":{"nested":"\ud800"}}`)); err == nil {
		t.Fatal("invalid Unicode in extension accepted")
	}
}
