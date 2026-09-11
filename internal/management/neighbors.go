package management

import (
	"context"
	"routerprobe/internal/device"
	"routerprobe/internal/task"
	"time"
)

func (s *Server) Neighbors(id string) (*device.Neighbors, error) {
	v, e := s.Devices().Get(id)
	if e != nil {
		return nil, e
	}
	return device.NeighborSnapshot(v, time.Now()), nil
}
func (s *Server) CreateNeighbor(ctx context.Context, id string, p task.NeighborRequest, cancel bool) (string, error) {
	if _, e := s.Devices().Get(id); e != nil {
		return "", e
	}
	return s.gateway.CreateNeighbor(ctx, id, p, cancel)
}

func (s *Server) InspectNeighbors(ctx context.Context, id string, p task.NeighborInspectRequest) (string, error) {
	return s.gateway.InspectNeighbors(ctx, id, p)
}
