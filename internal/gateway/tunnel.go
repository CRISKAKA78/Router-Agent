package gateway

import (
	"context"
	"routerprobe/internal/protocol"
	"routerprobe/internal/tunnel"
)

// SetTunnelStatus is a composition hook; install before starting Serve.
func (s *Server) SetTunnelStatus(report func(string, tunnel.Status) error) { s.tunnelStatus = report }

// BindTunnel exposes a revocable transport handle, never the connection map.
func (s *Server) BindTunnel(deviceID string) (tunnel.Binding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	active := s.sessions[deviceID]
	if active == nil || s.closed {
		return tunnel.Binding{}, tunnel.ErrSession
	}
	d, e := s.devices.Get(deviceID)
	if e != nil {
		return tunnel.Binding{}, e
	}
	capable := false
	for _, c := range d.Registration.Capabilities {
		if c == "tunnel" {
			capable = true
		}
	}
	if !capable {
		return tunnel.Binding{}, tunnel.ErrSession
	}
	return tunnel.Binding{ID: active.sessionID, Done: active.lifetime, Enqueue: func(ctx context.Context, closing bool, command tunnel.Command) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed || s.sessions[deviceID] != active {
			return tunnel.ErrSession
		}
		select {
		case active.tunnelQueue <- tunnelMessage{ctx, closing, command}:
			return nil
		default:
			return tunnel.ErrCapacity
		}
	}}, nil
}

type tunnelMessage struct {
	ctx     context.Context
	closing bool
	command tunnel.Command
}

// One worker per capable control Session, not per maintenance or flow. Slow
// control writes cannot retain released maintenance goroutines or sockets.
func (s *Server) runTunnelControl(active *session) {
	defer close(active.tunnelDone)
	for {
		select {
		case <-active.lifetime:
			return
		default:
		}
		select {
		case <-active.lifetime:
			return
		case message := <-active.tunnelQueue:
			if message.ctx.Err() != nil {
				continue
			}
			typ := protocol.TypeTunnelConnect
			if message.closing {
				typ = protocol.TypeTunnelClose
			}
			_, _ = active.transport.sendJSON(typ, 0, message.command, func(uint64) error {
				if e := message.ctx.Err(); e != nil {
					return e
				}
				s.mu.Lock()
				defer s.mu.Unlock()
				if s.closed || s.sessions[active.deviceID] != active {
					return tunnel.ErrSession
				}
				return nil
			})
		}
	}
}
