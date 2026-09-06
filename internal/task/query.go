package task

import (
	"sort"
	"time"
)

// Summary deliberately excludes output, environment and dispatch history.
type Summary struct {
	ID        string    `json:"task_id"`
	DeviceID  string    `json:"device_id"`
	Type      string    `json:"type"`
	State     State     `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) List(deviceID, state string, offset, limit int) ([]Summary, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := []string{}
	for id, r := range s.records {
		v := r.snapshot
		if (deviceID == "" || v.Spec.DeviceID == deviceID) && (state == "" || string(v.State) == state) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	total := len(ids)
	out := []Summary{}
	if offset < 0 || limit <= 0 || offset >= total {
		return out, total
	}
	end := offset + limit
	if end < offset || end > total {
		end = total
	}
	for _, id := range ids[offset:end] {
		v := s.records[id].snapshot
		out = append(out, Summary{id, v.Spec.DeviceID, v.Spec.Type, v.State, time.Unix(v.Spec.CreatedAt, 0).UTC()})
	}
	return out, total
}

func (s *Service) Revision() uint64 { return s.revision.Load() }
