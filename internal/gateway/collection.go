package gateway

import (
	"encoding/json"
	"errors"
	"routerprobe/internal/device"
	"routerprobe/internal/probetemplate"
	"routerprobe/internal/protocol"
)

func parseCollection(o map[string]json.RawMessage, m *registerMessage) error {
	fail := errors.New("invalid registration collection snapshot")
	if b, ok := o["template"]; ok {
		if protocol.ValidObject(b) != nil {
			return fail
		}
		var t device.TemplateReference
		if json.Unmarshal(b, &t) != nil || !probetemplate.ValidText(t.ID, 128) || !probetemplate.ValidText(t.Name, 128) || t.Version == 0 {
			return fail
		}
		m.Template = &t
	}
	if b, ok := o["attributes"]; ok {
		if m.Template == nil || protocol.ValidObject(b) != nil || json.Unmarshal(b, &m.Attributes) != nil || len(m.Attributes) > 32 {
			return fail
		}
		for k, a := range m.Attributes {
			if !probetemplate.ValidKey(k) || probetemplate.StandardLimit(k) != 0 || !probetemplate.ValidText(a.Name, 128) || !probetemplate.ValidText(a.Value, 4096) {
				return fail
			}
		}
	}
	if b, ok := o["collection_errors"]; ok {
		if m.Template == nil || protocol.ValidObject(b) != nil || json.Unmarshal(b, &m.CollectionErrors) != nil || len(m.CollectionErrors) > 38 {
			return fail
		}
		for k, e := range m.CollectionErrors {
			if !probetemplate.ValidKey(k) || !probetemplate.ValidText(e.Name, 128) {
				return fail
			}
			if _, ok := m.Attributes[k]; ok {
				return fail
			}
			switch e.Reason {
			case "command_failed", "timeout", "empty", "invalid_output", "budget_exhausted":
			default:
				return fail
			}
		}
	}
	custom := len(m.Attributes)
	for k := range m.CollectionErrors {
		if probetemplate.StandardLimit(k) == 0 {
			custom++
		}
	}
	if custom > 32 {
		return fail
	}
	return nil
}
