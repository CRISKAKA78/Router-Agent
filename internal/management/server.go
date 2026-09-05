package management

import (
	"context"
	"errors"
	"net"
	"routerprobe/internal/gateway"
	"routerprobe/internal/repository"
	"sync"
)

type Config struct {
	RepositoryDirectory string
	Gateway             gateway.Config
}
type Server struct {
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
	return &Server{Service: NewService(r, g.Devices(), g), gateway: g, repo: r}, nil
}
func (s *Server) Serve(l net.Listener) error { return s.gateway.Serve(l) }
func (s *Server) Close() error {
	s.once.Do(func() { s.closeErr = errors.Join(s.gateway.Close(), s.repo.Close()) })
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
