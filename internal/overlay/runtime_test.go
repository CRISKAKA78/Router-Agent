package overlay

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type startingController struct {
	fakeController
	reads  int
	cancel context.CancelFunc
}

func (c *startingController) Collect(context.Context, string, string) (Running, error) {
	c.reads++
	if c.cancel != nil {
		c.cancel()
		return Running{}, nil
	}
	return Running{Running: c.reads >= 2}, nil
}
func TestRuntimeWaitsForAsynchronousStartWithoutReplay(t *testing.T) {
	c := &startingController{}
	s := &Service{controller: c}
	r, err := s.waitRuntime(context.Background(), UUID(), UUID(), true)
	if err != nil || !r.Running || c.reads != 2 || c.applies != 0 {
		t.Fatalf("r=%+v err=%v reads=%d writes=%d", r, err, c.reads, c.applies)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c = &startingController{cancel: cancel}
	s.controller = c
	if _, err = s.waitRuntime(ctx, UUID(), UUID(), true); !errors.Is(err, ErrUncertain) {
		t.Fatal(err)
	}
	if c.reads != 1 || c.applies != 0 {
		t.Fatal("cancellation replayed mutation")
	}
}

// EasyTier 2.6.4 loses the enabled-empty distinction in its runtime-to-JSON
// conversion. Do not silently treat an unprovable configuration as confirmed.
func TestConfirmDoesNotAcceptLossyManualRoutes(t *testing.T) {
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/login" {
			w.Write([]byte(`{}`))
			return
		}
		w.Write([]byte(`{"enable_manual_routes":null,"routes":[]}`))
	}))
	defer h.Close()
	cfg := testConfig()
	cfg.APIURL = h.URL
	client, err := NewWebClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err = client.Confirm(context.Background(), UUID(), UUID(), map[string]any{"enable_manual_routes": true, "routes": []string{}}); !errors.Is(err, ErrUncertain) {
		t.Fatal(err)
	}
}
