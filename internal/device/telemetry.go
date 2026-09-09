package device

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Metric is a latest observation. Source and timestamps are assigned by the server.
type Metric struct {
	Reason    string    `json:"reason,omitempty"`
	Name      string    `json:"name"`
	Value     string    `json:"value"`
	Unit      string    `json:"unit"`
	Status    string    `json:"status"`
	Entity    string    `json:"entity"`
	Interval  uint32    `json:"interval_seconds"`
	AgeMS     uint64    `json:"age_ms,omitempty"`
	Source    string    `json:"source"`
	Group     string    `json:"group"`
	SampledAt time.Time `json:"sampled_at"`
	Stale     bool      `json:"stale"`
}
type Telemetry struct {
	Groups   map[string]map[string]Metric
	Template map[string]Metric
}

func MetricUnit(key string) string {
	if key != "probe_bits" && key != "kernel_bits" && !strings.HasPrefix(key, "cpu_") && !strings.HasPrefix(key, "memory_") && !strings.HasPrefix(key, "disk_") && !strings.HasPrefix(key, "net_") && !strings.HasPrefix(key, "switch_") {
		return "text"
	}
	switch {
	case strings.HasSuffix(key, "_seconds"):
		return "seconds"
	case strings.HasSuffix(key, "_usage"):
		return "percent"
	case strings.HasSuffix(key, "_bytes_per_sec"):
		return "bytes_per_sec"
	case strings.HasSuffix(key, "_bytes"):
		return "bytes"
	case strings.HasSuffix(key, "_mhz"):
		return "mhz"
	case key == "probe_bits" || key == "kernel_bits" || key == "cpu_hardware_bits":
		return "bits"
	default:
		return "text"
	}
}
func ValidMetricValue(value, unit string) bool {
	if unit == "bytes" || unit == "seconds" {
		_, err := strconv.ParseUint(value, 10, 64)
		return err == nil
	}
	if unit == "text" {
		return value != ""
	}
	n, e := strconv.ParseFloat(value, 64)
	if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || strings.TrimSpace(value) != value {
		return false
	}
	if unit == "percent" {
		return n <= 100
	}
	if unit == "bits" {
		return n == 32 || n == 64
	}
	return unit != "bytes" || (n == math.Trunc(n) && n <= float64(math.MaxInt64))
}
func emptyTelemetry() *Telemetry {
	return &Telemetry{Groups: map[string]map[string]Metric{}, Template: map[string]Metric{}}
}
func cloneMetrics(in map[string]Metric) map[string]Metric {
	out := make(map[string]Metric, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func cloneTelemetry(t *Telemetry) *Telemetry {
	if t == nil {
		return nil
	}
	out := &Telemetry{Groups: map[string]map[string]Metric{}, Template: cloneMetrics(t.Template)}
	for k, v := range t.Groups {
		out.Groups[k] = cloneMetrics(v)
	}
	return out
}
func EffectiveMetrics(s Session, now time.Time) map[string]Metric {
	if s.Telemetry == nil {
		return nil
	}
	out := map[string]Metric{}
	for _, group := range s.Telemetry.Groups {
		for k, v := range group {
			out[k] = v
		}
	}
	for k, v := range s.Telemetry.Template {
		if native, ok := out[k]; ok {
			v.Entity = native.Entity
			v.Group = native.Group
		}
		out[k] = v
	}
	for k, v := range out {
		if v.Interval > 0 {
			ttl := time.Duration(v.Interval) * 3 * time.Second
			if ttl < 15*time.Second {
				ttl = 15 * time.Second
			}
			v.Stale = now.Sub(v.SampledAt) > ttl
		}
		out[k] = v
	}
	return out
}
func (s *Service) Observe(id, sessionID, group string, values map[string]Metric, at time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.devices[id]
	if r == nil || r.current == nil || r.current.ID != sessionID {
		return false
	}
	if r.current.Telemetry == nil {
		r.current.Telemetry = emptyTelemetry()
	}
	t := r.current.Telemetry
	intervals := map[string]uint32{}
	if r.current.ConfigTemplate != nil {
		intervals = map[string]uint32{}
		for k, p := range r.current.ConfigTemplate.Properties {
			intervals[k] = p.Interval
		}
	}
	if group == "template" {
		for k := range values {
			if _, ok := intervals[k]; !ok {
				return false
			}
		}
	}
	next := map[string]Metric{}
	for k, v := range values {
		v.Source = "builtin"
		v.Group = group
		v.SampledAt = at.UTC().Add(-time.Duration(v.AgeMS) * time.Millisecond)
		v.AgeMS = 0
		if group == "template" {
			v.Source = "template"
			v.Interval = intervals[k]
			v.Unit = MetricUnit(k)
		}
		if v.Status == "ok" && !ValidMetricValue(v.Value, v.Unit) {
			v.Status = "error"
			v.Reason = "invalid_output"
			v.Value = ""
		}
		if v.Status != "ok" {
			v.Value = ""
		}
		next[k] = v
	}
	if group == "template" {
		for k, v := range next {
			if old, ok := t.Template[k]; !ok || !v.SampledAt.Before(old.SampledAt) {
				t.Template[k] = v
			}
		}
	} else if group == "egress" {
		if t.Groups[group] == nil {
			t.Groups[group] = map[string]Metric{}
		}
		for k, v := range next {
			t.Groups[group][k] = v
		}
	} else {
		t.Groups[group] = next
	}
	r.current.LastSeenAt = at
	s.revision.Add(1)
	return true
}
