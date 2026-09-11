package gateway

import (
	"encoding/json"
	"errors"
	"regexp"
	"routerprobe/internal/device"
	"routerprobe/internal/protocol"
	"slices"
	"strings"
	"time"
)

var cellularPath = regexp.MustCompile(`^/dev/tty(USB|ACM)[0-9]{1,10}$`)
var cellularIMEI = regexp.MustCompile(`^[0-9]{15}$`)

func parseCellular(raw []byte) (device.Cellular, time.Duration, error) {
	var v struct {
		device.Cellular
		Event string `json:"event"`
		Age   uint64 `json:"age_ms"`
	}
	bad := errors.New("invalid cellular event")
	if len(raw) > 65536 || protocol.ValidObject(raw) != nil || json.Unmarshal(raw, &v) != nil || (v.Event != "cellular" && v.Event != "cellular_telemetry" && v.Event != "cellular_details") || v.Revision == 0 || v.Interval < 10 || v.Interval > 86400 || v.Age > 315360000000 || v.Ports == nil || len(v.Ports) > 16 || !slices.Contains([]string{"ok", "partial", "unavailable", "no_ports", "error"}, v.Status) || len(v.Reason) > 64 || strings.ContainsAny(v.Reason, "\x00\r\n") {
		return v.Cellular, 0, bad
	}
	v.Details = v.Event == "cellular_details"
	v.Telemetry = v.Event == "cellular_telemetry" || v.Details
	fields, _ := decodeObject(raw)
	for _, k := range []string{"event", "config_revision", "interval_seconds", "age_ms", "status", "reason", "limited", "ports"} {
		if b, ok := fields[k]; !ok || string(b) == "null" {
			return v.Cellular, 0, bad
		}
	}
	if _, e := parseUnsignedInteger(fields["config_revision"], "config_revision", ^uint64(0)); e != nil {
		return v.Cellular, 0, bad
	}
	// No server-derived time/stale fields are accepted from a Probe.
	for k := range fields {
		if !slices.Contains([]string{"event", "config_revision", "interval_seconds", "age_ms", "status", "reason", "limited", "ports"}, k) {
			return v.Cellular, 0, bad
		}
	}
	validQuery := func(q device.AtIdentity, imei bool) bool {
		if !slices.Contains([]string{"ok", "not_queried", "rejected", "invalid_value", "invalid_response", "timeout", "io_error", "overflow", "cancelled"}, q.Status) || len(q.Value) > 1024 {
			return false
		}
		if imei {
			if !slices.Contains([]string{"", "AT+CGSN", "AT+CGSN=1", "AT+GSN"}, q.Command) {
				return false
			}
		} else if q.Command != "ATI" {
			return false
		}
		if q.Status != "ok" {
			return q.Value == ""
		}
		if q.Command == "" || q.Value == "" {
			return false
		}
		if imei {
			return cellularIMEI.MatchString(q.Value)
		}
		for _, c := range q.Value {
			if (c < 32 && c != '\n' && c != '\t') || c > 126 {
				return false
			}
		}
		return true
	}
	var rawPorts []json.RawMessage
	if json.Unmarshal(fields["ports"], &rawPorts) != nil {
		return v.Cellular, 0, bad
	}
	for _, rawPort := range rawPorts {
		obj, e := decodeObject(rawPort)
		if e != nil {
			return v.Cellular, 0, bad
		}
		for key := range obj {
			if !slices.Contains([]string{"path", "device_key", "status", "reason", "selected", "age_ms", "ati", "imei"}, key) && !(v.Telemetry && (key == "profile" || key == "queries")) {
				return v.Cellular, 0, bad
			}
		}
		if v.Telemetry {
			for _, key := range []string{"profile", "queries"} {
				if b, ok := obj[key]; !ok || string(b) == "null" {
					return v.Cellular, 0, bad
				}
			}
		}
	}
	paths, selected := map[string]bool{}, map[string]bool{}
	success := 0
	for _, p := range v.Ports {
		if !validCellularExtension(p, v.Telemetry, v.Details) {
			return v.Cellular, 0, bad
		}
		if !cellularPath.MatchString(p.Path) || paths[p.Path] || len(p.DeviceKey) > 256 || !strings.HasPrefix(p.DeviceKey, "/sys/devices/") || strings.ContainsAny(p.DeviceKey, "\x00\r\n") || strings.Contains(p.DeviceKey, "/../") || p.AgeMS < v.Age || p.AgeMS > 315360000000 || len(p.Reason) > 64 || strings.ContainsAny(p.Reason, "\x00\r\n") || !p.SampledAt.IsZero() || !slices.Contains([]string{"ok", "partial", "busy", "not_at", "error", "pending", "alternate"}, p.Status) || !validQuery(p.ATI, false) || !validQuery(p.IMEI, true) {
			return v.Cellular, 0, bad
		}
		paths[p.Path] = true
		if p.Status == "ok" && (p.ATI.Status != "ok" || p.IMEI.Status != "ok") {
			return v.Cellular, 0, bad
		}
		if p.Status != "ok" && p.Status != "partial" && (p.ATI.Status != "not_queried" || p.IMEI.Status != "not_queried") {
			return v.Cellular, 0, bad
		}
		if p.Selected {
			if selected[p.DeviceKey] || (p.Status != "ok" && p.Status != "partial") {
				return v.Cellular, 0, bad
			}
			selected[p.DeviceKey] = true
			success++
			if v.Status == "ok" && p.Status != "ok" {
				return v.Cellular, 0, bad
			}
		}
	}
	if (v.Status == "ok" || v.Status == "partial") && success == 0 || v.Status == "no_ports" && len(v.Ports) != 0 || v.Status == "unavailable" && success != 0 {
		return v.Cellular, 0, bad
	}
	return v.Cellular, time.Duration(v.Age) * time.Millisecond, nil
}

func validCellularExtension(p device.CellularPort, telemetry bool, detailed ...bool) bool {
	details := len(detailed) > 0 && detailed[0]
	profile := "fibocom-fm160-v1"
	if details {
		profile = "fibocom-fm160-details-v1"
	}
	if !telemetry {
		return p.Profile == "" && len(p.Queries) == 0
	}
	if p.Profile == "" {
		return len(p.Queries) == 0
	}
	if p.Profile != profile || p.Status != "ok" || !strings.Contains(p.ATI.Value, "Manufacturer: Fibocom Wireless Inc.") || !strings.Contains("\n"+p.ATI.Value+"\n", "\nModel: FM160-CN\n") {
		return false
	}
	commands := []string{"AT+CPIN?", "AT+CCID", "AT+CIMI", "AT+COPS?", "AT+CEREG?", "AT+C5GREG?", "AT+CSQ", "AT+CESQ"}
	maxValue := 1024
	if details {
		maxValue = 4096
		commands = append(commands, "AT+CBC", "AT+MTSM?", "AT+MTSM=1", "AT+MTSM=6", "AT+MTSM=7", "AT+CGATT?", "AT+CGACT?", "AT+CGDCONT?", "AT+CGPADDR", "AT+CGCONTRDP", "AT+GTACT?", "AT+GTACT=?", "AT+GTCELLLOCK?", "AT+GTCAINFO?", "AT+GTCELLINFO?", "AT+GTCCINFO?")
	}
	if len(p.Queries) != len(commands) {
		return false
	}
	for i, q := range p.Queries {
		if q.Command != commands[i] || len(q.Value) > maxValue || !slices.Contains([]string{"ok", "not_queried", "rejected", "invalid_value", "invalid_response", "timeout", "io_error", "overflow", "cancelled"}, q.Status) || (q.Status != "ok" && q.Value != "") {
			return false
		}
		for _, c := range q.Value {
			if (c < 32 && c != '\n' && c != '\t') || c > 126 {
				return false
			}
		}
	}
	return true
}
