package management

import (
	"context"
	"routerprobe/internal/task"
)

// Application entry points delegate to the existing Task/Device services.
func (s *Server) CreateExec(ctx context.Context, deviceID string, q task.ExecRequest) (string, error) {
	if _, err := s.Devices().Get(deviceID); err != nil {
		return "", err
	}
	return s.gateway.CreateExec(ctx, deviceID, q)
}
func (s *Server) Disconnect(deviceID string) (bool, error) {
	if _, err := s.Devices().Get(deviceID); err != nil {
		return false, err
	}
	return s.gateway.Disconnect(deviceID), nil
}
func (s *Server) Tasks(deviceID, state string, offset, limit int) ([]task.Summary, int) {
	return s.gateway.Tasks(deviceID, state, offset, limit)
}
func (s *Server) Revisions() [3]uint64 {
	v := s.gateway.Revisions()
	v[2] += s.Files().Revision()
	return v
}
