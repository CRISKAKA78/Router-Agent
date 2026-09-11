package task

import (
	"encoding/json"
	"routerprobe/internal/devicelog"
	"time"
)

func (s *Service) NewDeviceLog(deviceID string, p devicelog.Params) (Spec, error) {
	if deviceID == "" || p.Validate() != nil {
		return Spec{}, devicelog.ErrInvalid
	}
	raw, e := json.Marshal(p)
	if e != nil {
		return Spec{}, e
	}
	id, e := newTaskID()
	if e != nil {
		return Spec{}, e
	}
	spec := Spec{ID: id, DeviceID: deviceID, Type: "device_logs", CreatedAt: time.Now().Unix(), Timeout: 30, Params: raw}
	s.mu.Lock()
	s.records[id] = &record{snapshot: Snapshot{Spec: spec, State: StateReceived}, done: make(chan struct{})}
	s.mu.Unlock()
	s.revision.Add(1)
	spec.Params = append(json.RawMessage(nil), raw...)
	return spec, nil
}
