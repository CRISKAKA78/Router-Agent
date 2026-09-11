package task

import (
	"encoding/json"
	"net/netip"
	"routerprobe/internal/probetemplate"
	"time"
)

type NeighborRequest struct {
	DomainID     string `json:"domain_id,omitempty"`
	CIDR         string `json:"cidr,omitempty"`
	Revision     uint64 `json:"config_revision,omitempty"`
	TargetTaskID string `json:"target_task_id,omitempty"`
}

func (p NeighborRequest) Validate(cancel bool) error {
	if cancel {
		if p.TargetTaskID == "" || len(p.TargetTaskID) > 128 || p.DomainID != "" || p.CIDR != "" || p.Revision != 0 {
			return probetemplate.ErrInvalid
		}
		return nil
	}
	prefix, e := netip.ParsePrefix(p.CIDR)
	if e != nil || !prefix.Addr().Is4() || prefix.Bits() < 24 || p.CIDR != prefix.Masked().String() || !probetemplate.ValidKey(p.DomainID) || len(p.DomainID) > 32 || p.Revision == 0 || p.TargetTaskID != "" {
		return probetemplate.ErrInvalid
	}
	return nil
}
func (s *Service) NewNeighbor(deviceID string, p NeighborRequest, cancel bool) (Spec, error) {
	if deviceID == "" {
		return Spec{}, probetemplate.ErrInvalid
	}
	if e := p.Validate(cancel); e != nil {
		return Spec{}, e
	}
	id, e := newTaskID()
	if e != nil {
		return Spec{}, e
	}
	raw, _ := json.Marshal(p)
	kind := "neighbor_scan"
	if cancel {
		kind = "neighbor_cancel"
	}
	spec := Spec{ID: id, DeviceID: deviceID, Type: kind, CreatedAt: time.Now().Unix(), Timeout: 30, Params: raw}
	s.mu.Lock()
	s.records[id] = &record{snapshot: Snapshot{Spec: spec, State: StateReceived}, done: make(chan struct{})}
	s.mu.Unlock()
	s.revision.Add(1)
	spec.Params = append(json.RawMessage(nil), raw...)
	return spec, nil
}

type NeighborInspectRequest struct {
	SessionID  string `json:"session_id"`
	Revision   uint64 `json:"config_revision"`
	VendorTest bool   `json:"vendor_test"`
}

func (s *Service) NewNeighborInspect(deviceID string, p NeighborInspectRequest) (Spec, error) {
	id, e := newTaskID()
	if e != nil {
		return Spec{}, e
	}
	raw, _ := json.Marshal(p)
	spec := Spec{ID: id, DeviceID: deviceID, Type: "neighbor_inspect", CreatedAt: time.Now().Unix(), Timeout: 30, Params: raw}
	s.mu.Lock()
	s.records[id] = &record{snapshot: Snapshot{Spec: spec, State: StateReceived}, done: make(chan struct{})}
	s.mu.Unlock()
	s.revision.Add(1)
	return spec, nil
}
