package management

import (
	"context"
	"routerprobe/internal/routerconfig"
)

func (s *Server) CreateRouterConfig(ctx context.Context, deviceID string, p routerconfig.Params, timeout uint32) (string, error) {
	if timeout < 1 || timeout > 30 {
		return "", routerconfig.ErrInvalid
	}
	if err := p.Validate(); err != nil {
		return "", err
	}
	if _, err := s.Devices().Get(deviceID); err != nil {
		return "", err
	}
	return s.gateway.CreateRouterConfig(ctx, deviceID, p, timeout)
}
