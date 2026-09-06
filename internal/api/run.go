package api

import (
	"context"
	"errors"
	"net"
	"routerprobe/internal/management"
	"sync"
)

// Run owns both adapters. API admission stops before business services close.
func Run(ctx context.Context, controlAddress, httpAddress string, config management.Config, apiConfig Config) error {
	app, e := management.New(config)
	if e != nil {
		return e
	}
	defer app.Close()
	control, e := net.Listen("tcp", controlAddress)
	if e != nil {
		return e
	}
	defer control.Close()
	httpListener, e := net.Listen("tcp", httpAddress)
	if e != nil {
		return e
	}
	defer httpListener.Close()
	a, e := New(app, apiConfig)
	if e != nil {
		return e
	}
	defer a.Close()
	results := make(chan error, 2)
	go func() { results <- app.Serve(control) }()
	go func() { results <- a.Serve(httpListener) }()
	var first error
	received := 0
	select {
	case <-ctx.Done():
	case first = <-results:
		received = 1
	}
	a.Close()
	app.Close()
	for received < 2 {
		e := <-results
		if first == nil && !errors.Is(e, net.ErrClosed) {
			first = e
		}
		received++
	}
	return first
}

type boundedListener struct {
	net.Listener
	slots chan struct{}
}
type boundedConn struct {
	net.Conn
	once  sync.Once
	slots chan struct{}
}

func (l *boundedListener) Accept() (net.Conn, error) {
	for {
		c, e := l.Listener.Accept()
		if e != nil {
			return nil, e
		}
		select {
		case l.slots <- struct{}{}:
			return &boundedConn{Conn: c, slots: l.slots}, nil
		default:
			c.Close()
		}
	}
}
func (c *boundedConn) Close() error { e := c.Conn.Close(); c.once.Do(func() { <-c.slots }); return e }
