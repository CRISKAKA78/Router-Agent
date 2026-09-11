package device

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func detailedFixture(t *testing.T) CellularPort {
	t.Helper()
	b, e := os.ReadFile("testdata/fm160-details.json")
	if e != nil {
		t.Fatal(e)
	}
	var p CellularPort
	if e = json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	return p
}
func fieldMap(p CellularPort) map[string]string {
	m := map[string]string{}
	for _, f := range p.Fields {
		m[f.Key] = f.Value
	}
	return m
}
func replaceDetailed(p *CellularPort, cmd, value, status string) {
	for i := range p.Queries {
		if p.Queries[i].Command == cmd {
			p.Queries[i].Value = value
			p.Queries[i].Status = status
		}
	}
}
func TestFM160DetailedProjection(t *testing.T) {
	p := detailedFixture(t)
	EnrichCellular(&p)
	m := fieldMap(p)
	want := map[string]string{"lock_band": "否", "lock_frequency": "否", "lock_cell": "否", "temperature": "38 °C", "temperature_bb": "42 °C", "voltage": "3953 mV", "packet_attached": "已附着", "cell_NR_serving_1_band": "n78", "cell_NR_serving_1_bandwidth": "100 MHz", "cell_NR_serving_1_pci": "198（0xC6）", "cell_NR_serving_1_arfcn": "627264（0x99240）", "neighbor_count": "2", "pdp_1_ipv4_apn": "ctnet", "pdp_1_ipv4_address": "192.0.2.1", "pdp_1_ipv4_mask": "/30", "pdp_1_ipv6_address": "2001:db8::1", "pdp_1_ipv6_mask": "/64", "pdp_3_active": "否", "ca_NR_PCC_dl_bw": "未识别（编码 500）", "ca_NR_PCC_dl_mod": "QPSK", "radio_power": "未提供（模块详细测量未启用）"}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s got %q want %q", k, m[k], v)
		}
	}
	if len(m) != len(p.Fields) || len(m) < 100 {
		t.Fatal("duplicate/omitted fields", len(m), len(p.Fields))
	}
	for _, f := range p.Fields {
		if strings.HasPrefix(f.Key, "lock_") && (f.Name == "" || f.Group != "锁定配置" || f.Source == "") {
			t.Fatal("presentation metadata", f)
		}
	}
}
func TestFM160LocksAreConfigurationNotServingBand(t *testing.T) {
	for _, tc := range []struct{ value, freq, cell string }{{"+GTCELLLOCK: 0", "否", "否"}, {"+GTCELLLOCK: 1,1,1,627264,,1,5078", "NR-ARFCN 627264", "否"}, {"+GTCELLLOCK: 1,1,0,627264,198,1,5078", "NR-ARFCN 627264", "NR-ARFCN 627264，PCI 198"}, {"+GTCELLLOCK: 1,0,0,40936,18", "LTE EARFCN 40936", "LTE EARFCN 40936，PCI 18"}} {
		p := detailedFixture(t)
		replaceDetailed(&p, "AT+GTCELLLOCK?", tc.value, "ok")
		EnrichCellular(&p)
		m := fieldMap(p)
		if m["lock_frequency"] != tc.freq || m["lock_cell"] != tc.cell {
			t.Fatal(tc, m["lock_frequency"], m["lock_cell"])
		}
	}
	p := detailedFixture(t)
	replaceDetailed(&p, "AT+GTACT?", "+GTACT: 20,6,3,1,5,8,101,103,105,108,134,138,139,140,141,5078", "ok")
	EnrichCellular(&p)
	if fieldMap(p)["lock_band"] != "n78" {
		t.Fatal(fieldMap(p)["lock_band"])
	}
	for _, cmd := range []string{"AT+GTACT?", "AT+GTACT=?", "AT+GTCELLLOCK?"} {
		p = detailedFixture(t)
		replaceDetailed(&p, cmd, "", "timeout")
		EnrichCellular(&p)
		m := fieldMap(p)
		if (cmd == "AT+GTCELLLOCK?" && (m["lock_cell"] == "否" || m["lock_frequency"] == "否")) || (cmd != "AT+GTCELLLOCK?" && m["lock_band"] == "否") {
			t.Fatal("failure falsely unlocked", cmd)
		}
	}
}
func TestFM160DetailsInvalidAndMissing(t *testing.T) {
	p := detailedFixture(t)
	for i := range p.Queries {
		p.Queries[i].Status = "timeout"
		p.Queries[i].Value = ""
	}
	EnrichCellular(&p)
	if len(p.Signals) != 0 {
		t.Fatal("stale signals")
	}
	for k, v := range fieldMap(p) {
		if strings.HasPrefix(k, "lock_") && v == "否" {
			t.Fatal(k)
		}
	}
	if atIP("32.1.13.184.0.0.0.0.0.0.0.0.0.0.0.1") != "2001:db8::1" || atIP("0.0.0.0") != "未分配" || atIP("999.0.0.1") != "未提供" {
		t.Fatal("AT addresses")
	}
	if _, ok := bandSet("1-50512"); ok {
		t.Fatal("unbounded bands")
	}
}
func TestFM160LTECAAndRadio(t *testing.T) {
	p := detailedFixture(t)
	replaceDetailed(&p, "AT+GTCCINFO?", "+GTCCINFO:\nLTE service cell:\n1,4,460,00,90F3,A6F3EC6,9FE8,12,141,100,16,43,43,24\nLTE neighbor cell:\n2,4,,,,,A0AE,7C,,,31,12", "ok")
	replaceDetailed(&p, "AT+GTCAINFO?", "PCC: 141,18,40936,100,2,1,0,3,45\nSCC1: 2,1,103,12,1300,100,50,2,1,4,3,40", "ok")
	replaceDetailed(&p, "AT+GTCELLINFO?", "+GTCELLINFO: 1\nLTE:\nCQI: 11\nPower: 5\nRANK: RANK1\nDLMCS: 0\nULMCS: 0", "ok")
	EnrichCellular(&p)
	m := fieldMap(p)
	if m["ca_LTE_SCC1_ul_bw"] != "10 MHz" || m["ca_LTE_SCC1_state"] != "已配置并激活" || m["radio_LTE_Power"] != "5" || m["cell_LTE_serving_1_sinr"] != "≈7.75 dB" {
		t.Fatal(m)
	}
	found := false
	for _, s := range p.Signals {
		if s.Rat == "LTE" && s.Key == "sinr" && s.Minimum == -50 && s.Maximum == 50 {
			found = true
		}
	}
	if !found {
		t.Fatal("LTE SINR missing")
	}
}

func TestFM160ManualENDCAndNeighborExample(t *testing.T) {
	p := detailedFixture(t)
	replaceDetailed(&p, "AT+GTCCINFO?", "+GTCCINFO:\nLTE-NR EN-DC service cell:\n1,4,460,00,90F3,A6F3EC6,9FE8,12,141,100,16,43,43,24\n1,9,460,11,123456,0123456789,99240,C6,5078,100,91,74,74,67\nLTE neighbor cell:\n2,4,,,,,A0AE,7C,,,31,31,12", "ok")
	EnrichCellular(&p)
	m := fieldMap(p)
	for k, v := range map[string]string{"cell_LTE_serving_1_band": "B41", "cell_NR_serving_2_band": "n78", "cell_LTE_neighbor_1_rxlev": "31", "neighbor_count": "1"} {
		if m[k] != v {
			t.Errorf("%s=%q want %q", k, m[k], v)
		}
	}
}
