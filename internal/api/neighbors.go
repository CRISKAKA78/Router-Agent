package api

import (
	"net/http"
	"routerprobe/internal/task"
)

func (a *Server) neighborRoutes() {
	a.route("GET /api/v1/devices/{id}/neighbors", false, func(r *http.Request) response {
		n, e := a.app.Neighbors(r.PathValue("id"))
		return ok(object{"snapshot": n}, e)
	})
	a.route("POST /api/v1/devices/{id}/neighbor-scans", true, func(r *http.Request) response {
		var q task.NeighborRequest
		if decode(r, &q) != nil {
			return invalid()
		}
		id, e := a.app.CreateNeighbor(r.Context(), r.PathValue("id"), q, false)
		return accepted(id, object{"task_id": id}, e)
	})
	a.route("POST /api/v1/devices/{id}/neighbor-scans/{task}/cancel", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		id, e := a.app.CreateNeighbor(r.Context(), r.PathValue("id"), task.NeighborRequest{TargetTaskID: r.PathValue("task")}, true)
		return accepted(id, object{"task_id": id}, e)
	})
}
