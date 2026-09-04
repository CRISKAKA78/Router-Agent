package gateway

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"routerprobe/internal/protocol"
)

func startTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	server, err := New(Config{
		HeartbeatInterval: 10 * time.Second,
		Logger:            log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		select {
		case err := <-serveDone:
			if err != nil {
				t.Errorf("Serve: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Serve did not stop")
		}
	})
	return server, listener.Addr().String()
}

func writeJSONFrame(t *testing.T, conn net.Conn, messageType uint8, messageID uint64, payload string) {
	t.Helper()
	err := protocol.WriteFrame(conn, protocol.Frame{
		Header:  protocol.Header{Version: protocol.Version1, Type: messageType, MessageID: messageID},
		Payload: []byte(payload),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func readFrame(t *testing.T, conn net.Conn) protocol.Frame {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	headerBytes := make([]byte, protocol.HeaderSize)
	if _, err := io.ReadFull(conn, headerBytes); err != nil {
		t.Fatal(err)
	}
	header, err := protocol.DecodeHeader(headerBytes, protocol.MaxControlPayload)
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, header.PayloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatal(err)
	}
	return protocol.Frame{Header: header, Payload: payload}
}

func validRegister(deviceID string) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"device_id":     deviceID,
		"probe_version": "1.0.0",
		"arch":          "x86_64",
		"boot_id":       "test-boot",
		"capabilities":  []string{},
	})
	return string(payload)
}

func TestRegisterAndHeartbeatReplyTo(t *testing.T) {
	server, address := startTestServer(t)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	writeJSONFrame(t, conn, protocol.TypeRegister, 1, validRegister("device-legal"))
	ack := readFrame(t, conn)
	if ack.Header.Type != protocol.TypeRegisterAck || ack.Header.Flags != protocol.FlagResponse || ack.Header.MessageID != 1 {
		t.Fatalf("REGISTER_ACK header = %#v", ack.Header)
	}
	var registerResponse registerAckSuccess
	if err := json.Unmarshal(ack.Payload, &registerResponse); err != nil {
		t.Fatal(err)
	}
	if !registerResponse.Success || registerResponse.ReplyTo != 1 || registerResponse.SessionID == "" || registerResponse.HeartbeatInterval != 10 {
		t.Fatalf("REGISTER_ACK = %#v", registerResponse)
	}

	select {
	case event := <-server.Events():
		if event.Type != EventOnline || event.DeviceID != "device-legal" || event.SessionID != registerResponse.SessionID {
			t.Fatalf("online event = %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("online event timeout")
	}

	writeJSONFrame(t, conn, protocol.TypeHeartbeat, 2, `{"uptime":123,"running_tasks":0}`)
	heartbeatResponseFrame := readFrame(t, conn)
	if heartbeatResponseFrame.Header.Type != protocol.TypeHeartbeatAck || heartbeatResponseFrame.Header.Flags != protocol.FlagResponse || heartbeatResponseFrame.Header.MessageID != 2 {
		t.Fatalf("HEARTBEAT_ACK header = %#v", heartbeatResponseFrame.Header)
	}
	var heartbeatResponse heartbeatAck
	if err := json.Unmarshal(heartbeatResponseFrame.Payload, &heartbeatResponse); err != nil {
		t.Fatal(err)
	}
	if heartbeatResponse.ReplyTo != 2 || heartbeatResponse.ServerTime < 0 {
		t.Fatalf("HEARTBEAT_ACK = %#v", heartbeatResponse)
	}
}

func TestRegisterValidationFailure(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "missing required field", payload: `{"probe_version":"1.0.0","arch":"x86_64","boot_id":"boot","capabilities":[]}`},
		{name: "field out of range", payload: validRegister(strings.Repeat("d", 129))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, address := startTestServer(t)
			conn, err := net.Dial("tcp", address)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			writeJSONFrame(t, conn, protocol.TypeRegister, 1, test.payload)
			ack := readFrame(t, conn)
			if ack.Header.Type != protocol.TypeRegisterAck || ack.Header.Flags != protocol.FlagResponse {
				t.Fatalf("header = %#v", ack.Header)
			}
			var response registerAckFailure
			if err := json.Unmarshal(ack.Payload, &response); err != nil {
				t.Fatal(err)
			}
			if response.Success || response.ReplyTo != 1 || response.ErrorCode != "INVALID_REGISTER" || response.RetryAfter < 1 {
				t.Fatalf("REGISTER_ACK failure = %#v", response)
			}
		})
	}
}
