package api

import (
	"net/http"
	"routerprobe/internal/device"
	"time"
)

func connectionDTO(p device.ConnectionPeriod, now time.Time) object {
	seconds := func(start, end time.Time) int64 {
		if end.Before(start) {
			return 0
		}
		return int64(end.Sub(start) / time.Second)
	}
	state := "online"
	end := now
	var offline any
	if !p.OfflineAt.IsZero() {
		end, state = p.OfflineAt, "offline"
		until := now
		if !p.ReconnectedAt.IsZero() {
			until, state = p.ReconnectedAt, "completed"
		}
		offline = seconds(p.OfflineAt, until)
	}
	return object{"id": p.ID, "state": state, "online_at": timestamp(p.OnlineAt), "offline_at": timestamp(p.OfflineAt), "reconnected_at": timestamp(p.ReconnectedAt), "online_seconds": seconds(p.OnlineAt, end), "offline_seconds": offline, "end_reason": p.EndReason, "observed_at": timestamp(now)}
}

func (a *Server) connectionHistory(r *http.Request) response {
	id := r.PathValue("id")
	h, err := a.app.Devices().Connections(id)
	if err != nil {
		if _, e := a.app.Enrollment().Get(id); e != nil {
			return failure(err)
		}
		return paged(r, []object{})
	}
	now := time.Now()
	items := make([]object, 0, len(h.Periods))
	for _, p := range h.Periods {
		items = append(items, connectionDTO(p, now))
	}
	out := paged(r, items)
	if out.code == "" {
		data := out.data.(object)
		data["history_limit"], data["total_periods"], data["evicted_periods"] = h.Limit, h.Total, h.Evicted
	}
	return out
}
