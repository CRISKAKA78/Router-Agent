package device

import (
	"routerprobe/internal/probetemplate"
	"testing"
	"time"
)

func TestFM160SignalProjection(t *testing.T) {
	p := CellularPort{Profile: "fibocom-fm160-v1", Queries: []AtIdentity{{Command: "AT+CESQ", Status: "ok", Value: "+CESQ: 99,99,255,255,14,33,255,255,255"}}}
	EnrichCellular(&p)
	if len(p.Signals) != 2 || p.Signals[0].Value != -107.5 || p.Signals[1].Value != -12.75 {
		t.Fatal(p.Signals)
	}
	if p.Signals[0].Minimum != -140 || p.Signals[0].Maximum != -44 || p.Signals[1].Minimum != -19.5 || p.Signals[1].Maximum != -3 {
		t.Fatal("manual LTE scale", p.Signals)
	}
	p.Queries[0].Value = "+CESQ: 99,99,255,255,255,255,126,80,127"
	EnrichCellular(&p)
	if len(p.Signals) != 3 || p.Signals[0].Value != 19.75 || p.Signals[1].Unit != "dBm" || p.Signals[1].Value != -76.5 || p.Signals[2].Value != 40 {
		t.Fatal("NR codes/physical quantities", p.Signals)
	}
	p.Queries[0].Value = "+CESQ: 99,99,255,255,0,0"
	EnrichCellular(&p)
	if len(p.Signals) != 2 || p.Signals[0].Qualifier != "<" {
		t.Fatal("lower censoring", p.Signals)
	}
	for _, v := range []string{"+CESQ: 99,99,255,255,255,255,255,255,255", "+CESQ: -1,99,255,255,14,33", "+CESQ: 99,99,255,255,14,NaN", "+CESQ: 99,99,255,255,14,33\n+CEREG: 1"} {
		p.Queries[0].Value = v
		EnrichCellular(&p)
		if len(p.Signals) != 0 {
			t.Fatal("invalid must not chart", v)
		}
	}
	p.Queries[0].Status = "timeout"
	p.Queries[0].Value = ""
	EnrichCellular(&p)
	if len(p.Signals) != 0 {
		t.Fatal("timeout carried stale signal")
	}
}
func TestFM160FieldsAndSnapshotCopy(t *testing.T) {
	p := CellularPort{Profile: "fibocom-fm160-v1", ATI: AtIdentity{Status: "ok", Value: "Manufacturer: Fibocom Wireless Inc.\nModel: FM160-CN\nRevision: test"}, Queries: []AtIdentity{{Command: "AT+CPIN?", Status: "ok", Value: "+CPIN: READY"}, {Command: "AT+COPS?", Status: "ok", Value: `+COPS: 0,2,"46001",13`}, {Command: "AT+CCID", Status: "ok", Value: "+CCID: 89860000191988755034"}}}
	EnrichCellular(&p)
	fields := map[string]string{}
	for _, f := range p.Fields {
		fields[f.Key] = f.Value
	}
	if fields["sim"] != "就绪" || fields["mnc"] != "01" || fields["iccid"] != "89860000191988755034" {
		t.Fatal(fields)
	}
	n := Cellular{Ports: []CellularPort{p}}
	copied := copyCellular(&n)
	copied.Ports[0].Fields[0].Value = "bad"
	copied.Ports[0].Queries[0].Value = "bad"
	if n.Ports[0].Fields[0].Value == "bad" || n.Ports[0].Queries[0].Value == "bad" {
		t.Fatal("alias")
	}
	s, _ := New(2)
	at := time.Now()
	s.Publish(Registration{DeviceID: "d"}, "s", at)
	plan := probetemplate.Template{CellularProbe: &probetemplate.CellularProbe{Interval: 30, Telemetry: true}}
	s.ApplyConfiguration("d", "s", 1, &plan, "")
	n.Revision = 1
	n.Interval = 30
	if s.ObserveCellular("d", "s", n, at, 0) {
		t.Fatal("v1 cannot satisfy telemetry config")
	}
	n.Telemetry = true
	if !s.ObserveCellular("d", "s", n, at, 0) {
		t.Fatal("v2 rejected")
	}
}

func TestFM160PhysicalSample(t *testing.T) {
	p := CellularPort{Profile: "fibocom-fm160-v1", Queries: []AtIdentity{{Command: "AT+CESQ", Status: "ok", Value: "+CESQ: 99,99,255,255,255,255,65,66,81"}}}
	EnrichCellular(&p)
	if len(p.Signals) != 3 || p.Signals[0].Value != -10.75 || p.Signals[1].Value != -90.5 || p.Signals[2].Value != 17.25 || p.Signals[1].Minimum != -156 || p.Signals[1].Maximum != -31 {
		t.Fatal(p.Signals)
	}
	for _, tc := range []struct {
		code      string
		want      float64
		qualifier string
	}{{"0", -156, "<"}, {"1", -155.5, "≈"}, {"125", -31.5, "≈"}, {"126", -31, "≥"}} {
		p.Queries[0].Value = "+CESQ: 99,99,255,255,255,255,255," + tc.code + ",255"
		EnrichCellular(&p)
		if len(p.Signals) != 1 || p.Signals[0].Value != tc.want || p.Signals[0].Qualifier != tc.qualifier {
			t.Fatal(tc, p.Signals)
		}
	}
}
