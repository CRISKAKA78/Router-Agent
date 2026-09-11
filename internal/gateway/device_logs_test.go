package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"routerprobe/internal/devicelog"
	"routerprobe/internal/protocol"
	"strings"
	"testing"
	"time"
)

func logPeer(t *testing.T) (*Server, net.Conn, string) {
	s, address := startTestServer(t)
	c, e := net.Dial("tcp", address)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	writeJSONFrame(t, c, protocol.TypeRegister, 1, strings.Replace(validRegister("log-device"), `"telemetry_v2"`, `"telemetry_v2","device_logs_v1"`, 1))
	ack := readFrame(t, c)
	var v struct {
		Session string `json:"session_id"`
	}
	json.Unmarshal(ack.Payload, &v)
	deviceEvent(t, s, EventOnline)
	return s, c, v.Session
}
func TestLogReadIsSessionBoundAndDoesNotCreateTasks(t *testing.T) {
	s, c, session := logPeer(t)
	q := devicelog.Query{Operation: "live"}
	var raw json.RawMessage
	var err error
	done := make(chan struct{})
	go func() { raw, err = s.QueryDeviceLog(context.Background(), "log-device", session, q); close(done) }()
	request := readFrame(t, c)
	var wire devicelog.Query
	if e := json.Unmarshal(request.Payload, &wire); e != nil || request.Header.Type != protocol.TypeEvent || wire.Event != "device_log_query" {
		t.Fatal(e, string(request.Payload))
	}
	writeJSONFrame(t, c, protocol.TypeEvent, 2, fmt.Sprintf(`{"event":"device_log_reply","request_id":%q,"data":{"state":"ok","start":0,"offset":1,"generation":"1:2","gap":false,"data_hex":"41"},"error":""}`, wire.RequestID))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("query did not finish")
	}
	if err != nil || !strings.Contains(string(raw), "41") {
		t.Fatal(string(raw), err)
	}
	if _, total := s.tasks.List("log-device", "", 0, 100); total != 0 {
		t.Fatal("live query created task history")
	}
	if _, e := s.QueryDeviceLog(context.Background(), "log-device", session, q); !errors.Is(e, devicelog.ErrBusy) {
		t.Fatal(e)
	}
	if _, e := s.QueryDeviceLog(context.Background(), "log-device", "old-session", q); !errors.Is(e, ErrSessionChanged) {
		t.Fatal(e)
	}
}
func TestLogMutationWireAndResend(t *testing.T) {
	s, c, session := logPeer(t)
	id, e := s.CreateDeviceLog(context.Background(), "log-device", session, devicelog.Params{"action": "enable_live"})
	if e != nil {
		t.Fatal(e)
	}
	f := readFrame(t, c)
	if f.Header.Type != protocol.TypeTask || !strings.Contains(string(f.Payload), `"type":"device_logs"`) || !strings.Contains(string(f.Payload), `"action":"enable_live"`) {
		t.Fatal(string(f.Payload))
	}
	if e = s.ResendTask(context.Background(), id); e != nil {
		t.Fatal(e)
	}
	again := readFrame(t, c)
	if string(again.Payload) != string(f.Payload) {
		t.Fatal("resend changed immutable task")
	}
}
func TestLogQueryDisconnectReleasesWaiter(t *testing.T) {
	s, c, session := logPeer(t)
	done := make(chan error, 1)
	go func() {
		_, e := s.QueryDeviceLog(context.Background(), "log-device", session, devicelog.Query{Operation: "status"})
		done <- e
	}()
	readFrame(t, c)
	c.Close()
	select {
	case e := <-done:
		if !errors.Is(e, ErrSessionChanged) {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnected query leaked")
	}
}
