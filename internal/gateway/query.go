package gateway

import "routerprobe/internal/task"

func (s *Server) Tasks(deviceID, state string, offset, limit int) ([]task.Summary, int) {
	return s.tasks.List(deviceID, state, offset, limit)
}
func (s *Server) Revisions() [3]uint64 {
	return [3]uint64{s.devices.Revision(), s.tasks.Revision(), s.files.Revision()}
}
