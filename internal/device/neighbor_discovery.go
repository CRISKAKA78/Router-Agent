package device

import (
	"encoding/json"
	"net/netip"
	"routerprobe/internal/probetemplate"
	"slices"
	"strings"
	"time"
)

type NeighborNetwork struct {
	Interface string   `json:"interface"`
	Bridge    bool     `json:"bridge"`
	VLAN      bool     `json:"vlan"`
	Master    string   `json:"master"`
	Eligible  bool     `json:"eligible"`
	Reason    string   `json:"reason"`
	IPv4      []string `json:"ipv4"`
	Networks  []string `json:"networks"`
	Ports     []string `json:"ports"`
}
type NeighborDiscovery struct {
	Networks     []NeighborNetwork `json:"networks"`
	Preset       string            `json:"preset"`
	PresetStatus string            `json:"preset_status"`
	RawSummary   string            `json:"raw_summary"`
	Ports        []string          `json:"ports"`
	Revision     uint64            `json:"config_revision"`
	SessionID    string            `json:"session_id"`
	DetectedAt   time.Time         `json:"detected_at"`
	Stale        bool              `json:"stale"`
}

func copyDiscovery(n *NeighborDiscovery) *NeighborDiscovery {
	if n == nil {
		return nil
	}
	b, _ := json.Marshal(n)
	var out NeighborDiscovery
	_ = json.Unmarshal(b, &out)
	return &out
}
func NeighborDiscoverySnapshot(v Snapshot, at time.Time) *NeighborDiscovery {
	n := copyDiscovery(v.NeighborDiscovery)
	if n != nil {
		n.Stale = v.Status != Online || at.Sub(n.DetectedAt) > 90*time.Second || n.Revision != v.LatestSession.ConfigRevision || n.SessionID != v.LatestSession.ID
	}
	return n
}
func ParseNeighborDiscovery(raw string) (NeighborDiscovery, error) {
	var n NeighborDiscovery
	if len(raw) > 24576 || json.Unmarshal([]byte(raw), &n) != nil || n.Networks == nil || len(n.Networks) > 64 || len(n.RawSummary) > 4096 || len(n.Ports) > 5 || !slices.Contains([]string{"not_tested", "verified", "unavailable", "failed"}, n.PresetStatus) {
		return n, probetemplate.ErrInvalid
	}
	seen := map[string]bool{}
	for i := range n.Networks {
		v := &n.Networks[i]
		if !probetemplate.ValidInterface(v.Interface) || seen[v.Interface] || len(v.IPv4) > 8 || len(v.Ports) > 64 || len(v.Reason) > 128 || (v.Master != "" && !probetemplate.ValidInterface(v.Master)) {
			return n, probetemplate.ErrInvalid
		}
		seen[v.Interface] = true
		v.Networks = []string{}
		for _, a := range v.IPv4 {
			p, e := netip.ParsePrefix(a)
			if e != nil || !p.Addr().Is4() {
				return n, probetemplate.ErrInvalid
			}
			cidr := p.Masked().String()
			if !slices.Contains(v.Networks, cidr) {
				v.Networks = append(v.Networks, cidr)
			}
		}
		if v.Master != "" {
			v.Eligible = false
			v.Reason = "bridge_member_use_master"
		}
		for _, p := range v.Ports {
			if !probetemplate.ValidInterface(p) {
				return n, probetemplate.ErrInvalid
			}
		}
	}
	for _, p := range n.Ports {
		if !slices.Contains([]string{"lan1", "lan2", "lan3", "lan4", "wan"}, p) {
			return n, probetemplate.ErrInvalid
		}
	}
	if n.Preset != "" && n.Preset != "fnr100" {
		return n, probetemplate.ErrInvalid
	}
	return n, nil
}
func (s *Service) ObserveNeighborDiscovery(id, session string, revision uint64, n NeighborDiscovery, at time.Time, order ...uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.devices[id]
	if r == nil || r.current == nil || r.current.ID != session || r.current.ConfigRevision != revision {
		return false
	}
	if r.neighborDiscovery != nil && r.neighborDiscovery.DetectedAt.After(at) {
		return false
	}
	if len(order) > 0 {
		if order[0] <= r.neighborDiscoveryOrder {
			return false
		}
		r.neighborDiscoveryOrder = order[0]
	}
	n.SessionID = session
	n.Revision = revision
	n.DetectedAt = at
	r.neighborDiscovery = copyDiscovery(&n)
	s.revision.Add(1)
	return true
}
func NeighborRangeOnLink(v Snapshot, domain, cidr string, at time.Time) bool {
	n := NeighborDiscoverySnapshot(v, at)
	if n == nil || n.Stale {
		return false
	}
	p, e := netip.ParsePrefix(cidr)
	if e != nil || !p.Addr().Is4() || p.Bits() < 24 || p != p.Masked() {
		return false
	}
	if v.LatestSession.ConfigTemplate == nil || v.LatestSession.ConfigTemplate.NeighborProbe == nil {
		return false
	}
	for _, d := range v.LatestSession.ConfigTemplate.NeighborProbe.Domains {
		if d.ID != domain {
			continue
		}
		for _, net := range n.Networks {
			if net.Interface != d.Interface || !net.Eligible {
				continue
			}
			for _, s := range net.Networks {
				q, err := netip.ParsePrefix(s)
				if err == nil && q.Bits() <= p.Bits() && q.Contains(p.Addr()) {
					return true
				}
			}
		}
	}
	return false
}
func FNR100Model(v Snapshot) bool {
	return strings.EqualFold(strings.TrimSpace(v.Registration.Model), "FNR100")
}
