package api

import (
	"net/http"
	"routerprobe/internal/device"
	"routerprobe/internal/task"
	"time"
)

func (a *Server) neighborRoutes() {
	a.route("GET /api/v1/capabilities", false, func(r *http.Request) response {
		return ok(object{"capabilities": []string{"neighbor_probe", "neighbors_inspect_v1", "neighbors_recent_v1"}}, nil)
	})
	a.route("GET /api/v1/devices/{id}/neighbor-discovery", false, func(r *http.Request) response {
		d, e := a.app.Devices().Get(r.PathValue("id"))
		return ok(object{"discovery": device.NeighborDiscoverySnapshot(d, time.Now())}, e)
	})
	a.route("POST /api/v1/devices/{id}/neighbor-inspections", true, func(r *http.Request) response {
		var p task.NeighborInspectRequest
		if decode(r, &p) != nil {
			return invalid()
		}
		id, e := a.app.InspectNeighbors(r.Context(), r.PathValue("id"), p)
		return accepted(id, object{"task_id": id}, e)
	})

	a.route("GET /api/v1/devices/{id}/neighbors", false, func(r *http.Request) response {
		n, e := a.app.Neighbors(r.PathValue("id"))
		d, err := a.app.Devices().Get(r.PathValue("id"))
		if err != nil {
			return failure(err)
		}
		return ok(object{"snapshot": n, "recent": device.RecentNeighborSnapshot(d, time.Now()), "retention_seconds": 86400, "capacity": 1024, "persistent": false}, e)
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
