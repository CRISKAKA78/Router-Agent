package overlay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

// Controller is the official EasyTier Web configuration API, not core's RPC.
type Controller interface {
	Apply(context.Context, string, string, map[string]any) error
	Stop(context.Context, string, string) error
	Collect(context.Context, string, string) (Running, error)
}
type WebClient struct {
	config Config
	client *http.Client
	mu     sync.Mutex
	logged bool
}

func NewWebClient(c Config) (*WebClient, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	j, _ := cookiejar.New(nil)
	return &WebClient{config: c, client: &http.Client{Jar: j, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (w *WebClient) Close() { w.client.CloseIdleConnections() }
func (w *WebClient) request(ctx context.Context, method, p string, b []byte, out any) (int, error) {
	r, e := http.NewRequestWithContext(ctx, method, w.config.APIURL+p, bytes.NewReader(b))
	if e != nil {
		return 0, ErrUpstream
	}
	r.Header.Set("Content-Type", "application/json")
	resp, e := w.client.Do(r)
	if e != nil {
		if method == http.MethodGet {
			return 0, ErrUpstream
		}
		return 0, ErrUncertain
	}
	defer resp.Body.Close()
	data, e := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if e != nil || len(data) > 4*1024*1024 {
		if method == http.MethodGet {
			return resp.StatusCode, ErrUpstream
		}
		return resp.StatusCode, ErrUncertain
	}
	// Never forward upstream bodies: errors and node config can contain secrets.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode >= 500 && method != http.MethodGet {
			return resp.StatusCode, ErrUncertain
		}
		return resp.StatusCode, ErrUpstream
	}
	if out != nil {
		if e = json.Unmarshal(data, out); e != nil {
			return resp.StatusCode, ErrUpstream
		}
	}
	return resp.StatusCode, nil
}
func (w *WebClient) login(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.logged {
		return nil
	}
	b, _ := json.Marshal(map[string]string{"username": w.config.Username, "password": w.config.Password})
	_, e := w.request(ctx, "POST", "/api/v1/auth/login", b, nil)
	if e != nil {
		return ErrUpstream
	}
	w.logged = true
	return nil
}
func (w *WebClient) call(ctx context.Context, method, p string, q, out any) error {
	if w.config.APIURL == "" {
		return ErrDisabled
	}
	if e := w.login(ctx); e != nil {
		return e
	}
	var b []byte
	if q != nil {
		b, _ = json.Marshal(q)
	}
	status, e := w.request(ctx, method, p, b, out)
	if status == 401 {
		w.mu.Lock()
		w.logged = false
		w.mu.Unlock()
		if x := w.login(ctx); x != nil {
			return x
		}
		_, e = w.request(ctx, method, p, b, out)
	}
	return e
}
func machinePath(machine string) string { return "/api/v1/machines/" + machine + "/networks" }
func (w *WebClient) Apply(ctx context.Context, machine, instance string, c map[string]any) error {
	if !ValidUUID(machine) || !ValidUUID(instance) {
		return ErrInvalid
	}
	// PUT saves a deterministic instance ID and marks it disabled; enable that
	// exact instance next. A failed/uncertain second step is not a success.
	if e := w.call(ctx, "PUT", machinePath(machine)+"/config/"+instance, map[string]any{"config": c}, nil); e != nil {
		return e
	}
	if e := w.call(ctx, "PUT", machinePath(machine)+"/"+instance, map[string]bool{"disabled": false}, nil); e != nil {
		return ErrUncertain
	}
	return nil
}
func (w *WebClient) Stop(ctx context.Context, machine, instance string) error {
	if !ValidUUID(machine) || !ValidUUID(instance) {
		return ErrInvalid
	}
	return w.call(ctx, "PUT", machinePath(machine)+"/"+instance, map[string]bool{"disabled": true}, nil)
}
func (w *WebClient) Collect(ctx context.Context, machine, instance string) (Running, error) {
	var response struct {
		Info struct {
			Map map[string]Running `json:"map"`
		} `json:"info"`
	}
	if !ValidUUID(machine) || !ValidUUID(instance) {
		return Running{}, ErrInvalid
	}
	e := w.call(ctx, "GET", machinePath(machine)+"/info/"+instance, nil, &response)
	if e != nil {
		return Running{}, e
	}
	v, ok := response.Info.Map[instance]
	if !ok {
		return Running{}, ErrNotFound
	}
	return v, nil
}
func uncertain(e error) bool {
	return errors.Is(e, ErrUncertain) || errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded)
}

func (w *WebClient) Ready(ctx context.Context, machine string) error {
	if !ValidUUID(machine) {
		return ErrInvalid
	}
	return w.call(ctx, "GET", machinePath(machine), nil, nil)
}

// Confirm reads configuration but never exposes its secret in public results.
func (w *WebClient) Confirm(ctx context.Context, machine, instance string, expected map[string]any) error {
	if !ValidUUID(machine) || !ValidUUID(instance) {
		return ErrInvalid
	}
	var actual map[string]json.RawMessage
	if e := w.call(ctx, "GET", machinePath(machine)+"/config/"+instance, nil, &actual); e != nil {
		return e
	}
	for key, value := range expected {
		a, e := json.Marshal(value)
		if e != nil {
			return ErrInvalid
		}
		var canonical any
		if json.Unmarshal(actual[key], &canonical) != nil {
			return ErrUncertain
		}
		b, _ := json.Marshal(canonical)
		if !bytes.Equal(a, b) {
			return ErrUncertain
		}
	}
	return nil
}
