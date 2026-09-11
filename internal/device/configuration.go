package device

import (
	"encoding/json"
	"routerprobe/internal/probetemplate"
	"time"
)

func copyTemplate(t *probetemplate.Template) *probetemplate.Template {
	if t == nil {
		return nil
	}
	b, _ := json.Marshal(t)
	var n probetemplate.Template
	_ = json.Unmarshal(b, &n)
	return &n
}
func (s *Service) ApplyConfiguration(id, session string, revision uint64, t *probetemplate.Template, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.devices[id]
	if r == nil || r.current == nil || r.current.ID != session {
		return
	}
	current := r.current
	current.ConfigError = reason
	if t != nil && revision != current.ConfigRevision {
		current.ConfigRevision = revision
		current.ConfigTemplate = copyTemplate(t)
		current.Telemetry = emptyTelemetry()
		current.Neighbors = nil
		current.Cellular = nil
		r.recentNeighbors = nil
		r.neighborDiscovery = nil
		r.neighborDiscoveryOrder = 0
		for key, p := range t.Properties {
			current.Telemetry.Template[key] = Metric{Name: p.Name, Unit: MetricUnit(key), Source: "template", Group: "template", Interval: p.Interval, Status: "waiting", SampledAt: time.Time{}}
		}
	}
	s.revision.Add(1)
}
