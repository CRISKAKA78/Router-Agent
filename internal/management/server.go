package management

import (
	"context"
	"errors"
	"net"
	"routerprobe/internal/gateway"
	"routerprobe/internal/repository"
	"routerprobe/internal/tunnel"
	"sync"
)

type Config struct {
	Tunnel              *tunnel.Config // nil disables data listener for embedded legacy callers
	RepositoryDirectory string
	Gateway             gateway.Config
}
type Server struct {
	maintenance *tunnel.Service
	*Service
	gateway  *gateway.Server
	repo     *repository.Store
	once     sync.Once
	closeErr error
}

func New(config Config) (*Server, error) {
	r, e := repository.Open(config.RepositoryDirectory)
	if e != nil {
		return nil, e
	}
	g, e := gateway.New(config.Gateway)
	if e != nil {
		r.Close()
		return nil, e
	}
	if config.Gateway.Logger != nil {
		config.Gateway.Logger.Printf("repository_dir=%s", r.Directory())
		leftovers, err := r.Leftovers()
		if err != nil {
			g.Close()
			r.Close()
			return nil, err
		}
		for _, p := range leftovers {
			config.Gateway.Logger.Printf("repository_leftover=%s", p)
		}
	}
	var maintenance *tunnel.Service
	if config.Tunnel != nil {
		maintenance, e = tunnel.New(*config.Tunnel, g)
		if e != nil {
			g.Close()
			r.Close()
			return nil, e
		}
		g.SetTunnelStatus(maintenance.Report)
	}
	return &Server{Service: NewService(r, g.Devices(), g), gateway: g, repo: r, maintenance: maintenance}, nil
}
func (s *Server) Maintenance() *tunnel.Service { return s.maintenance }
func (s *Server) Serve(l net.Listener) error   { return s.gateway.Serve(l) }
func (s *Server) Close() error {
	s.once.Do(func() {
		if s.maintenance != nil {
			s.maintenance.Close()
		}
		s.closeErr = errors.Join(s.gateway.Close(), s.repo.Close())
	})
	return s.closeErr
}
func Run(ctx context.Context, address string, config Config) error {
	s, e := New(config)
	if e != nil {
		return e
	}
	defer s.Close()
	l, e := net.Listen("tcp", address)
	if e != nil {
		return e
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			s.Close()
		case <-done:
		}
	}()
	return s.Serve(l)
}
