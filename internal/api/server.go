package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"routerprobe/internal/device"
	"routerprobe/internal/enrollment"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/probetemplate"
	"routerprobe/internal/repository"
	"routerprobe/internal/routerconfig"
	"routerprobe/internal/task"
	"routerprobe/internal/tunnel"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	MaxRequests, MaxClients, IdempotencyCapacity int
	MaxAssetBytes                                int64
	RequestTimeout, WriteTimeout, PollInterval   time.Duration
}
type response struct {
	message  string
	status   int
	data     any
	location string
	code     string
}
type entry struct {
	signature [32]byte
	done      chan struct{}
	response  response
}
type Server struct {
	app      *management.Server
	config   Config
	mux      *http.ServeMux
	http     *http.Server
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	closed   bool
	keys     map[string]*entry
	clients  map[*client]struct{}
	requests chan struct{}
	wg       sync.WaitGroup
	once     sync.Once
}

func New(app *management.Server, c Config) (*Server, error) {
	if app == nil {
		return nil, errors.New("management server required")
	}
	if c.MaxRequests == 0 {
		c.MaxRequests = 32
	}
	if c.MaxClients == 0 {
		c.MaxClients = 64
	}
	if c.IdempotencyCapacity == 0 {
		c.IdempotencyCapacity = 4096
	}
	if c.MaxAssetBytes == 0 {
		c.MaxAssetBytes = 1 << 30
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = 30 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 5 * time.Second
	}
	if c.PollInterval == 0 {
		c.PollInterval = 250 * time.Millisecond
	}
	if c.MaxRequests < 1 || c.MaxClients < 1 || c.IdempotencyCapacity < 1 || c.MaxAssetBytes < 1 || c.RequestTimeout < time.Millisecond || c.WriteTimeout < time.Millisecond || c.PollInterval < time.Millisecond {
		return nil, errors.New("invalid API limits")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &Server{app: app, config: c, ctx: ctx, cancel: cancel, mux: http.NewServeMux(), keys: map[string]*entry{}, clients: map[*client]struct{}{}, requests: make(chan struct{}, c.MaxRequests)}
	a.routes()
	a.http = &http.Server{Handler: a, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: c.RequestTimeout, WriteTimeout: c.RequestTimeout + c.WriteTimeout, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	a.wg.Add(1)
	go a.poll()
	return a, nil
}
func (a *Server) Serve(l net.Listener) error {
	err := a.http.Serve(&boundedListener{Listener: l, slots: make(chan struct{}, a.config.MaxRequests+a.config.MaxClients+32)})
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Close first stops admission and live sockets, then joins admitted requests.
// The owner closes Management only after this returns.
func (a *Server) Close() error {
	a.once.Do(func() {
		a.mu.Lock()
		a.closed = true
		a.cancel()
		for c := range a.clients {
			if c.conn != nil {
				c.conn.Close()
			}
		}
		a.mu.Unlock()
		a.http.Close()
		a.wg.Wait()
	})
	return nil
}
func (a *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		if e != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
			write(w, response{status: 403, code: "origin_denied"})
			return
		}
	}
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		write(w, response{status: 503, code: "server_closed"})
		return
	}
	a.wg.Add(1)
	a.mu.Unlock()
	defer a.wg.Done()
	if r.URL.Path == "/api/v1/events" && r.Method == "GET" {
		a.events(w, r)
		return
	}
	select {
	case a.requests <- struct{}{}:
		defer func() { <-a.requests }()
	default:
		write(w, response{status: 503, code: "capacity_exhausted"})
		return
	}
	_, pattern := a.mux.Handler(r)
	if pattern == "" {
		status := 404
		code := "not_found"
		for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
			probe := r.Clone(r.Context())
			probe.Method = method
			if _, p := a.mux.Handler(probe); p != "" {
				status = 405
				code = "method_not_allowed"
				w.Header().Add("Allow", method)
			}
		}
		write(w, response{status: status, code: code})
		return
	}
	// ServeMux populates path variables in ServeHTTP, not Handler.
	a.mux.ServeHTTP(w, r)
}
func write(w http.ResponseWriter, v response) {
	if v.status == 0 {
		v.status = 200
	}
	if v.location != "" {
		w.Header().Set("Location", v.location)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(v.status)
	if v.status == 204 {
		return
	}
	if v.code != "" {
		message := v.message
		if message == "" {
			message = strings.ReplaceAll(v.code, "_", " ")
		}
		json.NewEncoder(w).Encode(object{"error": object{"code": v.code, "message": message}})
		return
	}
	json.NewEncoder(w).Encode(object{"data": v.data})
}
func failure(err error) response {
	s, c := 500, "internal_error"
	switch {
	case errors.Is(err, enrollment.ErrNotManaged):
		s, c = 409, "device_not_managed"
	case errors.Is(err, repository.ErrInvalid), errors.Is(err, probetemplate.ErrInvalid), errors.Is(err, routerconfig.ErrInvalid):
		s, c = 400, "invalid_request"
	case errors.Is(err, routerconfig.ErrUnsupported):
		s, c = 422, "unsupported_capability"
	case errors.Is(err, device.ErrNotFound), errors.Is(err, task.ErrTaskNotFound), errors.Is(err, repository.ErrNotFound), errors.Is(err, tunnel.ErrNotFound), errors.Is(err, probetemplate.ErrNotFound):
		s, c = 404, "not_found"
	case errors.Is(err, gateway.ErrOffline), errors.Is(err, management.ErrOffline):
		s, c = 409, "device_offline"
	case errors.Is(err, gateway.ErrSessionChanged), errors.Is(err, tunnel.ErrSession):
		s, c = 409, "session_changed"
	case errors.Is(err, tunnel.ErrConflict), errors.Is(err, repository.ErrConflict), errors.Is(err, repository.ErrReferenced), errors.Is(err, repository.ErrArchived), errors.Is(err, management.ErrNotCommitted), errors.Is(err, task.ErrTaskRejected), errors.Is(err, probetemplate.ErrConflict):
		s, c = 409, "conflict"
	case errors.Is(err, management.ErrIncompatible), errors.Is(err, management.ErrAmbiguous):
		s, c = 422, "incompatible"
	case errors.Is(err, tunnel.ErrCapacity), errors.Is(err, probetemplate.ErrCapacity):
		s, c = 503, "capacity_exhausted"
	case errors.Is(err, repository.ErrClosed), errors.Is(err, net.ErrClosed), errors.Is(err, probetemplate.ErrClosed):
		s, c = 503, "server_closed"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		s, c = 504, "operation_timeout"
	}
	return response{status: s, code: c}
}
func invalid() response { return response{status: 400, code: "invalid_request"} }
func ok(v any, e error) response {
	if e != nil {
		return failure(e)
	}
	return response{data: v}
}
func decode(r *http.Request, v any) error {
	b, e := io.ReadAll(r.Body)
	if e != nil {
		return e
	}
	if e = validObject(b); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

type action func(*http.Request) response

func (a *Server) route(pattern string, mutation bool, f action) {
	a.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if !mutation {
			write(w, f(r))
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if len(key) < 1 || len(key) > 128 {
			write(w, invalid())
			return
		}
		for _, c := range key {
			if c < 33 || c > 126 {
				write(w, invalid())
				return
			}
		}
		var signature [32]byte
		if r.URL.Path == "/api/v1/assets" && r.Method == "POST" {
			mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mt != "application/octet-stream" {
				write(w, response{status: 415, code: "unsupported_media_type"})
				return
			}
			if r.ContentLength < 0 {
				write(w, response{status: 411, code: "length_required"})
				return
			}
			if r.ContentLength > a.config.MaxAssetBytes {
				write(w, response{status: 413, code: "payload_too_large"})
				return
			}
			digest := r.Header.Get("X-Content-SHA256")
			b, e := hex.DecodeString(digest)
			if e != nil || len(b) != 32 || strings.ToLower(digest) != digest {
				write(w, invalid())
				return
			}
			signature = sha256.Sum256([]byte(r.Method + " " + r.URL.RequestURI() + " " + strconv.FormatInt(r.ContentLength, 10) + " " + digest))
			r.Body = http.MaxBytesReader(w, r.Body, a.config.MaxAssetBytes)
		} else {
			mt, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if e != nil || mt != "application/json" {
				write(w, response{status: 415, code: "unsupported_media_type"})
				return
			}
			b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
			if e != nil {
				write(w, response{status: 413, code: "payload_too_large"})
				return
			}
			if validObject(b) != nil {
				write(w, invalid())
				return
			}
			signature = sha256.Sum256(append([]byte(r.Method+" "+r.URL.RequestURI()+" "), b...))
			r.Body = io.NopCloser(bytes.NewReader(b))
		}
		a.mu.Lock()
		previous := a.keys[key]
		if previous != nil {
			a.mu.Unlock()
			if previous.signature != signature {
				write(w, response{status: 409, code: "idempotency_conflict"})
				return
			}
			select {
			case <-previous.done:
				w.Header().Set("Idempotency-Replayed", "true")
				write(w, previous.response)
			case <-r.Context().Done():
			case <-a.ctx.Done():
				write(w, response{status: 503, code: "server_closed"})
			}
			return
		}
		if len(a.keys) >= a.config.IdempotencyCapacity {
			a.mu.Unlock()
			write(w, response{status: 503, code: "idempotency_capacity"})
			return
		}
		record := &entry{signature: signature, done: make(chan struct{})}
		a.keys[key] = record
		a.mu.Unlock()
		// The creation lifetime belongs to the adapter, not the HTTP connection.
		ctx, cancel := context.WithTimeout(a.ctx, a.config.RequestTimeout)
		defer cancel()
		if r.URL.Path == "/api/v1/assets" && r.Method == "POST" {
			stop := context.AfterFunc(r.Context(), cancel)
			defer stop()
			// Cancel an in-flight body read as well as repository preparation.
			stopBody := context.AfterFunc(ctx, func() { r.Body.Close() })
			defer stopBody()
		}
		result := f(r.WithContext(ctx))
		record.response = result
		close(record.done)
		write(w, result)
	})
}
func page(r *http.Request) (int, int, error) {
	o, l := 0, 50
	var e error
	if v := r.URL.Query().Get("offset"); v != "" {
		o, e = strconv.Atoi(v)
		if e != nil || o < 0 {
			return 0, 0, errors.New("offset")
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		l, e = strconv.Atoi(v)
		if e != nil || l < 1 || l > 200 {
			return 0, 0, errors.New("limit")
		}
	}
	return o, l, nil
}
func paged[T any](r *http.Request, items []T) response {
	o, l, e := page(r)
	if e != nil {
		return invalid()
	}
	total := len(items)
	if o > total {
		o = total
	}
	end := o + l
	if end > total {
		end = total
	}
	return response{data: object{"items": items[o:end], "total": total, "offset": o, "limit": l}}
}
func archived(r *http.Request) (bool, error) {
	v := r.URL.Query().Get("include_archived")
	if v == "" {
		return false, nil
	}
	return strconv.ParseBool(v)
}
