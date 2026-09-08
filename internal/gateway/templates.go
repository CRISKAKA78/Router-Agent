package gateway

import (
	"encoding/json"
	"errors"
	"routerprobe/internal/probetemplate"
	"routerprobe/internal/protocol"
)

func (s *Server) handleTemplate(w *connectionWriter, f protocol.Frame) {
	reply := map[string]any{"reply_to": f.Header.MessageID, "success": false, "error_code": "INVALID_TEMPLATE_REQUEST"}
	var q struct {
		ID   string `json:"template_id"`
		Name string `json:"name"`
	}
	if f.Header.Flags == 0 && len(f.Payload) <= 1024 && protocol.ValidObject(f.Payload) == nil && json.Unmarshal(f.Payload, &q) == nil && ((q.ID == "") != (q.Name == "")) && len(q.ID) <= 128 && len(q.Name) <= 128 {
		if s.config.Templates == nil {
			reply["error_code"] = "TEMPLATES_UNAVAILABLE"
		} else {
			v, e := s.config.Templates.Resolve(q.ID, q.Name)
			if e == nil {
				reply = map[string]any{"reply_to": f.Header.MessageID, "success": true, "template": v, "max_control_payload": s.config.MaxControlPayload}
			} else if errors.Is(e, probetemplate.ErrNotFound) {
				reply["error_code"] = "TEMPLATE_NOT_FOUND"
			} else {
				reply["error_code"] = "TEMPLATES_UNAVAILABLE"
			}
		}
	}
	b, _ := json.Marshal(reply)
	if len(b) > int(s.config.MaxControlPayload) {
		reply = map[string]any{"reply_to": f.Header.MessageID, "success": false, "error_code": "TEMPLATE_TOO_LARGE"}
	}
	_, _ = w.sendJSON(protocol.TypeTemplateReply, protocol.FlagResponse, reply, nil)
}
