package device

import (
	"slices"
	"time"
)

type NeighborRow struct {
	Interface string `json:"interface,omitempty"`
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Port      string `json:"port"`
	Hostname  string `json:"hostname"`
	Source    string `json:"source"`
	State     string `json:"state"`
}
type NeighborDomain struct {
	ID        string        `json:"id"`
	Scope     string        `json:"scope"`
	Interface string        `json:"interface"`
	Status    string        `json:"status"`
	Reason    string        `json:"reason"`
	Limited   bool          `json:"limited"`
	Rows      []NeighborRow `json:"rows"`
}
type Neighbors struct {
	Limited      bool             `json:"limited"`
	Unclassified []NeighborRow    `json:"unclassified"`
	Revision     uint64           `json:"config_revision"`
	Interval     uint32           `json:"interval_seconds"`
	Domains      []NeighborDomain `json:"domains"`
	SampledAt    time.Time        `json:"sampled_at"`
	Stale        bool             `json:"stale"`
}

func copyNeighbors(n *Neighbors) *Neighbors {
	if n == nil {
		return nil
	}
	v := *n
	v.Unclassified = append([]NeighborRow{}, n.Unclassified...)
	v.Domains = append([]NeighborDomain(nil), n.Domains...)
	for i := range v.Domains {
		v.Domains[i].Rows = append([]NeighborRow{}, n.Domains[i].Rows...)
	}
	return &v
}
func NeighborSnapshot(v Snapshot, at time.Time) *Neighbors {
	n := copyNeighbors(v.LatestSession.Neighbors)
	if n != nil {
		n.Stale = v.Status != Online || at.Sub(n.SampledAt) > time.Duration(n.Interval)*3*time.Second
	}
	return n
}
func (s *Service) ObserveNeighbors(id, session string, n Neighbors, at time.Time, age time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.devices[id]
	if r == nil || r.current == nil || r.current.ID != session || r.current.ConfigRevision != n.Revision || r.current.ConfigTemplate == nil || r.current.ConfigTemplate.NeighborProbe == nil {
		return false
	}
	plan := r.current.ConfigTemplate.NeighborProbe
	if n.Interval != plan.Interval || len(n.Domains) != len(plan.Domains) {
		return false
	}
	for i, d := range n.Domains {
		p := plan.Domains[i]
		if d.ID != p.ID || d.Scope != p.Scope || d.Interface != p.Interface {
			return false
		}
		if p.Scope == "lan" {
			for _, r := range d.Rows {
				if !slices.Contains(p.Ports, r.Port) {
					return false
				}
			}
		}
	}
	for _, r := range n.Unclassified {
		found := false
		for _, d := range plan.Domains {
			if d.Interface == r.Interface {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	n.SampledAt = at.Add(-age)
	r.current.Neighbors = copyNeighbors(&n)
	s.revision.Add(1)
	return true
}
