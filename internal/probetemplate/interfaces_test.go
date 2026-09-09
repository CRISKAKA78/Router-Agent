package probetemplate

import (
	"encoding/json"
	"testing"
)

func TestInterfaceConfigurationRoundTrip(t *testing.T) {
	for _, names := range []string{"eth0,br0,ppp0", ""} {
		var in Input
		raw := `{"name":"network","properties":{"x":{"name":"X","command":"echo x","timeout_seconds":5}},"monitoring":{"network_interfaces":"` + names + `","egress_seconds":10}}`
		if err := json.Unmarshal([]byte(raw), &in); err != nil {
			t.Fatal(err)
		}
		if err := Validate(in); err != nil {
			t.Fatal(err)
		}
		copied := normalize(in)
		*in.Monitoring.NetworkInterfaces = "changed"
		if *copied.Monitoring.NetworkInterfaces != names || copied.Monitoring.Egress != 10 {
			t.Fatal("configuration aliased")
		}
	}
	for _, names := range []string{"eth0,eth0", " eth0", "eth*", "../eth0", ",", "eth0,", "1234567890123456"} {
		var in Input
		json.Unmarshal([]byte(`{"name":"network","properties":{"x":{"name":"X","command":"echo x","timeout_seconds":5}},"monitoring":{"network_interfaces":"`+names+`"}}`), &in)
		if Validate(in) == nil {
			t.Fatal("accepted", names)
		}
	}
	var m Monitoring
	if json.Unmarshal([]byte(`{}`), &m) != nil || m.Egress != 600 || m.NetworkInterfaces != nil {
		t.Fatal("legacy defaults", m)
	}
	if json.Unmarshal([]byte(`{"network_interfaces":null}`), &m) == nil {
		t.Fatal("null accepted")
	}
}
