package api

import (
	"net/http"
	"routerprobe/internal/probetemplate"
)

func (a *Server) templateRoutes() {
	a.route("GET /api/v1/probe-templates", false, func(r *http.Request) response {
		items, e := a.app.ProbeTemplates().List()
		if e != nil {
			return failure(e)
		}
		return paged(r, items)
	})
	a.route("GET /api/v1/probe-templates/{id}", false, func(r *http.Request) response {
		v, e := a.app.ProbeTemplates().Resolve(r.PathValue("id"), "")
		return ok(v, e)
	})
	a.route("POST /api/v1/probe-templates", true, func(r *http.Request) response {
		var q probetemplate.Input
		if decode(r, &q) != nil {
			return invalid()
		}
		v, e := a.app.ProbeTemplates().Put("", 0, q)
		if e != nil {
			return failure(e)
		}
		return response{status: 201, data: v, location: "/api/v1/probe-templates/" + v.ID}
	})
	a.route("PUT /api/v1/probe-templates/{id}", true, func(r *http.Request) response {
		var q struct {
			probetemplate.Input
			Version uint64 `json:"version"`
		}
		if decode(r, &q) != nil || q.Version == 0 {
			return invalid()
		}
		v, e := a.app.ProbeTemplates().Put(r.PathValue("id"), q.Version, q.Input)
		return ok(v, e)
	})
	a.route("DELETE /api/v1/probe-templates/{id}", true, func(r *http.Request) response {
		var q struct {
			Version uint64 `json:"version"`
		}
		if decode(r, &q) != nil || q.Version == 0 {
			return invalid()
		}
		e := a.app.ProbeTemplates().Delete(r.PathValue("id"), q.Version)
		return ok(object{"deleted": true}, e)
	})
}
