package overlay

import "sort"

type BatchResult struct {
	DeviceID  string     `json:"device_id"`
	Operation *Operation `json:"operation,omitempty"`
	Error     string     `json:"error,omitempty"`
}

func (s *Service) JoinBatch(id string, requests []JoinRequest) ([]BatchResult, error) {
	if len(requests) < 1 || len(requests) > 64 {
		return nil, ErrInvalid
	}
	n, e := s.Get(id)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	ips := map[string]bool{}
	anchor := hasAnchor(n, "")
	for _, q := range requests {
		if q.DeviceID == "" || seen[q.DeviceID] {
			return nil, ErrInvalid
		}
		seen[q.DeviceID] = true
		cfg := DefaultMemberConfig(q.DeviceID)
		cfg.VirtualIP = q.VirtualIP
		if e := cfg.Validate(n, q.DeviceID); e != nil {
			return nil, e
		}
		if q.VirtualIP != "" {
			if ips[q.VirtualIP] {
				return nil, ErrConflict
			}
			ips[q.VirtualIP] = true
			anchor = true
		}
	}
	if n.Profile >= 2 && !anchor {
		return nil, ErrConflict
	}
	// Anchor first; DHCP workers wait for confirmed static membership, not just a saved record.
	requests = append([]JoinRequest(nil), requests...)
	sort.SliceStable(requests, func(i, j int) bool { return requests[i].VirtualIP != "" && requests[j].VirtualIP == "" })
	out := make([]BatchResult, 0, len(requests))
	for _, q := range requests {
		op, e := s.Start(id, q, "start")
		r := BatchResult{DeviceID: q.DeviceID}
		if e == nil {
			r.Operation = &op
		} else {
			r.Error = "member_not_accepted"
		}
		out = append(out, r)
	}
	return out, nil
}
