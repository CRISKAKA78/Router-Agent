package gateway

import (
	"context"
	"routerprobe/internal/forwarding"
	"routerprobe/internal/protocol"
)

type forwardingMessage struct {
	ctx     context.Context
	command forwarding.Command
}

func (s *Server) SetForwardingStatus(f func(string, forwarding.Status) error) { s.forwardingStatus = f }
func (s *Server) BindForwarding(id string) (forwarding.Binding, error) {
	if e := s.requireManaged(id); e != nil {
		return forwarding.Binding{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	active := s.sessions[id]
	if s.closed || active == nil || active.forwardingQueue == nil {
		return forwarding.Binding{}, forwarding.ErrUnavailable
	}
	return forwarding.Binding{ID: active.sessionID, Done: active.lifetime, Enqueue: func(ctx context.Context, c forwarding.Command) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.closed || s.sessions[id] != active {
			return forwarding.ErrUnavailable
		}
		select {
		case active.forwardingQueue <- forwardingMessage{ctx, c}:
			return nil
		default:
			return forwarding.ErrCapacity
		}
	}}, nil
}
func (s *Server) runForwardingControl(a *session) {
	for {
		select {
		case <-a.lifetime:
			return
		case m := <-a.forwardingQueue:
			if m.ctx.Err() != nil {
				continue
			}
			a.transport.sendJSON(protocol.TypeForwardingCommand, 0, m.command, func(uint64) error {
				if m.ctx.Err() != nil {
					return m.ctx.Err()
				}
				s.mu.Lock()
				defer s.mu.Unlock()
				if s.closed || s.sessions[a.deviceID] != a {
					return forwarding.ErrUnavailable
				}
				return nil
			})
		}
	}
}
