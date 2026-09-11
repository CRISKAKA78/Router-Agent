package device

import (
	"time"
)

type AtIdentity struct {
	Command string `json:"command"`
	Value   string `json:"value"`
	Status  string `json:"status"`
}
type CellularPort struct {
	Path      string     `json:"path"`
	DeviceKey string     `json:"device_key"`
	Status    string     `json:"status"`
	Reason    string     `json:"reason"`
	Selected  bool       `json:"selected"`
	AgeMS     uint64     `json:"age_ms"`
	ATI       AtIdentity `json:"ati"`
	IMEI      AtIdentity `json:"imei"`
	SampledAt time.Time  `json:"sampled_at"`
}
type Cellular struct {
	Revision  uint64         `json:"config_revision"`
	Interval  uint32         `json:"interval_seconds"`
	Status    string         `json:"status"`
	Reason    string         `json:"reason"`
	Limited   bool           `json:"limited"`
	Ports     []CellularPort `json:"ports"`
	SampledAt time.Time      `json:"sampled_at"`
	Stale     bool           `json:"stale"`
}

func copyCellular(v *Cellular) *Cellular {
	if v == nil {
		return nil
	}
	n := *v
	n.Ports = append([]CellularPort{}, v.Ports...)
	return &n
}
func CellularSnapshot(v Snapshot, now time.Time) *Cellular {
	n := copyCellular(v.LatestSession.Cellular)
	if n != nil {
		n.Stale = v.Status != Online || now.Sub(n.SampledAt) > time.Duration(n.Interval)*3*time.Second
		for i := range n.Ports {
			if n.Ports[i].Selected && now.Sub(n.Ports[i].SampledAt) > time.Duration(n.Interval)*3*time.Second {
				n.Stale = true
			}
			age := now.Sub(n.Ports[i].SampledAt).Milliseconds()
			if age < 0 {
				age = 0
			}
			n.Ports[i].AgeMS = uint64(age)
		}
	}
	return n
}
func (s *Service) ObserveCellular(id, session string, n Cellular, at time.Time, age time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.devices[id]
	if r == nil || r.current == nil || r.current.ID != session || r.current.ConfigRevision != n.Revision || r.current.ConfigTemplate == nil || r.current.ConfigTemplate.CellularProbe == nil || r.current.ConfigTemplate.CellularProbe.Interval != n.Interval {
		return false
	}
	n.SampledAt = at.Add(-age)
	n.Stale = false
	if r.current.Cellular != nil && n.SampledAt.Before(r.current.Cellular.SampledAt) {
		return false
	}
	for i := range n.Ports {
		n.Ports[i].SampledAt = at.Add(-time.Duration(n.Ports[i].AgeMS) * time.Millisecond)
	}
	r.current.Cellular = copyCellular(&n)
	s.revision.Add(1)
	return true
}
