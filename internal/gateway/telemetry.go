package gateway

import (
	"encoding/json"
	"errors"
	"net"
	"routerprobe/internal/device"
	"routerprobe/internal/probetemplate"
	"routerprobe/internal/protocol"
	"strings"
	"time"
)

func parseTelemetry(payload []byte) (string, map[string]device.Metric, error) {
	bad := errors.New("invalid telemetry event")
	if len(payload) > 65536 || protocol.ValidObject(payload) != nil {
		return "", nil, bad
	}
	var event struct {
		Event  string                     `json:"event"`
		Group  string                     `json:"group"`
		Values map[string]json.RawMessage `json:"values"`
	}
	if json.Unmarshal(payload, &event) != nil || event.Event != "telemetry" || event.Values == nil || len(event.Values) > 1024 || event.Group != "switch" && len(event.Values) > 256 {
		return "", nil, bad
	}
	switch event.Group {
	case "hardware", "cpu", "memory", "disk", "network", "template", "egress", "switch":
	default:
		return "", nil, bad
	}
	if event.Group == "template" && len(event.Values) > 38 {
		return "", nil, bad
	}
	values := map[string]device.Metric{}
	for k, raw := range event.Values {
		if !probetemplate.ValidKey(k) {
			return "", nil, bad
		}
		fields, e := decodeObject(raw)
		if e != nil {
			return "", nil, bad
		}
		var m device.Metric
		if json.Unmarshal(raw, &m) != nil || !probetemplate.ValidText(m.Name, 128) || len(m.Entity) > 512 || strings.ContainsRune(m.Entity, 0) || len(m.Reason) > 128 || strings.ContainsRune(m.Reason, 0) || m.Interval > 86400 || m.AgeMS > 315360000000 {
			return "", nil, bad
		}
		if _, ok := fields["reason"]; ok {
			if _, e := requiredString(fields, "reason", 0, 128, false); e != nil {
				return "", nil, bad
			}
		}
		for _, name := range []string{"name", "value", "unit", "status"} {
			if _, e := requiredString(fields, name, 0, 4096, false); e != nil {
				return "", nil, bad
			}
		}
		for _, name := range []string{"interval_seconds", "age_ms"} {
			if v, ok := fields[name]; ok {
				if _, e := parseUnsignedInteger(v, name, 315360000000); e != nil {
					return "", nil, bad
				}
			}
		}
		limit := 512
		if event.Group == "template" {
			limit = 4096
		}
		if len(m.Value) > limit || strings.ContainsRune(m.Value, 0) {
			return "", nil, bad
		}
		switch m.Unit {
		case "text", "percent", "bytes", "bytes_per_sec", "mhz", "bits", "seconds":
		default:
			return "", nil, bad
		}
		switch m.Status {
		case "ok", "unknown", "waiting", "error":
		default:
			return "", nil, bad
		}
		if event.Group != "template" {
			if event.Group == "egress" && m.Status == "ok" {
				ip := net.ParseIP(m.Value)
				if ip == nil || (k == "egress_ipv4") != (ip.To4() != nil) {
					return "", nil, bad
				}
			}
			valid := event.Group == "switch" && strings.HasPrefix(k, "switch_") || event.Group == "egress" && (k == "egress_ipv4" || k == "egress_ipv6") || event.Group == "hardware" && (k == "model" || k == "firmware" || k == "cpu_model" || k == "cpu_soc" || k == "cpu_arch" || k == "cpu_hardware_bits" || k == "cpu_max_mhz" || k == "cpu_collection_status" || k == "kernel_arch" || k == "kernel_bits" || k == "probe_bits") || event.Group == "cpu" && (k == "cpu_usage" || k == "cpu_current_mhz") || event.Group == "memory" && strings.HasPrefix(k, "memory_") || event.Group == "disk" && strings.HasPrefix(k, "disk_") || event.Group == "network" && strings.HasPrefix(k, "net_")
			if !valid || m.Unit != device.MetricUnit(k) || m.Status == "ok" && !device.ValidMetricValue(m.Value, m.Unit) {
				return "", nil, bad
			}
		}
		values[k] = m
	}
	return event.Group, values, nil
}
func (s *Server) recordTelemetry(active *session, group string, values map[string]device.Metric) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	at := time.Now()
	if s.sessions[active.deviceID] == active {
		s.devices.Observe(active.deviceID, active.sessionID, group, values, at)
	}
	return at
}
