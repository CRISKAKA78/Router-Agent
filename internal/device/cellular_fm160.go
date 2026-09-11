package device

import (
	"encoding/csv"
	"strconv"
	"strings"
)

type CellularField struct {
	Name   string `json:"name,omitempty"`
	Group  string `json:"group,omitempty"`
	Source string `json:"source,omitempty"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}
type CellularSignal struct {
	Key       string  `json:"key"`
	Rat       string  `json:"rat"`
	Value     float64 `json:"value"`
	Minimum   float64 `json:"minimum"`
	Maximum   float64 `json:"maximum"`
	Unit      string  `json:"unit"`
	Qualifier string  `json:"qualifier"`
}

// EnrichCellular is a pure projection of the current observation. No AT or I/O here.
func EnrichCellular(p *CellularPort) {
	p.Fields = nil
	p.Signals = nil
	seen := map[string]bool{}
	add := func(k, v string) {
		if !seen[k] {
			p.Fields = append(p.Fields, CellularField{Key: k, Value: v})
			seen[k] = true
		}
	}
	if p.ATI.Status == "ok" {
		names := map[string]string{"Manufacturer": "manufacturer", "Model": "model", "Revision": "firmware", "SVN": "svn"}
		for _, line := range strings.Split(p.ATI.Value, "\n") {
			if k, v, ok := strings.Cut(line, ":"); ok {
				if key := names[strings.TrimSpace(k)]; key != "" {
					add(key, strings.TrimSpace(v))
				}
			}
		}
	}
	if p.Profile != "fibocom-fm160-v1" && p.Profile != "fibocom-fm160-details-v1" {
		return
	}
	for _, q := range p.Queries {
		key := map[string]string{"AT+CPIN?": "sim", "AT+CCID": "iccid", "AT+CIMI": "imsi", "AT+COPS?": "operator", "AT+CEREG?": "eps_registration", "AT+C5GREG?": "nr_registration", "AT+CSQ": "rssi"}[q.Command]
		fail := "未提供"
		if q.Status != "ok" {
			switch q.Status {
			case "timeout":
				fail = "查询超时"
			case "rejected":
				fail = "查询被拒绝"
			}
			if key != "" {
				add(key, fail)
			}
			continue
		}
		switch q.Command {
		case "AT+CPIN?":
			v, ok := atBody(q.Value, "+CPIN:")
			if !ok {
				add(key, fail)
				continue
			}
			switch v {
			case "READY":
				v = "就绪"
			case "SIM PIN":
				v = "需要 PIN"
			case "SIM PUK":
				v = "需要 PUK"
			}
			add(key, v)
		case "AT+CCID", "AT+CIMI":
			prefix := "+CCID:"
			if key == "imsi" {
				prefix = "+CIMI:"
			}
			v := strings.TrimSpace(q.Value)
			if strings.HasPrefix(v, prefix) {
				v = strings.TrimSpace(strings.TrimPrefix(v, prefix))
			}
			v = strings.Trim(v, "\"")
			min, max := 19, 20
			if key == "imsi" {
				min, max = 5, 15
			}
			if len(v) < min || len(v) > max || !decimal(v) {
				v = fail
			}
			add(key, v)
		case "AT+COPS?":
			a, ok := atCSV(q.Value, "+COPS:")
			if !ok || len(a) < 3 || len(a) > 4 {
				add(key, fail)
				continue
			}
			add(key, a[2])
			if a[1] == "2" && (len(a[2]) == 5 || len(a[2]) == 6) && decimal(a[2]) {
				add("mcc", a[2][:3])
				add("mnc", a[2][3:])
			}
			if len(a) == 4 {
				act := map[string]string{"0": "GSM", "2": "UTRAN", "7": "LTE", "9": "NB-IoT", "10": "LTE / 5GC", "11": "NR / 5GC", "12": "NG-RAN", "13": "LTE-NR 双连接"}[a[3]]
				if act != "" {
					add("access", act)
				}
			}
		case "AT+CEREG?", "AT+C5GREG?":
			prefix := "+CEREG:"
			if key == "nr_registration" {
				prefix = "+C5GREG:"
			}
			a, ok := atCSV(q.Value, prefix)
			if !ok || len(a) < 2 {
				add(key, fail)
				continue
			}
			v := map[string]string{"0": "未注册", "1": "已注册 · 本地", "2": "正在搜索", "3": "注册被拒绝", "4": "未知", "5": "已注册 · 漫游", "6": "仅短信 · 本地", "7": "仅短信 · 漫游", "8": "仅紧急业务", "9": "CSFB受限 · 本地", "10": "CSFB受限 · 漫游", "11": "仅紧急业务"}[a[1]]
			if v == "" {
				v = fail
			}
			add(key, v)
		case "AT+CSQ":
			a, ok := atCSV(q.Value, "+CSQ:")
			if !ok || len(a) != 2 {
				add(key, fail)
				continue
			}
			n, e := strconv.Atoi(a[0])
			if e != nil || n < 0 || n > 31 {
				add(key, fail)
				continue
			}
			v := strconv.Itoa(-113+2*n) + " dBm"
			if n == 0 {
				v = "≤ " + v
			}
			if n == 31 {
				v = "≥ " + v
			}
			add(key, v)
		case "AT+CESQ":
			a, ok := atCSV(q.Value, "+CESQ:")
			if !ok || (len(a) != 6 && len(a) != 9) {
				add("signal", "返回格式不匹配")
				continue
			}
			codes := make([]int, len(a))
			valid := true
			for i, s := range a {
				n, e := strconv.Atoi(s)
				if e != nil || n < 0 || n > 255 {
					valid = false
					break
				}
				codes[i] = n
			}
			if !valid {
				add("signal", "返回格式不匹配")
				continue
			}
			// FM160/FG160 V1.3 §5.2, printed pp.53-55. Values are bins, not exact measurements.
			p.addSignalBin("rsrp", "LTE", codes[5], 97, -140, -44, 1, "dBm", true)
			p.addSignalBin("rsrq", "LTE", codes[4], 34, -19.5, -3, .5, "dB", true)
			if len(codes) == 9 {
				p.addSignalBin("rsrq", "NR", codes[6], 126, -43, 20, .5, "dB", false)
				// ETSI TS 127 007 V16.9.0 §8.69, pp.199–200 supplies the table omitted by FM160 V1.3.
				p.addSignalBin("rsrp", "NR", codes[7], 126, -156, -31, 1, "dBm", true)
				p.addSignalBin("sinr", "NR", codes[8], 127, -23, 40, .5, "dB", true)
			}
		}
	}
	if p.Profile == "fibocom-fm160-details-v1" {
		enrichFM160Details(p)
	}
}
func (p *CellularPort) addSignalBin(key, rat string, n, max int, low, high, step float64, unit string, saturated bool) {
	if n < 0 || n > max {
		return
	}
	value, qualifier := low, "<"
	if n > 0 {
		value = low + float64(n-1)*step + step/2
		qualifier = "≈"
	}
	if n == max && saturated {
		value = high
		qualifier = "≥"
	}
	p.Signals = append(p.Signals, CellularSignal{key, rat, value, low, high, unit, qualifier})
}
func decimal(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func atBody(s, prefix string) (string, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, prefix) || strings.ContainsAny(s, "\r\n") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(s, prefix)), true
}
func atCSV(s, prefix string) ([]string, bool) {
	body, ok := atBody(s, prefix)
	if !ok {
		return nil, false
	}
	r := csv.NewReader(strings.NewReader(body))
	r.TrimLeadingSpace = true
	a, e := r.Read()
	for i := range a {
		a[i] = strings.TrimSpace(a[i])
	}
	return a, e == nil
}
