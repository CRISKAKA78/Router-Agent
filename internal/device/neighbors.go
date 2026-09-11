package device

import (
	"slices"
	"sort"
	"strings"
	"time"
)

type NeighborRow struct {
	ActiveAgeMS *uint64   `json:"active_age_ms,omitempty"`
	ActiveAt    time.Time `json:"active_at,omitempty"`
	Interface   string    `json:"interface,omitempty"`
	IP          string    `json:"ip"`
	MAC         string    `json:"mac"`
	Port        string    `json:"port"`
	Hostname    string    `json:"hostname"`
	Source      string    `json:"source"`
	State       string    `json:"state"`
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
		for j := range v.Domains[i].Rows {
			if age := v.Domains[i].Rows[j].ActiveAgeMS; age != nil {
				value := *age
				v.Domains[i].Rows[j].ActiveAgeMS = &value
			}
		}
	}
	return &v
}
func NeighborSnapshot(v Snapshot, at time.Time) *Neighbors {
	n := copyNeighbors(v.LatestSession.Neighbors)
	if n != nil {
		for i := range n.Domains {
			rows := []NeighborRow{}
			for _, row := range n.Domains[i].Rows {
				if freshenRow(&row, at) {
					rows = append(rows, row)
				}
			}
			n.Domains[i].Rows = rows
		}
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
	s.mergeRecent(r, &n, at)
	r.current.Neighbors = copyNeighbors(&n)
	s.revision.Add(1)
	return true
}

const NeighborHistoryLimit = 1024
const NeighborHistoryTTL = 24 * time.Hour

type RecentNeighbor struct {
	NeighborRow
	DomainID  string    `json:"domain_id"`
	Scope     string    `json:"scope"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	Current   bool      `json:"current"`
}

func freshenRow(row *NeighborRow, at time.Time) bool {
	if strings.Contains(row.Source, "active_arp") && (row.ActiveAt.IsZero() || at.Sub(row.ActiveAt) >= 60*time.Second) {
		parts := []string{}
		for _, p := range strings.Split(row.Source, "+") {
			if p != "active_arp" {
				parts = append(parts, p)
			}
		}
		row.Source = strings.Join(parts, "+")
		if row.State == "responded" {
			row.State = "recent"
		}
		return row.Source != ""
	}
	return true
}
func RecentNeighborSnapshot(v Snapshot, at time.Time) []RecentNeighbor {
	out := []RecentNeighbor{}
	for _, r := range v.RecentNeighbors {
		if at.Sub(r.LastSeen) >= NeighborHistoryTTL {
			continue
		}
		freshenRow(&r.NeighborRow, at)
		if !r.Current || v.Status != Online || v.LatestSession.Neighbors == nil || at.Sub(v.LatestSession.Neighbors.SampledAt) > time.Duration(v.LatestSession.Neighbors.Interval)*3*time.Second {
			r.Current = false
			if r.State != "responded" || v.Status != Online {
				r.State = "recent"
			}
		}
		if r.State == "recent" && r.Source == "" {
			r.Source = "active_arp_history"
		}
		out = append(out, r)
	}
	return out
}
func (s *Service) mergeRecent(r *record, n *Neighbors, at time.Time) {
	entries := map[string]RecentNeighbor{}
	key := func(domain string, row NeighborRow) string { return domain + "/" + row.IP + "/" + row.MAC }
	for _, old := range r.recentNeighbors {
		if at.Sub(old.LastSeen) < NeighborHistoryTTL {
			old.Current = false
			entries[key(old.DomainID, old.NeighborRow)] = old
		}
	}
	for i := range n.Domains {
		d := &n.Domains[i]
		for j := range d.Rows {
			row := &d.Rows[j]
			row.Interface = d.Interface
			if strings.Contains(row.Source, "active_arp") {
				if row.ActiveAgeMS != nil {
					row.ActiveAt = n.SampledAt.Add(-time.Duration(*row.ActiveAgeMS) * time.Millisecond)
				} else if old, ok := entries[key(d.ID, *row)]; ok && !old.ActiveAt.IsZero() {
					row.ActiveAt = old.ActiveAt
				} else {
					row.ActiveAt = n.SampledAt
				}
			}
			k := key(d.ID, *row)
			old, exists := entries[k]
			seen := n.SampledAt
			if row.Source == "active_arp" && !row.ActiveAt.IsZero() {
				seen = row.ActiveAt
			}
			if exists && seen.Before(old.LastSeen) {
				continue
			}
			first := seen
			if exists {
				first = old.FirstSeen
			}
			entries[k] = RecentNeighbor{NeighborRow: *row, DomainID: d.ID, Scope: d.Scope, FirstSeen: first, LastSeen: seen, Current: true}
		}
	}
	r.recentNeighbors = make([]RecentNeighbor, 0, len(entries))
	for _, v := range entries {
		v.ActiveAgeMS = nil
		r.recentNeighbors = append(r.recentNeighbors, v)
	}
	sort.Slice(r.recentNeighbors, func(i, j int) bool {
		a, b := r.recentNeighbors[i], r.recentNeighbors[j]
		if a.LastSeen.Equal(b.LastSeen) {
			return key(a.DomainID, a.NeighborRow) < key(b.DomainID, b.NeighborRow)
		}
		return a.LastSeen.After(b.LastSeen)
	})
	if len(r.recentNeighbors) > NeighborHistoryLimit {
		r.recentNeighbors = r.recentNeighbors[:NeighborHistoryLimit]
	}
}

// A completed scan is an observation, not a replacement for the passive snapshot.
func (s *Service) ObserveNeighborScan(id, session string, revision uint64, domain string, rows []NeighborRow, at time.Time) (added, updated int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.devices[id]
	if r == nil || r.current == nil || r.current.ID != session || r.current.ConfigRevision != revision || r.current.ConfigTemplate == nil || r.current.ConfigTemplate.NeighborProbe == nil {
		return
	}
	iface := ""
	plan := r.current.ConfigTemplate.NeighborProbe
	for _, d := range plan.Domains {
		if d.ID == domain {
			iface = d.Interface
		}
	}
	if iface == "" {
		return
	}
	known := map[string]bool{}
	for _, v := range r.recentNeighbors {
		if v.Interface == iface && at.Sub(v.LastSeen) < NeighborHistoryTTL {
			known[v.IP+"/"+v.MAC] = true
		}
	}
	for _, row := range rows {
		if known[row.IP+"/"+row.MAC] {
			updated++
		} else {
			added++
		}
	}
	n := Neighbors{SampledAt: at}
	for _, d := range plan.Domains {
		if d.Interface == iface && d.Scope == "broadcast" {
			n.Domains = append(n.Domains, NeighborDomain{ID: d.ID, Scope: d.Scope, Interface: d.Interface, Rows: append([]NeighborRow{}, rows...)})
		}
	}
	// Preserve latest passive current flags while merging only new active evidence.
	flags := map[string]bool{}
	for _, v := range r.recentNeighbors {
		flags[v.DomainID+"/"+v.IP+"/"+v.MAC] = v.Current
	}
	s.mergeRecent(r, &n, at)
	for i := range r.recentNeighbors {
		v := &r.recentNeighbors[i]
		if flags[v.DomainID+"/"+v.IP+"/"+v.MAC] {
			v.Current = true
		}
	}
	s.revision.Add(1)
	return added, updated, true
}
