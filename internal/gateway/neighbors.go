package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
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
			if count > 256 || (r.ActiveAgeMS != nil && *r.ActiveAgeMS > 60000) || e != nil || len(mac) != 6 || mac[0]&1 != 0 || r.MAC == "00:00:00:00:00:00" || mac.String() != r.MAC || keys[key] || len(r.IP) > 45 || (r.IP != "" && net.ParseIP(r.IP) == nil) || len(r.Port) > 128 || len(r.Hostname) > 128 || len(r.Source) > 64 || strings.ContainsAny(r.Port+r.Hostname+r.Source, "\x00\r\n") || !slices.Contains([]string{"cached", "reachable", "lease", "mac_only", "responded"}, r.State) {
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
	baseline := []string{}
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
				for _, row := range device.RecentNeighborSnapshot(d, time.Now()) {
					if row.Interface == domain.Interface {
						baseline = append(baseline, row.IP+"/"+row.MAC)
					}
				}
				found = true
			}
		}
		if found && slices.Contains(active.capabilities, "neighbors_inspect_v1") && !device.NeighborRangeOnLink(d, p.DomainID, p.CIDR, time.Now()) {
			return "", &probetemplate.FieldError{Field: "cidr", Detail: "范围不属于当前已验证直连网络，或网络检测已过期"}
		}
		if !found {
			return "", probetemplate.ErrInvalid
		}
	}
	spec, e := s.tasks.NewNeighbor(id, p, cancel)
	if e != nil {
		return "", e
	}
	s.tasks.SetNeighborBaseline(spec.ID, baseline)
	message, e := s.dispatchChecked(active, spec, true)
	if e != nil && message == 0 {
		s.tasks.Remove(spec.ID)
		return "", e
	}
	return spec.ID, e
}

func (s *Server) InspectNeighbors(ctx context.Context, id string, p task.NeighborInspectRequest) (string, error) {
	d, e := s.devices.Get(id)
	if e != nil {
		return "", e
	}
	if d.CurrentSession == nil {
		return "", ErrOffline
	}
	if p.SessionID != d.CurrentSession.ID || p.Revision != d.CurrentSession.ConfigRevision {
		return "", ErrSessionChanged
	}
	if !slices.Contains(d.Registration.Capabilities, "neighbors_inspect_v1") {
		return "", routerconfig.ErrUnsupported
	}
	if p.VendorTest && !device.FNR100Model(d) {
		return "", &probetemplate.FieldError{Field: "vendor_test", Detail: "参考设备型号不是已支持的FNR100"}
	}
	if e := ctx.Err(); e != nil {
		return "", e
	}
	s.mu.Lock()
	active := s.sessions[id]
	s.mu.Unlock()
	if active == nil || active.sessionID != p.SessionID {
		return "", ErrSessionChanged
	}
	spec, e := s.tasks.NewNeighborInspect(id, p)
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
func (s *Server) observeNeighborResult(active *session, result task.Result) {
	if result.Status != "success" || result.Truncated {
		return
	}
	v, e := s.tasks.Snapshot(result.TaskID)
	if e != nil || len(v.Dispatches) != 1 || v.Dispatches[0].SessionID != active.sessionID {
		return
	}
	if v.Spec.Type == "neighbor_scan" {
		var p task.NeighborRequest
		var payload struct {
			Rows      []device.NeighborRow `json:"rows"`
			Responses int                  `json:"responses"`
		}
		if json.Unmarshal(v.Spec.Params, &p) != nil || json.Unmarshal([]byte(result.Stdout), &payload) != nil || payload.Rows == nil || payload.Responses != len(payload.Rows) {
			return
		}
		event := struct {
			device.Neighbors
			Event string `json:"event"`
		}{device.Neighbors{Revision: p.Revision, Interval: 30, Unclassified: []device.NeighborRow{}, Domains: []device.NeighborDomain{{ID: p.DomainID, Scope: "broadcast", Interface: "br0", Status: "ok", Rows: payload.Rows}}}, "neighbors"}
		raw, _ := json.Marshal(event)
		if _, _, e := parseNeighbors(raw); e != nil {
			return
		}
		rangePrefix, rangeError := netip.ParsePrefix(p.CIDR)
		if rangeError != nil {
			return
		}
		for i := range payload.Rows {
			row := &payload.Rows[i]
			ip, e := netip.ParseAddr(row.IP)
			if e != nil || !rangePrefix.Contains(ip) {
				return
			}
			if row.IP == "" || row.Source != "active_arp" || row.State != "responded" {
				return
			}
			if row.ActiveAgeMS == nil {
				zero := uint64(0)
				row.ActiveAgeMS = &zero
			}
		}
		_, _, ok := s.devices.ObserveNeighborScan(active.deviceID, active.sessionID, p.Revision, p.DomainID, payload.Rows, time.Now())
		if ok {
			known := map[string]bool{}
			for _, key := range v.NeighborBaseline {
				known[key] = true
			}
			added, updated := 0, 0
			for _, row := range payload.Rows {
				if known[row.IP+"/"+row.MAC] {
					updated++
				} else {
					added++
				}
			}
			s.tasks.SetNeighborSummary(result.TaskID, task.NeighborSummary{Responses: len(payload.Rows), Added: added, Updated: updated})
		}
	}

	if v.Spec.Type == "neighbor_inspect" {
		var p task.NeighborInspectRequest
		if json.Unmarshal(v.Spec.Params, &p) != nil {
			return
		}
		n, e := device.ParseNeighborDiscovery(result.Stdout)
		if e == nil {
			s.devices.ObserveNeighborDiscovery(active.deviceID, p.SessionID, p.Revision, n, time.Now(), v.Dispatches[0].MessageID)
		}
	}
}
