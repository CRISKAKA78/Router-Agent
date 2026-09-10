package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"routerprobe/internal/device"
	"routerprobe/internal/probetemplate"
	"routerprobe/internal/protocol"
	"routerprobe/internal/routerconfig"
	"routerprobe/internal/task"
	"slices"
	"strings"
	"time"
)

func parseNeighbors(raw []byte) (device.Neighbors, time.Duration, error) {
	var v struct {
		device.Neighbors
		Event string `json:"event"`
		Age   uint64 `json:"age_ms"`
	}
	bad := errors.New("invalid neighbors event")
	if len(raw) > 65536 || protocol.ValidObject(raw) != nil || json.Unmarshal(raw, &v) != nil || v.Event != "neighbors" || v.Revision == 0 || v.Interval < 10 || v.Interval > 86400 || v.Age > 86400000 || len(v.Domains) > 8 || len(v.Domains) == 0 {
		return v.Neighbors, 0, bad
	}
	count := 0
	validateRows := func(rows []device.NeighborRow) bool {
		if rows == nil {
			return false
		}
		keys := map[string]bool{}
		for _, r := range rows {
			count++
			mac, e := net.ParseMAC(r.MAC)
			key := r.Interface + "/" + r.IP + "/" + r.MAC
			if count > 256 || e != nil || len(mac) != 6 || mac[0]&1 != 0 || r.MAC == "00:00:00:00:00:00" || mac.String() != r.MAC || keys[key] || len(r.IP) > 45 || (r.IP != "" && net.ParseIP(r.IP) == nil) || len(r.Port) > 128 || len(r.Hostname) > 128 || len(r.Source) > 64 || strings.ContainsAny(r.Port+r.Hostname+r.Source, "\x00\r\n") || !slices.Contains([]string{"cached", "reachable", "lease", "mac_only", "responded"}, r.State) {
				return false
			}
			if r.Interface != "" && !probetemplate.ValidInterface(r.Interface) {
				return false
			}
			keys[key] = true
		}
		return true
	}
	ids := map[string]bool{}
	for _, d := range v.Domains {
		if !probetemplate.ValidKey(d.ID) || len(d.ID) > 32 || ids[d.ID] || (d.Scope != "lan" && d.Scope != "broadcast") || !probetemplate.ValidInterface(d.Interface) || !slices.Contains([]string{"ok", "partial", "error"}, d.Status) || len(d.Reason) > 256 || !validateRows(d.Rows) {
			return v.Neighbors, 0, bad
		}
		ids[d.ID] = true
	}
	if !validateRows(v.Unclassified) {
		return v.Neighbors, 0, bad
	}
	for _, r := range v.Unclassified {
		if !probetemplate.ValidInterface(r.Interface) {
			return v.Neighbors, 0, bad
		}
	}
	return v.Neighbors, time.Duration(v.Age) * time.Millisecond, nil
}
func (s *Server) CreateNeighbor(ctx context.Context, id string, p task.NeighborRequest, cancel bool) (string, error) {
	if e := p.Validate(cancel); e != nil {
		return "", e
	}
	if e := s.requireManaged(id); e != nil {
		return "", e
	}
	if e := ctx.Err(); e != nil {
		return "", e
	}
	s.mu.Lock()
	active := s.sessions[id]
	s.mu.Unlock()
	if active == nil {
		return "", ErrOffline
	}
	if !slices.Contains(active.capabilities, "neighbors_v1") {
		return "", routerconfig.ErrUnsupported
	}
	if cancel {
		original, e := s.tasks.Snapshot(p.TargetTaskID)
		if e != nil {
			return "", e
		}
		if original.Spec.DeviceID != id || original.Spec.Type != "neighbor_scan" {
			return "", probetemplate.ErrInvalid
		}
	} else {
		d, e := s.devices.Get(id)
		if e != nil {
			return "", e
		}
		t := d.LatestSession.ConfigTemplate
		if t == nil || t.NeighborProbe == nil || d.LatestSession.ConfigRevision != p.Revision {
			return "", probetemplate.ErrConflict
		}
		found := false
		for _, domain := range t.NeighborProbe.Domains {
			if domain.ID == p.DomainID {
				found = true
			}
		}
		if !found {
			return "", probetemplate.ErrInvalid
		}
	}
	spec, e := s.tasks.NewNeighbor(id, p, cancel)
	if e != nil {
		return "", e
	}
	message, e := s.dispatchChecked(active, spec, true)
	if e != nil && message == 0 {
		s.tasks.Remove(spec.ID)
		return "", e
	}
	return spec.ID, e
}
