package management

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"routerprobe/internal/devicelog"
	"routerprobe/internal/enrollment"
	"routerprobe/internal/forwarding"
	"routerprobe/internal/gateway"
	"routerprobe/internal/overlay"
	"routerprobe/internal/probetemplate"
	"routerprobe/internal/repository"
	"routerprobe/internal/tunnel"
	"sync"
)

type Config struct {
	EasyTier            overlay.Config
	Forwarding          *forwarding.Config
	TemplateFile        string
	Tunnel              *tunnel.Config // nil disables the optional data listener
	RepositoryDirectory string
	Gateway             gateway.Config
}
type Server struct {
	logs         *devicelog.Service
	networks     *overlay.Service
	forwarding   *forwarding.Service
	enrollment   *enrollment.Service
	enrollmentMu sync.Mutex
	templates    *probetemplate.Service
	maintenance  *tunnel.Service
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
	templateFile := config.TemplateFile
	if templateFile == "" {
		templateFile = filepath.Join(r.Directory(), "probe-templates", "catalog.json")
	}
	templates, e := probetemplate.Open(templateFile)
	if e != nil {
		r.Close()
		return nil, e
	}
	if config.Gateway.Logger != nil {
		for _, warning := range templates.StartupWarnings() {
			config.Gateway.Logger.Printf("warning=%s", warning)
		}
	}
	catalog, e := enrollment.Open(filepath.Join(r.Directory(), "devices", "catalog.json"))
	if e != nil {
		templates.Close()
		r.Close()
		return nil, e
	}
	if config.Gateway.Logger != nil {
		for _, warning := range catalog.StartupWarnings() {
			config.Gateway.Logger.Printf("warning=%s", warning)
		}
	}
	config.Gateway.Enrollment = catalog
	g, e := gateway.New(config.Gateway)
	if e != nil {
		catalog.Close()
		templates.Close()
		r.Close()
		return nil, e
	}
	if config.Gateway.Logger != nil {
		config.Gateway.Logger.Printf("repository_dir=%s", r.Directory())
		leftovers, err := r.Leftovers()
		if err != nil {
			catalog.Close()
			templates.Close()
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
			catalog.Close()
			templates.Close()
			g.Close()
			r.Close()
			return nil, e
		}
		g.SetTunnelStatus(maintenance.Report)
	}
	server := &Server{logs: devicelog.New(g), enrollment: catalog, Service: NewService(r, g.Devices(), g), gateway: g, repo: r, maintenance: maintenance, templates: templates}
	if e = config.EasyTier.Validate(); e != nil {
		server.Close()
		return nil, e
	}
	server.networks, e = overlay.Open(filepath.Join(r.Directory(), "networks", "catalog.json"), config.EasyTier, &overlayDriver{s: server, config: config.EasyTier}, nil)
	if e != nil {
		server.Close()
		return nil, e
	}
	if config.Forwarding != nil {
		fc := *config.Forwarding
		if fc.StateFile == "" {
			fc.StateFile = filepath.Join(r.Directory(), "forwarding-ports.json")
		}
		server.forwarding, e = forwarding.New(fc, g)
		if e != nil {
			server.Close()
			return nil, e
		}
		g.SetForwardingStatus(server.forwarding.Report)
	}
	return server, nil
}
func (s *Server) Forwarding() *forwarding.Service        { return s.forwarding }
func (s *Server) ProbeTemplates() *probetemplate.Service { return s.templates }
func (s *Server) Maintenance() *tunnel.Service           { return s.maintenance }
func (s *Server) Serve(l net.Listener) error             { return s.gateway.Serve(l) }
func (s *Server) Close() error {
	s.once.Do(func() {
		if s.networks != nil {
			s.networks.Close()

		}
		if s.forwarding != nil {
			s.forwarding.Close()
		}
		if s.maintenance != nil {
			s.maintenance.Close()
		}
		s.closeErr = errors.Join(s.gateway.Close(), s.enrollment.Close(), s.templates.Close(), s.repo.Close())
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
