package probetemplate

import (
	"encoding/json"
	"testing"
)

func TestPhysicalCounterConfiguration(t *testing.T) {
	var s SwitchProbe
	if e := json.Unmarshal([]byte(`{"backend":"swconfig","ports":[{"id":"lan1","switch_id":"switch0","port":1,"role":"external"}],"counters":{"backend":"swconfig_mib","rx_field":"RxGoodByte","tx_field":"TxByte","bits":64,"basis":"RX good / TX bytes"}}`), &s); e != nil {
		t.Fatal(e)
	}
	if e := ValidatePresentation(nil, &s); e != nil {
		t.Fatal(e)
	}
	copy := CopySwitch(&s)
	copy.Counters.RXField = "changed"
	if s.Counters.RXField != "RxGoodByte" {
		t.Fatal("copy aliases counter configuration")
	}
	for _, change := range []func(*SwitchProbe){func(v *SwitchProbe) { v.Ports[0].SwitchID = "switch0;cmd" }, func(v *SwitchProbe) { v.Counters.Bits = 16 }, func(v *SwitchProbe) { v.Counters.RXField = "TxByte" }, func(v *SwitchProbe) { v.Counters.Backend = "guess" }, func(v *SwitchProbe) { v.Ports = nil }} {
		v := CopySwitch(&s)
		change(v)
		if ValidatePresentation(nil, v) == nil {
			t.Fatalf("invalid counters accepted: %+v", v)
		}
	}
}
