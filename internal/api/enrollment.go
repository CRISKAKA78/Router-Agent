package api

import (
	"net/http"
	"routerprobe/internal/device"
	"routerprobe/internal/enrollment"
	"routerprobe/internal/management"
	"routerprobe/internal/probetemplate"
	"strings"
)

func (a *Server) managedDTO(d device.Snapshot) object {
	out := deviceDTO(d)
	p, e := a.app.Enrollment().Get(d.Registration.DeviceID)
	if e != nil {
		return out
	}
	out["first_seen_at"] = timestamp(p.FirstSeen)
	state := "waiting_dispatch"
	if p.Admission != "managed" {
		state = "not_managed"
	} else if d.CurrentSession != nil {
		if d.CurrentSession.ConfigError != "" {
			state = "failed"
		} else if d.CurrentSession.ConfigRevision == p.Configuration.Revision {
			state = "applied"
		} else {
			state = "waiting_confirmation"
		}
	}
	out["profile"] = object{"version": p.Version, "admission": p.Admission, "name": p.Name, "model_id": p.ModelID, "model_name": p.ModelName, "monitoring": p.Monitoring, "property_intervals": p.PropertyIntervals, "bound_template": templateSummary(p.BoundTemplate), "desired_revision": p.Configuration.Revision, "configuration_state": state, "configuration_error": d.LatestSession.ConfigError}
	out["applied_revision"] = d.LatestSession.ConfigRevision
	profile := out["profile"].(object)
	profile["interface_sampling"] = p.InterfaceSampling
	profile["template_generation"] = p.Configuration.TemplateGeneration
	if p.BoundTemplate != nil {
		if latest, err := a.app.ProbeTemplates().Resolve(p.BoundTemplate.ID, ""); err == nil {
			profile["latest_template"] = templateSummary(&latest)
		}
	}
	if t := d.LatestSession.ConfigTemplate; t != nil {
		out["presentation"] = t.Presentation
		if t.CellularProbe != nil {
			out["cellular_configuration"] = t.CellularProbe
		}
		if t.NeighborProbe != nil {
			out["neighbor_domains"] = t.NeighborProbe.Domains
			out["neighbor_configuration"] = t.NeighborProbe
		}
		out["active_template"] = object{"template_id": t.ID, "name": t.Name, "version": t.Version}
	}
	return out
}
func (a *Server) enrollmentRoutes() {
	a.route("GET /api/v1/discoveries", false, func(r *http.Request) response {
		state := r.URL.Query().Get("admission")
		if state == "" {
			state = "pending"
		}
		if state != "pending" && state != "ignored" {
			return invalid()
		}
		out := []object{}
		for _, d := range a.app.Inventory() {
			p, _ := a.app.Enrollment().Get(d.Registration.DeviceID)
			if p.Admission == state {
				out = append(out, a.managedDTO(d))
			}
		}
		return paged(r, out)
	})
	a.route("PUT /api/v1/devices/{id}/profile", true, func(r *http.Request) response {
		var q management.DeviceUpdate
		if decode(r, &q) != nil {
			return invalid()
		}
		v, e := a.app.UpdateDevice(r.PathValue("id"), q)
		return ok(v, e)
	})
	a.route("GET /api/v1/device-models", false, func(r *http.Request) response {
		items := []enrollment.Model{}
		q := strings.ToLower(r.URL.Query().Get("q"))
		for _, m := range a.app.Enrollment().Models() {
			if q == "" || strings.Contains(strings.ToLower(m.Name+" "+strings.Join(m.Aliases, " ")), q) {
				items = append(items, m)
			}
		}
		return paged(r, items)
	})
	a.route("GET /api/v1/device-models/match", false, func(r *http.Request) response { return ok(a.app.Enrollment().Match(r.URL.Query().Get("name")), nil) })
	a.route("PUT /api/v1/device-models/{id}", true, func(r *http.Request) response {
		var m enrollment.Model
		if decode(r, &m) != nil {
			return invalid()
		}
		m.ID = r.PathValue("id")
		v, e := a.app.PutModel(m)
		return ok(v, e)
	})
}

// This template summary contains collection metadata only. Applied neighbor settings
// are exposed separately as neighbor_configuration for explicit editor import.
func templateSummary(t *probetemplate.Template) any {
	if t == nil {
		return nil
	}
	props := object{}
	for k, p := range t.Properties {
		props[k] = object{"name": p.Name, "interval_seconds": p.Interval}
	}
	return object{"template_id": t.ID, "name": t.Name, "version": t.Version, "monitoring": t.Monitoring, "properties": props, "cellular_probe": t.CellularProbe}
}
