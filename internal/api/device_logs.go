package api

import (
	"errors"
	"net/http"
	"routerprobe/internal/devicelog"
	"strconv"
)

func logError(e error) response {
	if errors.Is(e, devicelog.ErrInvalid) {
		return invalid()
	}
	if errors.Is(e, devicelog.ErrBusy) {
		return response{status: 429, code: "log_busy"}
	}
	if errors.Is(e, devicelog.ErrRead) {
		return response{status: 422, code: "log_read_failed", message: e.Error()}
	}
	return failure(e)
}
func (a *Server) deviceLogRoutes() {
	a.route("POST /api/v1/log-assets/{id}/text", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		asset, e := a.app.DecodeLog(r.Context(), r.PathValue("id"))
		if e != nil {
			return response{status: 422, code: "log_decode_failed"}
		}
		return response{status: 201, data: asset}
	})
	a.route("GET /api/v1/devices/{id}/logs/{operation}", false, func(r *http.Request) response {
		q := r.URL.Query()
		offset := uint64(0)
		if q.Get("offset") != "" {
			var e error
			offset, e = strconv.ParseUint(q.Get("offset"), 10, 64)
			if e != nil {
				return invalid()
			}
		}
		v, e := a.app.DeviceLogs().Read(r.Context(), r.PathValue("id"), q.Get("session_id"), devicelog.Query{Operation: r.PathValue("operation"), Directory: q.Get("directory"), Generation: q.Get("generation"), Offset: offset})
		if e != nil {
			return logError(e)
		}
		return response{status: 200, data: v}
	})
	a.route("POST /api/v1/devices/{id}/log-tasks", true, func(r *http.Request) response {
		var q struct {
			SessionID string           `json:"session_id"`
			Params    devicelog.Params `json:"params"`
		}
		if decode(r, &q) != nil || q.Params.Validate() != nil || q.SessionID == "" {
			return invalid()
		}
		id, e := a.app.DeviceLogs().Create(r.Context(), r.PathValue("id"), q.SessionID, q.Params)
		return accepted(id, object{"task_id": id}, e)
	})
	a.route("GET /api/v1/log-assets/{id}/preview", false, func(r *http.Request) response {
		v, e := a.app.PreviewLog(r.Context(), r.PathValue("id"))
		if e != nil {
			return response{status: 422, code: "log_preview_failed", message: e.Error()}
		}
		return response{status: 200, data: v}
	})
}
