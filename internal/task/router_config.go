package task

import (
	"encoding/json"
	"routerprobe/internal/routerconfig"
	"time"
)

func (s *Service) NewRouterConfig(deviceID string, p routerconfig.Params, timeout uint32) (Spec, error) {
	if deviceID == "" || timeout < 1 || timeout > 30 {
		return Spec{}, routerconfig.ErrInvalid
	}
	if err := p.Validate(); err != nil {
		return Spec{}, err
	}
	params, err := json.Marshal(p)
	if err != nil {
		return Spec{}, err
	}
	id, err := newTaskID()
	if err != nil {
		return Spec{}, err
	}
	spec := Spec{ID: id, DeviceID: deviceID, Type: "router_config", CreatedAt: time.Now().Unix(), Timeout: timeout, Params: params}
	s.mu.Lock()
	s.records[id] = &record{snapshot: Snapshot{Spec: spec, State: StateReceived}, done: make(chan struct{})}
	s.mu.Unlock()
	s.revision.Add(1)
	spec.Params = append(json.RawMessage(nil), params...)
	return spec, nil
}
