package gateway

import (
	"context"
	"routerprobe/internal/routerconfig"
)

func supportsRouterConfig(active *session) bool {
	for _, c := range active.capabilities {
		if c == "router_config" {
			return true
		}
	}
	return false
}

func (s *Server) CreateRouterConfig(ctx context.Context, deviceID string, p routerconfig.Params, timeout uint32) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.Lock()
	active := s.sessions[deviceID]
	s.mu.Unlock()
	if active == nil {
		return "", ErrOffline
	}
	if !supportsRouterConfig(active) {
		return "", routerconfig.ErrUnsupported
	}
	spec, err := s.tasks.NewRouterConfig(deviceID, p, timeout)
	if err != nil {
		return "", err
	}
	messageID, err := s.dispatchChecked(active, spec, true)
	if err != nil && messageID == 0 {
		s.tasks.Remove(spec.ID)
		return "", err
	}
	return spec.ID, err
}
