package management

import (
	"encoding/json"
	"reflect"
	"routerprobe/internal/device"
	"routerprobe/internal/enrollment"
	"routerprobe/internal/probetemplate"
)

type DeviceUpdate struct {
	Version           uint64                        `json:"version"`
	Admission         string                        `json:"admission"`
	Name              string                        `json:"name"`
	ModelID           string                        `json:"model_id"`
	TemplateID        string                        `json:"template_id"`
	TemplateVersion   uint64                        `json:"template_version"`
	ApplyTemplate     bool                          `json:"apply_template"`
	Monitoring        *probetemplate.Monitoring     `json:"monitoring"`
	PropertyIntervals map[string]uint32             `json:"property_intervals"`
	InterfaceSampling *enrollment.InterfaceSampling `json:"interface_sampling"`
}

func (s *Server) Enrollment() *enrollment.Service { return s.enrollment }
func (s *Server) Inventory() []device.Snapshot {
	out := []device.Snapshot{}
	for _, p := range s.enrollment.List() {
		d, e := s.Devices().Get(p.DeviceID)
		if e != nil {
			d = device.Snapshot{Registration: p.Reported, Status: device.Offline, FirstSeenAt: p.FirstSeen, LastSeenAt: p.LastSeen, LatestSession: device.Session{Registration: p.Reported}}
		}
		out = append(out, d)
	}
	return out
}
func (s *Server) UpdateDevice(id string, q DeviceUpdate) (enrollment.Profile, error) {
	s.enrollmentMu.Lock()
	defer s.enrollmentMu.Unlock()
	p, e := s.enrollment.Get(id)
	if e != nil {
		return p, e
	}
	if p.Version != q.Version {
		return p, probetemplate.ErrConflict
	}
	// Legacy overrides are read-only until an explicit template application.
	if q.Monitoring != nil && !reflect.DeepEqual(q.Monitoring, p.Monitoring) || len(q.PropertyIntervals) != 0 && !reflect.DeepEqual(q.PropertyIntervals, p.PropertyIntervals) {
		return p, probetemplate.ErrInvalid
	}
	previousTemplate := ""
	if p.BoundTemplate != nil {
		previousTemplate = p.BoundTemplate.ID
	}
	apply := q.ApplyTemplate || previousTemplate != q.TemplateID || p.Configuration.Revision == 0
	p.Admission = q.Admission
	p.Name = q.Name
	p.ModelID = q.ModelID
	p.ModelName = ""
	if q.ModelID != "" {
		found := false
		for _, m := range s.enrollment.Models() {
			if m.ID == q.ModelID {
				found = true
				p.ModelName = m.Name
			}
		}
		if !found {
			return p, probetemplate.ErrInvalid
		}
	}
	if q.TemplateID == "" {
		p.BoundTemplate = nil
	} else if q.ApplyTemplate || p.BoundTemplate == nil || p.BoundTemplate.ID != q.TemplateID {
		t, err := s.templates.Resolve(q.TemplateID, "")
		if err != nil {
			return p, err
		}
		if q.TemplateVersion != 0 && t.Version != q.TemplateVersion {
			return p, probetemplate.ErrConflict
		}
		p.BoundTemplate = &t
	}
	if apply {
		if p.Configuration.TemplateGeneration == ^uint64(0) {
			return p, probetemplate.ErrConflict
		}
		p.Configuration.TemplateGeneration++
		p.Monitoring, p.PropertyIntervals = nil, nil
	}
	t := probetemplate.Template{ID: "builtin", Name: "内置采集", Version: 1, Properties: map[string]probetemplate.Property{}}
	if p.BoundTemplate != nil {
		t = *p.BoundTemplate
		b, _ := json.Marshal(t)
		var copied probetemplate.Template
		_ = json.Unmarshal(b, &copied)
		t = copied
	}
	if p.Monitoring != nil {
		copied := *p.Monitoring
		t.Monitoring = &copied
	}
	for key, interval := range p.PropertyIntervals {
		v, ok := t.Properties[key]
		if !ok || interval > 86400 {
			return p, probetemplate.ErrInvalid
		}
		v.Interval = interval
		t.Properties[key] = v
	}
	p.InterfaceSampling = q.InterfaceSampling
	if q.InterfaceSampling != nil {
		m := probetemplate.Monitoring{CPU: 5, Memory: 5, Disk: 60, Network: 5, Egress: 600}
		if t.Monitoring != nil {
			m = *t.Monitoring
		}
		if q.InterfaceSampling.NetworkSeconds != nil {
			m.Network = *q.InterfaceSampling.NetworkSeconds
		}
		if q.InterfaceSampling.NetworkInterfaces != nil {
			m.NetworkInterfaces = q.InterfaceSampling.NetworkInterfaces
		}
		t.Monitoring = &m
	}
	if e := probetemplate.Validate(probetemplate.Input{Name: t.Name, Properties: t.Properties, Monitoring: t.Monitoring, Presentation: t.Presentation, SwitchProbe: t.SwitchProbe}); e != nil {
		// The built-in empty template is a valid server-owned collection plan.
		if t.ID != "builtin" || t.Monitoring != nil {
			return p, e
		}
	}
	before, _ := json.Marshal(p.Configuration.Template)
	after, _ := json.Marshal(t)
	if string(before) != string(after) || apply {
		if p.Configuration.Revision == ^uint64(0) {
			return p, probetemplate.ErrConflict
		}
		p.Configuration.Revision++
		p.Configuration.Template = t
	}
	wire, err := json.Marshal(p.Configuration)
	if err != nil || len(wire) > 65536 {
		return p, probetemplate.ErrInvalid
	}
	return s.enrollment.Put(p, q.Version)
}
func (s *Server) PutModel(m enrollment.Model) (enrollment.Model, error) {
	s.enrollmentMu.Lock()
	defer s.enrollmentMu.Unlock()
	if m.TemplateID != "" {
		if _, e := s.templates.Resolve(m.TemplateID, ""); e != nil {
			return m, e
		}
	}
	return s.enrollment.PutModel(m)
}
func (s *Server) DeleteTemplate(id string, version uint64) error {
	s.enrollmentMu.Lock()
	defer s.enrollmentMu.Unlock()
	if s.enrollment.Referenced(id) {
		return probetemplate.ErrConflict
	}
	return s.templates.Delete(id, version)
}
