package task

import (
	"encoding/json"
	"routerprobe/internal/overlay"
	"time"
)

func (s *Service) NewNetworkAgent(deviceID string, p overlay.AgentRequest) (Spec, error) {
	if deviceID == "" {
		return Spec{}, overlay.ErrInvalid
	}
	if e := p.Validate(); e != nil {
		return Spec{}, e
	}
	id, e := newTaskID()
	if e != nil {
		return Spec{}, e
	}
	raw, _ := json.Marshal(p)
	spec := Spec{ID: id, DeviceID: deviceID, Type: "network_agent", CreatedAt: time.Now().Unix(), Timeout: 30, Params: raw}
	s.mu.Lock()
	s.records[id] = &record{snapshot: Snapshot{Spec: spec, State: StateReceived}, done: make(chan struct{})}
	s.mu.Unlock()
	s.revision.Add(1)
	spec.Params = append(json.RawMessage(nil), raw...)
	return spec, nil
}
