package probetemplate

import (
	"bytes"
	"encoding/json"
)

// Presence enables automatic USB AT identity collection. No arbitrary commands or ports.
type CellularProbe struct {
	Interval  uint32 `json:"interval_seconds"`
	Details   bool   `json:"details,omitempty"`
	Telemetry bool   `json:"telemetry,omitempty"`
}

func (c *CellularProbe) UnmarshalJSON(b []byte) error {
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil || fields == nil {
		return ErrInvalid
	}
	v := CellularProbe{Interval: 30}
	for k, raw := range fields {
		if k == "details" {
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &v.Details) != nil {
				return ErrInvalid
			}
			continue
		}
		if k == "telemetry" {
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &v.Telemetry) != nil {
				return ErrInvalid
			}
			continue
		}
		if k != "interval_seconds" || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &v.Interval) != nil {
			return ErrInvalid
		}
	}
	if e := ValidateCellular(&v); e != nil {
		return e
	}
	*c = v
	return nil
}
func ValidateCellular(c *CellularProbe) error {
	if c != nil && c.Details && !c.Telemetry {
		return &FieldError{Field: "cellular_probe.details", Detail: "详细采集须同时启用telemetry"}
	}
	if c != nil && (c.Interval < 10 || c.Interval > 86400) {
		return &FieldError{Field: "cellular_probe.interval_seconds", Detail: "AT 自动探测周期须为10～86400秒"}
	}
	return nil
}
func CopyCellular(c *CellularProbe) *CellularProbe {
	if c == nil {
		return nil
	}
	v := *c
	return &v
}
