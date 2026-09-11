package api

import (
	"errors"
	"net/http"
	"routerprobe/internal/forwarding"
)

func forwardingError(e error) response {
	if e == nil {
		return response{data: object{"released": true}}
	}
	switch {
	case errors.Is(e, forwarding.ErrInvalid):
		return invalid()
	case errors.Is(e, forwarding.ErrNotFound):
		return response{status: 404, code: "not_found"}
	case errors.Is(e, forwarding.ErrUnavailable):
		return response{status: 503, code: "forwarding_unavailable"}
	case errors.Is(e, forwarding.ErrCapacity):
		return response{status: 409, code: "capacity_exhausted"}
	default:
		return response{status: 409, code: "forwarding_failed"}
	}
}
func (a *Server) forwardingRoutes() {
	a.route("GET /api/v1/devices/{id}/forwarding-capabilities", false, func(r *http.Request) response {
		f := a.app.Forwarding()
		if f == nil {
			return forwardingError(forwarding.ErrUnavailable)
		}
		v, e := f.Inventory(r.Context(), r.PathValue("id"))
		if e != nil {
			return forwardingError(e)
		}
		return response{data: v}
	})
	a.route("GET /api/v1/forwardings", false, func(r *http.Request) response {
		f := a.app.Forwarding()
		if f == nil {
			return response{data: object{"items": []any{}, "total": 0, "offset": 0, "limit": 200}}
		}
		out := []any{}
		for _, v := range f.List() {
			if id := r.URL.Query().Get("device_id"); id == "" || id == v.DeviceID {
				out = append(out, v)
			}
		}
		return paged(r, out)
	})
	a.route("POST /api/v1/forwardings", true, func(r *http.Request) response {
		f := a.app.Forwarding()
		if f == nil {
			return forwardingError(forwarding.ErrUnavailable)
		}
		var q forwarding.Request
		if decode(r, &q) != nil {
			return invalid()
		}
		v, e := f.Create(r.Context(), q)
		if e != nil {
			return forwardingError(e)
		}
		return response{status: 201, data: v, location: "/api/v1/forwardings/" + v.Mapping.ID}
	})
	a.route("GET /api/v1/forwardings/{id}", false, func(r *http.Request) response {
		f := a.app.Forwarding()
		if f == nil {
			return forwardingError(forwarding.ErrUnavailable)
		}
		v, e := f.Get(r.PathValue("id"))
		if e != nil {
			return forwardingError(e)
		}
		return response{data: v}
	})
	a.route("POST /api/v1/forwardings/{id}/close", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		f := a.app.Forwarding()
		if f == nil {
			return forwardingError(forwarding.ErrUnavailable)
		}
		return forwardingError(f.CloseMapping(r.PathValue("id")))
	})
}
