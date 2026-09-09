package probetemplate

import (
	"encoding/json"
	"testing"
)

func TestOptionalPortPreservesAbsentAndZero(t *testing.T) {
	for _, body := range []string{`{"backend":"dsa","ports":[{"id":"p1","system_name":"lan1"}]}`, `{"backend":"swconfig","ports":[{"id":"p1","switch_id":"switch0","port":0}]}`} {
		var s SwitchProbe
		if err := json.Unmarshal([]byte(body), &s); err != nil {
			t.Fatal(err)
		}
		if err := ValidatePresentation(nil, &s); err != nil {
			t.Fatal(err)
		}
		copy := CopySwitch(&s)
		if s.Ports[0].Port == nil {
			if copy.Ports[0].Port != nil {
				t.Fatal("absent port became zero")
			}
		} else {
			*copy.Ports[0].Port = 3
			if *s.Ports[0].Port != 0 {
				t.Fatal("copy aliases original port")
			}
		}
	}
	var invalid SwitchProbe
	_ = json.Unmarshal([]byte(`{"backend":"dsa","ports":[{"id":"p1","switch_id":"switch0"}]}`), &invalid)
	if ValidatePresentation(nil, &invalid) == nil {
		t.Fatal("chip matching without a port accepted")
	}
}
