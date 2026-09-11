package api

import (
	"net/http"
	"routerprobe/internal/device"
	"time"
)

func (a *Server) cellularRoutes() {
	a.route("GET /api/v1/devices/{id}/cellular", false, func(r *http.Request) response {
		v, e := a.app.Devices().Get(r.PathValue("id"))
		return ok(object{"snapshot": device.CellularSnapshot(v, time.Now())}, e)
	})
}
