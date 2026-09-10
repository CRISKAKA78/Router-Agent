package gateway

import (
	"encoding/json"
	"routerprobe/internal/protocol"
	"slices"
	"time"
)

func (s *Server) requireManaged(id string) error {
	if s.config.Enrollment == nil {
		return nil
	}
	return s.config.Enrollment.RequireManaged(id)
}
func (s *Server) runConfiguration(active *session) {
	defer close(active.configDone)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var last time.Time
	for {
		p, e := s.config.Enrollment.Get(active.deviceID)
		if e == nil && p.Admission == "managed" && p.Configuration.Revision > 0 {
			s.mu.Lock()
			applied := active.appliedConfig
			sent := active.sentConfig
			s.mu.Unlock()
			if applied != p.Configuration.Revision && (sent.Revision != p.Configuration.Revision || time.Since(last) > 10*time.Second) {
				q := p.Configuration
				if q.Template.NeighborProbe != nil && !slices.Contains(active.capabilities, "neighbors_v1") {
					if sent.Revision != q.Revision {
						s.mu.Lock()
						active.sentConfig = q
						s.mu.Unlock()
						s.devices.ApplyConfiguration(active.deviceID, active.sessionID, 0, nil, "unsupported_neighbors")
					}
					last = time.Now()
				} else if q.Template.SwitchProbe != nil && q.Template.SwitchProbe.Counters != nil && !slices.Contains(active.capabilities, "port_counters_v1") {
					if sent.Revision != q.Revision {
						s.mu.Lock()
						active.sentConfig = q
						s.mu.Unlock()
						s.devices.ApplyConfiguration(active.deviceID, active.sessionID, 0, nil, "unsupported_port_counters")
					}
					last = time.Now()
				} else {
					_, err := active.transport.sendJSON(protocol.TypeConfigApply, 0, q, func(id uint64) error {
						s.mu.Lock()
						defer s.mu.Unlock()
						if s.closed || s.sessions[active.deviceID] != active {
							return ErrOffline
						}
						active.sentConfig = q
						active.configMessage = id
						return nil
					})
					if err != nil {
						return
					}
					last = time.Now()
				}
			}
		}
		select {
		case <-active.lifetime:
			return
		case <-tick.C:
		}
	}
}
func (s *Server) acceptConfiguration(active *session, f protocol.Frame) bool {
	var a struct {
		ReplyTo  uint64 `json:"reply_to"`
		Revision uint64 `json:"revision"`
		Success  *bool  `json:"success"`
		Error    string `json:"error"`
	}
	if f.Header.Flags != protocol.FlagResponse || protocol.ValidObject(f.Payload) != nil || json.Unmarshal(f.Payload, &a) != nil || active.configDone == nil || a.ReplyTo == 0 || a.Revision == 0 || a.Success == nil || len(a.Error) > 128 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[active.deviceID] != active {
		return true
	}
	if a.ReplyTo != active.configMessage || a.Revision != active.sentConfig.Revision {
		return true
	}
	if *a.Success {
		active.appliedConfig = a.Revision
		s.devices.ApplyConfiguration(active.deviceID, active.sessionID, a.Revision, &active.sentConfig.Template, "")
	} else {
		if a.Error == "" {
			a.Error = "configuration_rejected"
		}
		s.devices.ApplyConfiguration(active.deviceID, active.sessionID, 0, nil, a.Error)
	}
	return true
}
