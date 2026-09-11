package gateway

import (
	"context"
	"routerprobe/internal/overlay"
	"routerprobe/internal/routerconfig"
	"slices"
)

func (s *Server) CreateNetworkAgent(ctx context.Context, id string, p overlay.AgentRequest) (string, error) {
	if e := p.Validate(); e != nil {
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
	if !slices.Contains(active.capabilities, "network_agent_v1") {
		return "", routerconfig.ErrUnsupported
	}
	spec, e := s.tasks.NewNetworkAgent(id, p)
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
