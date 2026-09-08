package api

import (
	"net/http"
	"routerprobe/internal/routerconfig"
)

func (a *Server) routerConfigRoutes() {
	a.route("POST /api/v1/devices/{id}/config-tasks", true, func(r *http.Request) response {
		var q struct {
			routerconfig.Params
			Timeout *uint32 `json:"timeout_seconds,omitempty"`
		}
		if decode(r, &q) != nil {
			return invalid()
		}
		timeout := uint32(5)
		if q.Timeout != nil {
			timeout = *q.Timeout
		}
		id, err := a.app.CreateRouterConfig(r.Context(), r.PathValue("id"), q.Params, timeout)
		return accepted(id, object{"task_id": id}, err)
	})
}
