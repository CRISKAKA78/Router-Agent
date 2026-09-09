package device

import "time"

// A period includes one uninterrupted online interval and its following outage.
type ConnectionPeriod struct {
	ID                                 uint64
	OnlineAt, OfflineAt, ReconnectedAt time.Time
	EndReason                          EndReason
}

type ConnectionHistory struct {
	Periods        []ConnectionPeriod
	Limit          int
	Total, Evicted uint64
}

func (s *Service) beginConnection(r *record, at time.Time) {
	if r.connection != nil {
		previous := *r.connection
		if at.Before(previous.OfflineAt) {
			at = previous.OfflineAt
		}
		previous.ReconnectedAt = at
		if len(r.connections) == s.historyLimit {
			copy(r.connections, r.connections[1:])
			r.connections[len(r.connections)-1] = previous
			r.connectionEvicted++
		} else {
			r.connections = append(r.connections, previous)
		}
	}
	r.connectionTotal++
	r.connection = &ConnectionPeriod{ID: r.connectionTotal, OnlineAt: at}
}

func (s *Service) Connections(id string) (ConnectionHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r := s.devices[id]
	if r == nil {
		return ConnectionHistory{}, ErrNotFound
	}
	h := ConnectionHistory{Limit: s.historyLimit, Total: r.connectionTotal, Evicted: r.connectionEvicted, Periods: []ConnectionPeriod{}}
	if r.connection != nil {
		h.Periods = append(h.Periods, *r.connection)
	}
	for i := len(r.connections) - 1; i >= 0; i-- {
		h.Periods = append(h.Periods, r.connections[i])
	}
	return h, nil
}
