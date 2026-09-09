package probetemplate

import (
	"encoding/json"
	"strconv"
)

type DisplayGroup struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}
type DisplayField struct {
	GroupID string `json:"group_id"`
	Order   int    `json:"order"`
	Visible *bool  `json:"visible,omitempty"`
}
type Presentation struct {
	StorageVisible    *bool                   `json:"storage_visible,omitempty"`
	Groups            []DisplayGroup          `json:"groups"`
	Fields            map[string]DisplayField `json:"fields"`
	BuiltinVisibility map[string]bool         `json:"builtin_visibility"`
}

var BuiltinGroups = map[string]string{
	"builtin_system": "系统信息", "builtin_resources": "资源监控",
	"builtin_interfaces": "接口信息", "other": "其他信息",
}

type SwitchPort struct {
	ID          string `json:"id"`
	SwitchID    string `json:"switch_id"`
	Role        string `json:"role,omitempty"`
	Port        *int   `json:"port,omitempty"`
	SystemName  string `json:"system_name"`
	Uplink      string `json:"uplink"`
	DisplayName string `json:"display_name"`
}
type SwitchProbe struct {
	Backend  string        `json:"backend"`
	Command  string        `json:"command,omitempty"`
	Ports    []SwitchPort  `json:"ports"`
	Counters *PortCounters `json:"counters,omitempty"`
}

// Sources are cumulative per-port bytes, never interface estimates or read-clear counters.
type PortCounters struct {
	Backend string `json:"backend"`
	Command string `json:"command,omitempty"`
	RXField string `json:"rx_field,omitempty"`
	TXField string `json:"tx_field,omitempty"`
	Bits    uint32 `json:"bits"`
	Basis   string `json:"basis"`
}

func CopyPresentation(v *Presentation) *Presentation {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	var n Presentation
	_ = json.Unmarshal(b, &n)
	if n.Groups == nil {
		n.Groups = []DisplayGroup{}
	}
	if n.Fields == nil {
		n.Fields = map[string]DisplayField{}
	}
	if n.BuiltinVisibility == nil {
		n.BuiltinVisibility = map[string]bool{}
	}
	return &n
}
func CopySwitch(v *SwitchProbe) *SwitchProbe {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	var n SwitchProbe
	_ = json.Unmarshal(b, &n)
	if n.Ports == nil {
		n.Ports = []SwitchPort{}
	}
	return &n
}
func ValidatePresentation(p *Presentation, s *SwitchProbe) error {
	if p != nil {
		if len(p.Groups) > 64 || len(p.Fields) > 1024 || len(p.BuiltinVisibility) > 7 {
			return ErrInvalid
		}
		ids := map[string]bool{}
		names := map[string]bool{}
		for _, g := range p.Groups {
			if !keyPattern.MatchString(g.ID) || BuiltinGroups[g.ID] != "" || names[g.Name] || !ValidText(g.Name, 128) || ids[g.ID] || g.Order < 0 || g.Order > 1000000 {
				return ErrInvalid
			}
			for _, name := range BuiltinGroups {
				if g.Name == name {
					return ErrInvalid
				}
			}
			ids[g.ID] = true
			names[g.Name] = true
		}
		for k, f := range p.Fields {
			if !keyPattern.MatchString(k) || f.GroupID != "" && !ids[f.GroupID] && BuiltinGroups[f.GroupID] == "" || f.Order < 0 || f.Order > 1000000 {
				return ErrInvalid
			}
		}
		for k := range p.BuiltinVisibility {
			if k != "hardware" && k != "cpu" && k != "memory" && k != "disk" && k != "network" && k != "switch" && k != "egress" {
				return ErrInvalid
			}
		}
	}
	if s != nil {
		if len(s.Ports) > 64 {
			return ErrInvalid
		}
		switch s.Backend {
		case "auto", "dsa", "swconfig":
			if s.Command != "" {
				return ErrInvalid
			}
		case "command":
			if !ValidText(s.Command, 4096) {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
		ids := map[string]bool{}
		sources := map[string]bool{}
		for _, p := range s.Ports {
			if p.Role != "" && p.Role != "external" && p.Role != "cpu" {
				return ErrInvalid
			}
			source := "system:" + p.SystemName
			if p.SwitchID != "" {
				if p.Port == nil {
					return ErrInvalid
				}
				source = p.SwitchID + ":" + strconv.Itoa(*p.Port)
			}
			if p.SwitchID == "" {
				source = "system:" + p.SystemName
			}
			if source != "system:" && sources[source] {
				return ErrInvalid
			}
			sources[source] = true
			if !keyPattern.MatchString(p.ID) || len(p.ID) > 32 || ids[p.ID] || p.Port != nil && (*p.Port < 0 || *p.Port > 255) || len(p.SwitchID) > 64 || len(p.SystemName) > 15 || len(p.Uplink) > 15 || len(p.DisplayName) > 128 {
				return ErrInvalid
			}
			ids[p.ID] = true
		}
		if c := s.Counters; c != nil {
			if len(s.Ports) == 0 || len(s.Ports) > 16 || c.Bits != 32 && c.Bits != 64 || !ValidText(c.Basis, 128) {
				return ErrInvalid
			}
			switch c.Backend {
			case "swconfig_mib":
				if c.Command != "" || !ValidText(c.RXField, 64) || !ValidText(c.TXField, 64) || c.RXField == c.TXField {
					return ErrInvalid
				}
				for _, p := range s.Ports {
					if !safeSwitchName(p.SwitchID) || p.Port == nil {
						return ErrInvalid
					}
				}
			case "command":
				if !ValidText(c.Command, 4096) || c.RXField != "" || c.TXField != "" {
					return ErrInvalid
				}
			default:
				return ErrInvalid
			}
		}
	}
	return nil
}

func safeSwitchName(s string) bool {
	if s == "" || len(s) > 32 || s == "." || s == ".." {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}
