package device

import (
	"encoding/csv"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type fm160Detail struct {
	p       *CellularPort
	queries map[string]AtIdentity
	keys    map[string]bool
}

func enrichFM160Details(p *CellularPort) {
	d := fm160Detail{p: p, queries: map[string]AtIdentity{}, keys: map[string]bool{}}
	for _, f := range p.Fields {
		d.keys[f.Key] = true
	}
	for _, q := range p.Queries {
		d.queries[q.Command] = q
	}
	for _, entry := range []struct{ cmd, key, name string }{{"AT+CEREG?", "eps", "EPS"}, {"AT+C5GREG?", "nr", "NR"}} {
		if q := d.queries[entry.cmd]; q.Status == "ok" {
			if a, ok := atCSV(q.Value, "+"+strings.TrimSuffix(strings.TrimPrefix(entry.cmd, "AT+"), "?")+":"); ok && len(a) >= 4 {
				d.add("registration_"+entry.key+"_tac", "网络注册", entry.name+" TAC", hexValue(a[2], 24), entry.cmd)
				bits := 36
				if entry.key == "eps" {
					bits = 28
				}
				d.add("registration_"+entry.key+"_cell", "网络注册", entry.name+" 小区 ID", hexValue(a[3], bits), entry.cmd)
			}
		}
	}
	for _, t := range []struct{ cmd, key, name string }{{"AT+MTSM=1", "temperature", "模块温度"}, {"AT+MTSM=6", "temperature_bb", "基带温度"}, {"AT+MTSM=7", "temperature_rf", "射频温度"}} {
		v := d.failure(t.cmd)
		if q := d.queries[t.cmd]; q.Status == "ok" {
			if b, ok := atBody(q.Value, "+MTSM:"); ok {
				v = numberUnit(b, -100, 200, " °C")
			}
		}
		if d.queries[t.cmd].Status == "not_queried" && d.queries["AT+MTSM?"].Status == "ok" {
			v = "未采集（保留原温度上报配置）"
		}
		d.add(t.key, "温度与供电", t.name, v, t.cmd)
	}
	v := d.failure("AT+CBC")
	if q := d.queries["AT+CBC"]; q.Status == "ok" {
		if a, ok := atCSV(q.Value, "+CBC:"); ok && len(a) == 2 {
			v = numberUnit(a[1], 0, 20000, " mV")
		}
	}
	d.add("voltage", "温度与供电", "模块供电电压", v, "AT+CBC")
	v = d.failure("AT+CGATT?")
	if q := d.queries["AT+CGATT?"]; q.Status == "ok" {
		if b, ok := atBody(q.Value, "+CGATT:"); ok {
			v = map[string]string{"0": "未附着", "1": "已附着"}[b]
		}
	}
	d.add("packet_attached", "网络注册", "分组域附着", v, "AT+CGATT?")
	d.locks()
	d.contexts()
	d.cells()
	d.carriers()
	d.radio()
}
func (d *fm160Detail) add(key, group, name, value, source string) {
	if d.keys[key] {
		return
	}
	d.keys[key] = true
	if value == "" {
		value = "未提供"
	}
	d.p.Fields = append(d.p.Fields, CellularField{Key: key, Value: value, Name: name, Group: group, Source: source})
}
func (d *fm160Detail) failure(cmd string) string {
	switch d.queries[cmd].Status {
	case "rejected":
		return "模块拒绝查询"
	case "timeout":
		return "查询超时"
	case "overflow":
		return "响应超出采集上限"
	case "not_queried":
		return "本轮未采集"
	case "ok":
		return "返回格式不匹配"
	}
	return "未提供"
}
func intRange(s string, low, high int64) (int64, bool) {
	n, e := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n, e == nil && n >= low && n <= high
}
func numberUnit(s string, low, high int64, unit string) string {
	if n, ok := intRange(s, low, high); ok {
		return strconv.FormatInt(n, 10) + unit
	}
	return "未提供"
}
func hexValue(s string, bits int) string {
	if s == "" {
		return "未提供"
	}
	n, e := strconv.ParseUint(s, 16, bits)
	if e != nil {
		return "未提供"
	}
	return fmt.Sprintf("%d（0x%s）", n, strings.ToUpper(s))
}
func csvValues(s string) ([]string, bool) {
	r := csv.NewReader(strings.NewReader(s))
	r.TrimLeadingSpace = true
	a, e := r.Read()
	for i := range a {
		a[i] = strings.TrimSpace(a[i])
	}
	return a, e == nil
}
func fmBand(s string) string {
	n, ok := intRange(s, 1, 50512)
	if !ok {
		return "未提供"
	}
	if n >= 101 && n <= 171 {
		return fmt.Sprintf("B%d", n-100)
	}
	if strings.HasPrefix(s, "50") {
		if band, ok := intRange(s[2:], 1, 512); ok {
			return fmt.Sprintf("n%d", band)
		}
	}
	if n <= 25 {
		return fmt.Sprintf("WCDMA B%d", n)
	}
	return s + "（原始编码）"
}
func bandRAT(s string) string {
	if strings.HasPrefix(fmBand(s), "n") {
		return "NR"
	}
	if strings.HasPrefix(fmBand(s), "B") {
		return "LTE"
	}
	return ""
}
func bandwidth(s, rat string) string {
	if rat == "LTE" {
		if v := map[string]string{"6": "1.4", "15": "3", "25": "5", "50": "10", "75": "15", "100": "20"}[s]; v != "" {
			return v + " MHz"
		}
	} else if rat == "NR" {
		if s == "0" {
			return "5 MHz"
		}
		for _, n := range []string{"10", "15", "20", "25", "30", "40", "50", "60", "70", "80", "90", "100", "200", "400"} {
			if s == n {
				return n + " MHz"
			}
		}
	}
	if s == "" {
		return "未提供"
	}
	return "未识别（编码 " + s + "）"
}

var bandGroups = regexp.MustCompile(`\(([^()]*)\)`)

func bandSet(s string) (map[string]bool, bool) {
	set := map[string]bool{}
	if strings.TrimSpace(s) == "" {
		return set, true
	}
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		a, b, rangeValue := strings.Cut(v, "-")
		low, ok := intRange(a, 0, 50512)
		if !ok {
			return nil, false
		}
		high := low
		if rangeValue {
			high, ok = intRange(b, low, 50512)
			if !ok || high-low > 512 {
				return nil, false
			}
		}
		for n := low; n <= high; n++ {
			set[strconv.FormatInt(n, 10)] = true
		}
		if len(set) > 1024 {
			return nil, false
		}
	}
	return set, true
}
func sortedBands(v map[string]bool) string {
	a := []string{}
	for k := range v {
		a = append(a, k)
	}
	sort.Slice(a, func(i, j int) bool { x, _ := strconv.Atoi(a[i]); y, _ := strconv.Atoi(a[j]); return x < y })
	for i := range a {
		a[i] = fmBand(a[i])
	}
	return strings.Join(a, "、")
}
func (d *fm160Detail) locks() {
	cmd := "AT+GTACT?"
	value := d.failure(cmd)
	if q := d.queries[cmd]; q.Status == "ok" {
		if a, ok := atCSV(q.Value, "+GTACT:"); ok && len(a) >= 3 {
			d.add("rat_allowed", "锁定配置", "允许制式", map[string]string{"1": "WCDMA", "2": "LTE", "4": "LTE / WCDMA", "10": "自动", "14": "NR", "16": "NR / WCDMA", "17": "NR / LTE", "20": "NR / WCDMA / LTE"}[a[0]], cmd)
			pref := map[string]string{"2": "WCDMA", "3": "LTE", "6": "NR"}
			d.add("rat_preferred", "锁定配置", "优先制式", pref[a[1]], cmd)
			if len(a) > 3 {
				current, valid := bandSet(strings.Join(a[3:], ","))
				groups := bandGroups.FindAllStringSubmatch(d.queries["AT+GTACT=?"].Value, -1)
				if valid {
					d.add("allowed_bands", "锁定配置", "允许频段（配置）", sortedBands(current), cmd)
				}
				value = "未提供（无法核对支持频段）"
				if valid && len(current) == 1 && current["0"] {
					value = "否"
				} else if valid && len(current) > 0 && d.queries["AT+GTACT=?"].Status == "ok" && len(groups) == 9 {
					supported := map[string]bool{}
					validSupport := true
					for _, idx := range []int{3, 4, 5, 6, 7, 8} {
						set, ok := bandSet(groups[idx][1])
						if !ok {
							validSupport = false
						}
						for k := range set {
							supported[k] = true
						}
					}
					selectedRAT := map[string]bool{}
					for k := range current {
						r := bandRAT(k)
						if r == "" {
							r = "WCDMA"
						}
						selectedRAT[r] = true
						if !supported[k] {
							validSupport = false
						}
					}
					// Only compare families actually reported; disabling a RAT is not a band lock.
					locked := false
					lockedFamilies := map[string]bool{}
					for k := range supported {
						r := bandRAT(k)
						if r == "" {
							r = "WCDMA"
						}
						if selectedRAT[r] && !current[k] {
							locked = true
							lockedFamilies[r] = true
						}
					}
					if validSupport && len(supported) > 0 {
						value = "否"
						if locked {
							restricted := map[string]bool{}
							for k := range current {
								r := bandRAT(k)
								if r == "" {
									r = "WCDMA"
								}
								if lockedFamilies[r] {
									restricted[k] = true
								}
							}
							value = sortedBands(restricted)
						}
					}
				}
			}
		}
	}
	d.add("lock_band", "锁定配置", "锁频段", value, "AT+GTACT? / AT+GTACT=?")
	cmd = "AT+GTCELLLOCK?"
	freq, cell := d.failure(cmd), d.failure(cmd)
	if q := d.queries[cmd]; q.Status == "ok" {
		if a, ok := atCSV(q.Value, "+GTCELLLOCK:"); ok && len(a) >= 1 {
			if a[0] == "0" {
				freq = "否"
				cell = "否"
			} else if a[0] == "1" && len(a) >= 4 {
				rat := map[string]string{"0": "LTE EARFCN", "1": "NR-ARFCN"}[a[1]]
				_, validFreq := intRange(a[3], 0, 4294967295)
				if rat != "" && validFreq && (a[2] == "0" || a[2] == "1") {
					freq = rat + " " + a[3]
					cell = "否"
					if a[2] == "0" {
						cell = "未提供"
						max := int64(503)
						if a[1] == "1" {
							max = 1007
						}
						if len(a) > 4 {
							if _, ok := intRange(a[4], 0, max); ok {
								cell = rat + " " + a[3] + "，PCI " + a[4]
							}
						}
					}
					if len(a) > 5 {
						d.add("lock_scs", "锁定配置", "锁定子载波间隔", map[string]string{"0": "15 kHz", "1": "30 kHz"}[a[5]], cmd)
					}
					if len(a) > 6 && a[6] != "" {
						d.add("lock_cell_band", "锁定配置", "锁定小区频段", fmBand(a[6]), cmd)
					}
				}
			}
		}
	}
	d.add("lock_frequency", "锁定配置", "锁频点", freq, cmd)
	d.add("lock_cell", "锁定配置", "锁小区", cell, cmd)
}

// IPv6 AT output may use sixteen decimal octets rather than colon notation.
func atIP(s string) string {
	s = strings.TrimSpace(s)
	if ip := net.ParseIP(s); ip != nil {
		if ip.IsUnspecified() {
			return "未分配"
		}
		return ip.String()
	}
	parts := strings.Split(s, ".")
	if len(parts) != 16 {
		return "未提供"
	}
	b := make(net.IP, 16)
	for i, p := range parts {
		n, ok := intRange(p, 0, 255)
		if !ok {
			return "未提供"
		}
		b[i] = byte(n)
	}
	if b.IsUnspecified() {
		return "未分配"
	}
	return b.String()
}
func atLocalAddress(s string) (string, string, string) {
	a := strings.Split(s, ".")
	bytes := 4
	family := "IPv4"
	if len(a) == 32 {
		bytes = 16
		family = "IPv6"
	}
	if len(a) != bytes*2 {
		ip := atIP(s)
		if strings.Contains(ip, ":") {
			family = "IPv6"
		}
		return ip, "未提供", family
	}
	ip := atIP(strings.Join(a[:bytes], "."))
	mask := make(net.IPMask, bytes)
	for i, v := range a[bytes:] {
		n, ok := intRange(v, 0, 255)
		if !ok {
			return ip, "未提供", family
		}
		mask[i] = byte(n)
	}
	ones, bits := mask.Size()
	if bits == 0 {
		return ip, "未提供", family
	}
	return ip, fmt.Sprintf("/%d", ones), family
}
func (d *fm160Detail) contexts() {
	for _, cmd := range []string{"AT+CGDCONT?", "AT+CGACT?", "AT+CGPADDR", "AT+CGCONTRDP"} {
		q := d.queries[cmd]
		prefix := "+" + strings.TrimSuffix(strings.TrimPrefix(cmd, "AT+"), "?") + ":"
		count := 0
		if q.Status == "ok" {
			for _, line := range strings.Split(q.Value, "\n") {
				if !strings.HasPrefix(line, prefix) {
					continue
				}
				a, ok := csvValues(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
				if !ok || len(a) < 2 {
					continue
				}
				cid, ok := intRange(a[0], 1, 255)
				if !ok {
					continue
				}
				count++
				if count > 32 {
					break
				}
				group := fmt.Sprintf("PDP 上下文 %d", cid)
				key := fmt.Sprintf("pdp_%d_", cid)
				switch cmd {
				case "AT+CGDCONT?":
					if len(a) >= 4 {
						d.add(key+"type", group, "PDP 类型", a[1], cmd)
						apn := a[2]
						if apn == "" {
							apn = "未指定"
						}
						d.add(key+"apn_configured", group, "配置 APN", apn, cmd)
					}
				case "AT+CGACT?":
					d.add(key+"active", group, "上下文激活", map[string]string{"0": "否", "1": "是"}[a[1]], cmd)
				case "AT+CGPADDR":
					for i, v := range a[1:] {
						if i >= 2 {
							break
						}
						d.add(key+fmt.Sprintf("address%d", i+1), group, fmt.Sprintf("上下文地址 %d", i+1), atIP(v), cmd)
					}
				case "AT+CGCONTRDP":
					if len(a) >= 4 {
						addr, mask, family := atLocalAddress(a[3])
						key += strings.ToLower(family) + "_"
						d.add(key+"bearer", group, family+" 承载 ID", numberUnit(a[1], 0, 255, ""), cmd)
						d.add(key+"apn", group, family+" 生效 APN", a[2], cmd)
						d.add(key+"address", group, family+" 动态地址", addr, cmd)
						d.add(key+"mask", group, family+" 前缀长度", mask, cmd)
						for i, name := range []string{"网关", "首选 DNS", "备用 DNS", "首选 P-CSCF", "备用 P-CSCF"} {
							if len(a) > i+4 {
								d.add(key+fmt.Sprintf("network%d", i), group, family+" "+name, atIP(a[i+4]), cmd)
							}
						}
					}
				}
			}
		}
		if count == 0 {
			v := d.failure(cmd)
			if q.Status == "ok" && strings.TrimSpace(q.Value) == "" {
				v = "未返回上下文"
			}
			d.add("pdp_query_"+strings.TrimPrefix(cmd, "AT+"), "数据连接", map[string]string{"AT+CGDCONT?": "PDP 配置", "AT+CGACT?": "PDP 激活信息", "AT+CGPADDR": "PDP 地址", "AT+CGCONTRDP": "PDP 动态参数"}[cmd], v, cmd)
		}
	}
}
func signalText(s CellularSignal) string { return fmt.Sprintf("%s%g %s", s.Qualifier, s.Value, s.Unit) }
func (d *fm160Detail) bin(key, rat string, n int) *CellularSignal {
	p := CellularPort{}
	switch rat + "/" + key {
	case "LTE/rsrp":
		p.addSignalBin(key, rat, n, 97, -140, -44, 1, "dBm", true)
	case "LTE/rsrq":
		p.addSignalBin(key, rat, n, 34, -19.5, -3, .5, "dB", true)
	case "NR/rsrp":
		p.addSignalBin(key, rat, n, 126, -156, -31, 1, "dBm", true)
	case "NR/rsrq":
		p.addSignalBin(key, rat, n, 126, -43, 20, .5, "dB", false)
	case "NR/sinr":
		p.addSignalBin(key, rat, n, 127, -23, 40, .5, "dB", true)
	}
	if len(p.Signals) == 1 {
		return &p.Signals[0]
	}
	return nil
}
func (d *fm160Detail) appendSignal(s CellularSignal) {
	for _, old := range d.p.Signals {
		if old.Key == s.Key && old.Rat == s.Rat {
			return
		}
	}
	d.p.Signals = append(d.p.Signals, s)
}
func (d *fm160Detail) cells() {
	cmd := "AT+GTCCINFO?"
	q := d.queries[cmd]
	headerRat, role := "", ""
	counts := map[string]int{}
	validRows := 0
	if q.Status == "ok" {
		for _, line := range strings.Split(q.Value, "\n") {
			line = strings.TrimSpace(line)
			lower := strings.ToLower(line)
			if strings.Contains(lower, "service cell") || strings.Contains(lower, "neighbor cell") {
				headerRat = ""
				if strings.HasPrefix(lower, "lte-nr en-dc ") {
					headerRat = "ENDC"
				} else if strings.HasPrefix(lower, "nr ") {
					headerRat = "NR"
				} else if strings.HasPrefix(lower, "lte ") {
					headerRat = "LTE"
				}
				role = "neighbor"
				if strings.Contains(lower, "service cell") {
					role = "serving"
				}
				continue
			}
			if headerRat == "" || role == "" {
				continue
			}
			a, ok := csvValues(line)
			rat := headerRat
			if rat == "ENDC" && len(a) > 1 {
				rat = map[string]string{"4": "LTE", "9": "NR"}[a[1]]
			}
			// V1.3 page 90 example has an empty reserved slot before LTE neighbor bandwidth.
			if ok && rat == "LTE" && role == "neighbor" && len(a) == 13 && a[8] == "" {
				a = append(a[:8], a[9:]...)
			}
			size := 14
			if role == "neighbor" {
				size = 12
			}
			if !ok || len(a) != size || (rat != "LTE" && rat != "NR") {
				continue
			}
			expectedRat := "4"
			if rat == "NR" {
				expectedRat = "9"
			}
			expectedRole := "1"
			if role == "neighbor" {
				expectedRole = "2"
			}
			if a[0] != expectedRole || a[1] != expectedRat {
				continue
			}
			counts[role]++
			if counts[role] > 20 {
				continue
			}
			validRows++
			group := "服务小区 · " + rat
			if role == "neighbor" {
				group = fmt.Sprintf("邻区 %d · %s", counts[role], rat)
			}
			key := fmt.Sprintf("cell_%s_%s_%d_", rat, role, counts[role])
			bits := 28
			if rat == "NR" {
				bits = 36
			}
			for _, entry := range []struct{ key, name, value string }{{"mcc", "MCC", a[2]}, {"mnc", "MNC", a[3]}, {"tac", "TAC", hexValue(a[4], 24)}, {"id", "小区 ID", hexValue(a[5], bits)}, {"arfcn", map[string]string{"LTE": "EARFCN", "NR": "NR-ARFCN"}[rat], hexValue(a[6], 32)}, {"pci", "PCI", hexValue(a[7], 10)}} {
				d.add(key+entry.key, group, entry.name, entry.value, cmd)
			}
			if role == "serving" {
				d.add(key+"band", group, "驻留频段", fmBand(a[8]), cmd)
				d.add(key+"bandwidth", group, "小区带宽", bandwidth(a[9], rat), cmd)
			} else if rat == "LTE" {
				d.add(key+"bandwidth", group, "邻区带宽", bandwidth(a[8], rat), cmd)
			}
			rp, rq := 12, 13
			sinr := -1
			if role == "neighbor" {
				rp = 10
				rq = 11
				if rat == "NR" {
					sinr = 8
				}
			} else {
				sinr = 10
			}
			d.add(key+"rxlev", group, "接收电平（原始编码）", numberUnit(a[rp-1], 0, 255, ""), cmd)
			for _, e := range []struct {
				key string
				idx int
			}{{"rsrp", rp}, {"rsrq", rq}, {"sinr", sinr}} {
				if e.idx < 0 {
					continue
				}
				v := "未提供"
				n, ok := intRange(a[e.idx], -100, 255)
				if ok {
					var s *CellularSignal
					if rat == "LTE" && e.key == "sinr" && n <= 100 {
						value := float64(n)*.5 - .25
						qualifier := "≈"
						if n == -100 {
							value = -50
							qualifier = "≤"
						}
						s = &CellularSignal{Key: "sinr", Rat: "LTE", Value: value, Minimum: -50, Maximum: 50, Unit: "dB", Qualifier: qualifier}
					} else if !(rat == "LTE" && e.key == "sinr") {
						s = d.bin(e.key, rat, int(n))
					}
					if s != nil {
						v = signalText(*s)
						if role == "serving" {
							d.appendSignal(*s)
						}
					}
				}
				d.add(key+e.key, group, strings.ToUpper(e.key), v, cmd)
			}
		}
	}
	if validRows == 0 {
		d.add("serving_cells", "服务小区", "服务小区与邻区", d.failure(cmd), cmd)
	} else {
		d.add("neighbor_count", "网络注册", "本次报告邻区数", strconv.Itoa(counts["neighbor"]), cmd)
	}
}

var secondaryCarrier = regexp.MustCompile(`^SCC[1-9][0-9]?$`)

func (d *fm160Detail) carriers() {
	cmd := "AT+GTCAINFO?"
	q := d.queries[cmd]
	count, secondaries := 0, 0
	if q.Status == "ok" {
		for _, line := range strings.Split(q.Value, "\n") {
			label, body, ok := strings.Cut(strings.TrimSpace(line), ":")
			if !ok || (label != "PCC" && !secondaryCarrier.MatchString(label)) {
				continue
			}
			a, ok := csvValues(body)
			size, offset := 9, 0
			if label != "PCC" {
				size = 12
				offset = 2
			}
			if !ok || len(a) != size {
				continue
			}
			rat := bandRAT(a[offset])
			if rat == "" {
				continue
			}
			count++
			if count > 16 {
				break
			}
			group := "载波 " + label + " · " + rat
			key := "ca_" + rat + "_" + label + "_"
			d.add(key+"band", group, "频段", fmBand(a[offset]), cmd)
			// GTCAINFO's numeric base/bandwidth variants differ by firmware; never silently guess them.
			d.add(key+"pci", group, "PCI（原始值）", a[offset+1], cmd)
			d.add(key+"arfcn", group, map[string]string{"LTE": "EARFCN", "NR": "NR-ARFCN"}[rat]+"（原始值）", a[offset+2], cmd)
			d.add(key+"dl_bw", group, "下行带宽", bandwidth(a[offset+3], rat), cmd)
			mimo := offset + 4
			if offset == 2 {
				secondaries++
				d.add(key+"state", group, "辅载波激活", map[string]string{"1": "已配置、未激活", "2": "已配置并激活"}[a[0]], cmd)
				d.add(key+"ul_ca", group, "上行 CA 支持", map[string]string{"0": "否", "1": "是"}[a[1]], cmd)
				d.add(key+"ul_bw", group, "上行带宽", bandwidth(a[offset+4], rat), cmd)
				mimo++
			}
			d.add(key+"dl_mimo", group, "下行 MIMO 层数", numberUnit(a[mimo], 1, 4, ""), cmd)
			d.add(key+"ul_mimo", group, "上行 MIMO 层数", numberUnit(a[mimo+1], 1, 4, ""), cmd)
			modulation := map[string]string{"0": "BPSK", "1": "QPSK", "2": "16QAM", "3": "64QAM", "4": "256QAM", "5": "1024QAM", "6": "未知"}
			d.add(key+"dl_mod", group, "下行调制", modulation[a[mimo+2]], cmd)
			d.add(key+"ul_mod", group, "上行调制", modulation[a[mimo+3]], cmd)
			d.add(key+"rsrp", group, "RSRP（原始编码）", numberUnit(a[mimo+4], 0, 255, ""), cmd)
		}
	}
	value := d.failure(cmd)
	if count > 0 {
		value = fmt.Sprintf("本次报告 %d 个主载波、%d 个辅载波", count-secondaries, secondaries)
	}
	d.add("ca_summary", "网络注册", "载波信息", value, cmd)
}
func (d *fm160Detail) radio() {
	cmd := "AT+GTCELLINFO?"
	q := d.queries[cmd]
	rat := ""
	mode := ""
	count := 0
	if q.Status == "ok" {
		for _, line := range strings.Split(q.Value, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "+GTCELLINFO:") {
				mode = strings.TrimSpace(strings.TrimPrefix(line, "+GTCELLINFO:"))
				continue
			}
			if line == "LTE:" {
				rat = "LTE"
				continue
			}
			if line == "NR5G:" {
				rat = "NR"
				continue
			}
			if rat == "" {
				continue
			}
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			names := map[string]string{"CQI": "CQI", "NR_CQI": "CQI", "Power": "发射功率（原始值）", "NR_Power": "发射功率（原始值）", "RANK": "下行 Rank", "NR_RANK": "下行 Rank", "DLMCS": "下行 MCS", "NR_DLMCS": "下行 MCS", "ULMCS": "上行 MCS", "NR_ULMCS": "上行 MCS", "SSB_BeamID": "SSB 波束 ID", "TX_LTE_QCI": "发送 QCI", "RX_LTE_QCI": "接收 QCI", "TX_5G_QCI": "发送 QCI", "RX_5G_QCI": "接收 QCI"}
			if name := names[k]; name != "" {
				d.add("radio_"+rat+"_"+k, "射频测量 · "+rat, name, v, cmd)
				count++
			}
		}
	}
	if count == 0 {
		v := d.failure(cmd)
		if mode == "0" {
			v = "未提供（模块详细测量未启用）"
		}
		d.add("radio_power", "射频测量", "发射功率 / CQI / MCS", v, cmd)
	}
}
