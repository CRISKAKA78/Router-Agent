package gateway

import (
	"encoding/json"
	"net"
	"routerprobe/internal/protocol"
	"testing"
)

func TestCurrentCapabilitiesRequired(t *testing.T) {
	for _, capabilities := range [][]string{nil, {"telemetry_v2"}, {"managed_config_v1"}, {"telemetry_v1", "managed_config_v1"}} {
		server, address := startTestServer(t)
		conn, err := net.Dial("tcp", address)
		if err != nil {
			t.Fatal(err)
		}
		var registration map[string]any
		if err = json.Unmarshal([]byte(validRegister("outdated")), &registration); err != nil {
			t.Fatal(err)
		}
		registration["capabilities"] = capabilities
		payload, _ := json.Marshal(registration)
		writeJSONFrame(t, conn, protocol.TypeRegister, 1, string(payload))
		frame := readFrame(t, conn)
		conn.Close()
		var ack map[string]any
		if json.Unmarshal(frame.Payload, &ack) != nil || ack["success"] != false || frame.Header.Type != protocol.TypeRegisterAck {
			t.Fatal(string(frame.Payload))
		}
		if len(server.Devices().List()) != 0 {
			t.Fatal("unsupported registration created device")
		}
	}
}
