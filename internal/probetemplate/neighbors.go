package probetemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type NeighborDomain struct {
	Ports     []string `json:"ports,omitempty"`
	ID        string   `json:"id"`
	Scope     string   `json:"scope"`
	Interface string   `json:"interface"`
	LeaseFile string   `json:"lease_file,omitempty"`
}
type NeighborProbe struct {
	FDBPreset  string           `json:"fdb_preset,omitempty"`
	Interval   uint32           `json:"interval_seconds"`
	Domains    []NeighborDomain `json:"domains"`
	FDBCommand string           `json:"fdb_command,omitempty"`
}

func (n *NeighborDomain) UnmarshalJSON(b []byte) error {
	type plain NeighborDomain
	var v plain
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil || fields == nil {
		return ErrInvalid
	}
	for _, raw := range fields {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return ErrInvalid
		}
	}
	*n = NeighborDomain(v)
	return nil
}
func (n *NeighborProbe) UnmarshalJSON(b []byte) error {
	type plain NeighborProbe
	v := plain{Interval: 30}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil || fields == nil {
		return ErrInvalid
	}
	for _, raw := range fields {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return ErrInvalid
		}
	}
	*n = NeighborProbe(v)
	return ValidateNeighbors(n)
}
func ValidInterface(s string) bool {
	if len(s) < 1 || len(s) > 15 || s == "." || s == ".." {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.-", c) {
			return false
		}
	}
	return true
}
func ValidateNeighbors(n *NeighborProbe) error {
	if n == nil {
		return nil
	}
	if (n.FDBPreset != "" && n.FDBPreset != "fnr100") || (n.FDBPreset != "" && n.FDBCommand != "") {
		return &FieldError{Field: "neighbor_probe.fdb_preset", Detail: "型号预设与自定义命令不能同时使用"}
	}
	if n.Interval < 10 || n.Interval > 86400 {
		return &FieldError{Field: "neighbor_probe.interval_seconds", Detail: "刷新周期须为10～86400秒"}
	}
	if len(n.Domains) < 1 || len(n.Domains) > 8 {
		return &FieldError{Field: "neighbor_probe.domains", Detail: "请选择至少一个采集网络，最多8个域"}
	}
	if len(n.FDBCommand) > 4096 || strings.ContainsRune(n.FDBCommand, 0) {
		return &FieldError{Field: "neighbor_probe.fdb_command", Detail: "只读命令最多4096字节且不能包含NUL"}
	}
	ids, interfaces := map[string]bool{}, map[string]bool{}
	for index, d := range n.Domains {
		field := func(suffix, detail string) error {
			return &FieldError{Field: fmt.Sprintf("neighbor_probe.domains[%d].%s", index, suffix), Detail: detail}
		}
		if !ValidKey(d.ID) || len(d.ID) > 32 || ids[d.ID] {
			return field("id", "域ID必须唯一，使用小写字母开头的1～32字符")
		}
		if d.Scope != "lan" && d.Scope != "broadcast" {
			return field("scope", "scope仅支持lan或broadcast")
		}
		if !ValidInterface(d.Interface) {
			return field("interface", "请输入本机IP所在的有效Linux接口名")
		}
		if d.LeaseFile != "" && (!strings.HasPrefix(d.LeaseFile, "/") || len(d.LeaseFile) > 256 || strings.ContainsAny(d.LeaseFile, "\x00\r\n")) {
			return ErrInvalid
		}
		if d.Scope == "lan" && len(d.Ports) == 0 {
			return field("ports", "LAN筛选需要已确认的端口证据")
		}
		if len(d.Ports) > 64 {
			return ErrInvalid
		}
		seen := map[string]bool{}
		for _, port := range d.Ports {
			if !ValidText(port, 128) || seen[port] || strings.ContainsAny(port, "\r\n") {
				return ErrInvalid
			}
			seen[port] = true
		}
		if interfaces[d.Interface] {
			for _, other := range n.Domains {
				if other.ID == d.ID {
					break
				}
				if other.Interface == d.Interface {
					if d.Scope == "broadcast" || other.Scope == "broadcast" {
						if d.Scope == other.Scope {
							return ErrInvalid
						}
						continue
					}
					if len(d.Ports) == 0 || len(other.Ports) == 0 {
						return ErrInvalid
					}
					for _, port := range other.Ports {
						if seen[port] {
							return ErrInvalid
						}
					}
				}
			}
		}
		ids[d.ID] = true
		interfaces[d.Interface] = true
	}
	return nil
}
func CopyNeighbors(n *NeighborProbe) *NeighborProbe {
	if n == nil {
		return nil
	}
	v := *n
	v.Domains = append([]NeighborDomain(nil), n.Domains...)
	for i := range v.Domains {
		v.Domains[i].Ports = append([]string(nil), n.Domains[i].Ports...)
	}
	return &v
}
