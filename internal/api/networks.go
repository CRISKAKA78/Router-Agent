package api

import (
	"net/http"
	"routerprobe/internal/overlay"
)

func (a *Server) networkRoutes() {
	a.route("GET /api/v1/network-settings", false, func(r *http.Request) response { return ok(a.app.Networks().Status(), nil) })
	a.route("GET /api/v1/networks", false, func(r *http.Request) response { return paged(r, a.app.Networks().List()) })
	a.route("POST /api/v1/networks", true, func(r *http.Request) response {
		var q struct {
			overlay.Spec
			Password string `json:"password"`
		}
		if decode(r, &q) != nil {
			return invalid()
		}
		if q.Password == "" {
			q.Password = overlay.UUID()
		}
		v, e := a.app.Networks().CreateConfigured(q.Spec, q.Password)
		if e != nil {
			return failure(e)
		}
		return response{status: 201, data: v, location: "/api/v1/networks/" + v.ID}
	})
	a.route("GET /api/v1/networks/{network}", false, func(r *http.Request) response { v, e := a.app.Networks().Get(r.PathValue("network")); return ok(v, e) })
	a.route("PUT /api/v1/networks/{network}", true, func(r *http.Request) response {
		var q struct {
			overlay.Spec
			Revision uint64 `json:"revision"`
		}
		if decode(r, &q) != nil {
			return invalid()
		}
		v, e := a.app.Networks().Update(r.PathValue("network"), q.Spec, q.Revision)
		return ok(v, e)
	})
	a.route("DELETE /api/v1/networks/{network}", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		return ok(object{}, a.app.Networks().Delete(r.PathValue("network")))
	})
	a.route("GET /api/v1/networks/{network}/password", false, func(r *http.Request) response {
		v, e := a.app.Networks().Password(r.PathValue("network"))
		return ok(map[string]string{"password": v}, e)
	})
	a.route("PUT /api/v1/networks/{network}/members/{device}/config", true, func(r *http.Request) response {
		var q struct {
			Config   overlay.MemberConfig `json:"config"`
			Revision uint64               `json:"revision"`
		}
		if decode(r, &q) != nil {
			return invalid()
		}
		v, e := a.app.Networks().ConfigureMember(r.PathValue("network"), r.PathValue("device"), q.Config, q.Revision)
		return ok(v, e)
	})
	a.route("POST /api/v1/networks/{network}/members/batch", true, func(r *http.Request) response {
		var q struct {
			Members []overlay.JoinRequest `json:"members"`
		}
		if decode(r, &q) != nil {
			return invalid()
		}
		v, e := a.app.Networks().JoinBatch(r.PathValue("network"), q.Members)
		if e != nil {
			return failure(e)
		}
		return response{status: 202, data: v}
	})

	a.route("GET /api/v1/networks/{network}/topology", false, func(r *http.Request) response {
		v, e := a.app.Networks().Topology(r.PathValue("network"))
		return ok(v, e)
	})
	a.route("GET /api/v1/networks/{network}/operations", false, func(r *http.Request) response {
		if _, e := a.app.Networks().Get(r.PathValue("network")); e != nil {
			return failure(e)
		}
		return paged(r, a.app.Networks().Operations(r.PathValue("network")))
	})
	a.route("POST /api/v1/networks/{network}/members", true, func(r *http.Request) response {
		var q overlay.JoinRequest
		if decode(r, &q) != nil {
			return invalid()
		}
		v, e := a.app.Networks().Start(r.PathValue("network"), q, "start")
		if e != nil {
			return failure(e)
		}
		return response{status: 202, data: v}
	})
	a.route("POST /api/v1/networks/{network}/members/{device}/operations", true, func(r *http.Request) response {
		var q struct {
			Action string `json:"action"`
		}
		if decode(r, &q) != nil {
			return invalid()
		}
		v, e := a.app.Networks().Start(r.PathValue("network"), overlay.JoinRequest{DeviceID: r.PathValue("device")}, q.Action)
		if e != nil {
			return failure(e)
		}
		return response{status: 202, data: v}
	})
	a.route("DELETE /api/v1/networks/{network}/members/{device}", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		return ok(object{}, a.app.Networks().RemoveMember(r.PathValue("network"), r.PathValue("device")))
	})
	a.route("POST /api/v1/network-operations/{operation}/reconcile", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		v, e := a.app.Networks().Reconcile(r.Context(), r.PathValue("operation"))
		return ok(v, e)
	})
}
