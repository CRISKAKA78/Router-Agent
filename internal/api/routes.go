package api

import (
	"errors"
	"io"
	"net/http"
	"path"
	"routerprobe/internal/filetransfer"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/repository"
	"routerprobe/internal/task"

	"strings"
	"time"
)

type execInput struct {
	DeviceID string            `json:"device_id"`
	Command  string            `json:"command"`
	Cwd      string            `json:"cwd"`
	Env      map[string]string `json:"env"`
	Timeout  uint32            `json:"timeout_seconds"`
}
type transferInput struct {
	DeviceID   string `json:"device_id"`
	AssetID    string `json:"asset_id"`
	ToolID     string `json:"tool_id"`
	Version    string `json:"version"`
	ArtifactID string `json:"artifact_id"`
	RemotePath string `json:"remote_path"`
	Name       string `json:"name"`
	Mode       string `json:"mode"`
	Overwrite  bool   `json:"overwrite"`
	Timeout    uint32 `json:"timeout_seconds"`
}

func accepted(id string, data object, err error) response {
	if id == "" {
		return failure(err)
	}
	data["dispatch_uncertain"] = errors.Is(err, gateway.ErrDispatchUncertain)
	if err != nil && !errors.Is(err, gateway.ErrDispatchUncertain) {
		data["warning"] = "operation_recorded"
	}
	return response{status: 202, data: data, location: "/api/v1/tasks/" + id}
}
func emptyBody(r *http.Request) bool { var q struct{}; return decode(r, &q) == nil }
func (a *Server) routes() {
	a.templateRoutes()
	a.enrollmentRoutes()
	a.routerConfigRoutes()
	a.route("GET /api/v1/devices", false, func(r *http.Request) response {
		status := r.URL.Query().Get("status")
		if status != "" && status != "online" && status != "offline" {
			return invalid()
		}
		out := []object{}
		for _, v := range a.app.Inventory() {
			p, _ := a.app.Enrollment().Get(v.Registration.DeviceID)
			if (p.Admission == "managed" || r.URL.Query().Get("admission") == "all") && (status == "" || string(v.Status) == status) {
				out = append(out, a.managedDTO(v))
			}
		}
		return paged(r, out)
	})
	a.route("GET /api/v1/devices/{id}", false, func(r *http.Request) response {
		v, e := a.app.Devices().Get(r.PathValue("id"))
		if e != nil {
			for _, d := range a.app.Inventory() {
				if d.Registration.DeviceID == r.PathValue("id") {
					v = d
					e = nil
					break
				}
			}
		}
		return ok(a.managedDTO(v), e)
	})
	a.route("GET /api/v1/devices/{id}/connections", false, a.connectionHistory)
	a.route("GET /api/v1/devices/{id}/sessions", false, func(r *http.Request) response {
		v, e := a.app.Devices().Sessions(r.PathValue("id"))
		if e != nil {
			return failure(e)
		}
		items := []any{}
		if v.Current != nil {
			items = append(items, session(v.Current))
		}
		for i := len(v.Ended) - 1; i >= 0; i-- {
			items = append(items, session(&v.Ended[i]))
		}
		out := paged(r, items)
		if out.code == "" {
			data := out.data.(object)
			data["history_limit"] = v.Limit
			data["total_sessions"] = v.TotalSessions
			data["evicted_sessions"] = v.EvictedSessions
		}
		return out
	})
	a.route("POST /api/v1/devices/{id}/disconnect", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		v, e := a.app.Disconnect(r.PathValue("id"))
		return ok(object{"disconnected": v}, e)
	})
	a.route("GET /api/v1/tasks", false, func(r *http.Request) response {
		o, l, e := page(r)
		if e != nil {
			return invalid()
		}
		state := r.URL.Query().Get("state")
		switch state {
		case "", "received", "queued", "running", "success", "failed", "timeout", "rejected":
		default:
			return invalid()
		}
		v, total := a.app.Tasks(r.URL.Query().Get("device_id"), state, o, l)
		return response{data: object{"items": v, "total": total, "offset": o, "limit": l}}
	})
	a.route("POST /api/v1/tasks", true, func(r *http.Request) response {
		var q execInput
		if decode(r, &q) != nil || q.DeviceID == "" || q.Command == "" || q.Timeout == 0 {
			return invalid()
		}
		if strings.ContainsRune(q.Command, 0) || strings.ContainsRune(q.Cwd, 0) {
			return invalid()
		}
		for k, v := range q.Env {
			if k == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, 0) {
				return invalid()
			}
		}
		id, e := a.app.CreateExec(r.Context(), q.DeviceID, task.ExecRequest{Command: q.Command, Cwd: q.Cwd, Env: q.Env, Timeout: time.Duration(q.Timeout) * time.Second})
		return accepted(id, object{"task_id": id}, e)
	})
	a.route("GET /api/v1/tasks/{id}", false, func(r *http.Request) response {
		v, e := a.app.TaskSnapshot(r.PathValue("id"))
		return ok(taskDTO(v), e)
	})
	a.route("GET /api/v1/tasks/{id}/result", false, func(r *http.Request) response {
		v, e := a.app.TaskSnapshot(r.PathValue("id"))
		if e != nil {
			return failure(e)
		}
		status := 202
		if v.Result != nil || v.State == task.StateRejected {
			status = 200
		}
		return response{status: status, data: object{"task_id": v.Spec.ID, "state": v.State, "result": resultDTO(v.Result)}}
	})
	a.route("POST /api/v1/tasks/{id}/resend", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		id := r.PathValue("id")
		e := a.app.ResendTask(r.Context(), id)
		if e != nil && !errors.Is(e, gateway.ErrDispatchUncertain) {
			return failure(e)
		}
		return accepted(id, object{"task_id": id}, e)
	})
	a.route("GET /api/v1/tasks/{id}/transfer", false, func(r *http.Request) response {
		v, e := a.app.FileSnapshot(r.PathValue("id"))
		return ok(fileDTO(v), e)
	})
	a.route("GET /api/v1/tasks/{id}/operation", false, func(r *http.Request) response {
		v, e := a.app.Operation(r.PathValue("id"))
		return ok(operationDTO(v), e)
	})
	for _, kind := range []string{"uploads", "downloads", "deployments"} {
		kind := kind
		a.route("POST /api/v1/"+kind, true, func(r *http.Request) response {
			var q transferInput
			if decode(r, &q) != nil || q.DeviceID == "" || q.Timeout == 0 || q.RemotePath == "" {
				return invalid()
			}
			if !strings.HasPrefix(q.RemotePath, "/") || len(q.RemotePath) > 4096 || strings.ContainsRune(q.RemotePath, 0) {
				return invalid()
			}
			var op management.Operation
			var e error
			switch kind {
			case "uploads":
				if q.AssetID == "" || !validMode(q.Mode) || !filetransfer.Name(path.Base(q.RemotePath)) || q.ToolID != "" || q.Version != "" || q.ArtifactID != "" || q.Name != "" {
					return invalid()
				}
				op, e = a.app.Upload(r.Context(), management.UploadRequest{DeviceID: q.DeviceID, AssetID: q.AssetID, RemotePath: q.RemotePath, Mode: q.Mode, Overwrite: q.Overwrite, Timeout: time.Duration(q.Timeout) * time.Second})
			case "downloads":
				if q.AssetID != "" || q.ToolID != "" || q.Version != "" || q.ArtifactID != "" || q.Mode != "" || q.Overwrite {
					return invalid()
				}
				if q.Name == "" || len(q.Name) > 255 || strings.ContainsAny(q.Name, "/\\\x00") || q.Name == "." || q.Name == ".." {
					return invalid()
				}
				op, e = a.app.Download(r.Context(), management.DownloadRequest{DeviceID: q.DeviceID, RemotePath: q.RemotePath, Name: q.Name, Timeout: time.Duration(q.Timeout) * time.Second})
			case "deployments":
				if q.ToolID == "" || q.Version == "" || q.AssetID != "" || q.Mode != "" || q.Name != "" || !filetransfer.Name(path.Base(q.RemotePath)) {
					return invalid()
				}
				op, e = a.app.Deploy(r.Context(), management.DeployRequest{DeviceID: q.DeviceID, ToolID: q.ToolID, Version: q.Version, ArtifactID: q.ArtifactID, RemotePath: q.RemotePath, Overwrite: q.Overwrite, Timeout: time.Duration(q.Timeout) * time.Second})
			}
			return accepted(op.TaskID, operationDTO(op), e)
		})
	}
	a.route("POST /api/v1/downloads/{id}/complete", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		v, e := a.app.CompleteDownload(r.Context(), r.PathValue("id"))
		if e != nil && v.Asset.ID == "" {
			return failure(e)
		}
		data := object{"asset": v.Asset, "transfer": fileDTO(v.File), "task": object{"task_id": v.Task.Spec.ID, "state": v.Task.State}}
		if e != nil {
			data["warning"] = "cleanup_pending"
		}
		return response{data: data}
	})
	a.route("POST /api/v1/downloads/{id}/cleanup", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		return ok(object{"task_id": r.PathValue("id")}, a.app.CleanupDownload(r.PathValue("id")))
	})
	a.route("GET /api/v1/assets", false, func(r *http.Request) response {
		inc, e := archived(r)
		if e != nil {
			return invalid()
		}
		v, e := a.app.Files().List(inc)
		if e != nil {
			return failure(e)
		}
		return paged(r, v)
	})
	a.route("POST /api/v1/assets", true, func(r *http.Request) response {
		name := r.URL.Query().Get("name")
		if name == "" || len(name) > 255 || strings.ContainsRune(name, 0) {
			return invalid()
		}
		v, e := a.app.Files().ImportVerified(r.Context(), name, r.Body, r.ContentLength, r.Header.Get("X-Content-SHA256"))
		if errors.Is(e, repository.ErrIntegrity) {
			return response{status: 422, code: "integrity_mismatch"}
		}
		if e != nil {
			return failure(e)
		}
		return response{status: 201, data: v, location: "/api/v1/assets/" + v.ID}
	})
	a.route("GET /api/v1/assets/{id}", false, func(r *http.Request) response { v, e := a.app.Files().Get(r.PathValue("id")); return ok(v, e) })
	a.route("POST /api/v1/assets/{id}/archive", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		return ok(object{"archived": true}, a.app.Files().Archive(r.PathValue("id")))
	})
	a.mux.HandleFunc("GET /api/v1/assets/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		started := false
		e := a.app.Files().ReadContent(r.Context(), r.PathValue("id"), func(asset repository.Asset, reader io.ReadSeeker) error {
			started = true
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment")
			w.Header().Set("X-Content-SHA256", asset.SHA256)
			http.ServeContent(&contentWriter{ResponseWriter: w}, r, asset.Name, asset.CreatedAt, reader)
			return nil
		})
		if e != nil && !started {
			write(w, failure(e))
		}
	})
	a.route("GET /api/v1/tools", false, func(r *http.Request) response {
		inc, e := archived(r)
		if e != nil {
			return invalid()
		}
		v, e := a.app.Tools().List(inc)
		if e != nil {
			return failure(e)
		}
		return paged(r, v)
	})
	a.route("POST /api/v1/tools", true, func(r *http.Request) response {
		var q struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if decode(r, &q) != nil || q.Name == "" || len(q.Name) > 255 || len(q.Description) > 4096 || strings.ContainsRune(q.Name+q.Description, 0) {
			return invalid()
		}
		v, e := a.app.Tools().Create(q.Name, q.Description)
		if e != nil {
			return failure(e)
		}
		return response{status: 201, data: v, location: "/api/v1/tools/" + v.ID}
	})
	a.route("GET /api/v1/tools/{id}", false, func(r *http.Request) response { v, e := a.app.Tools().Get(r.PathValue("id")); return ok(v, e) })
	a.route("POST /api/v1/tools/{id}/archive", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		return ok(object{"archived": true}, a.app.Tools().Archive(r.PathValue("id")))
	})
	a.route("GET /api/v1/tools/{id}/versions", false, func(r *http.Request) response {
		inc, e := archived(r)
		if e != nil {
			return invalid()
		}
		v, e := a.app.Tools().Versions(r.PathValue("id"), inc)
		if e != nil {
			return failure(e)
		}
		return paged(r, v)
	})
	a.route("PUT /api/v1/tools/{id}/versions/{version}", true, func(r *http.Request) response {
		var q struct {
			Artifacts []struct {
				AssetID  string           `json:"asset_id"`
				Platform string           `json:"platform"`
				Mode     string           `json:"mode"`
				Rules    repository.Rules `json:"rules"`
			} `json:"artifacts"`
		}
		if decode(r, &q) != nil {
			return invalid()
		}
		specs := []repository.ArtifactSpec{}
		for _, v := range q.Artifacts {
			specs = append(specs, repository.ArtifactSpec{AssetID: v.AssetID, Platform: v.Platform, Mode: v.Mode, Rules: v.Rules})
		}
		v, e := a.app.Tools().Publish(r.PathValue("id"), r.PathValue("version"), specs)
		return ok(v, e)
	})
	a.route("GET /api/v1/tools/{id}/versions/{version}", false, func(r *http.Request) response {
		v, e := a.app.Tools().Version(r.PathValue("id"), r.PathValue("version"))
		return ok(v, e)
	})
	a.route("POST /api/v1/tools/{id}/versions/{version}/archive", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		return ok(object{"archived": true}, a.app.Tools().ArchiveVersion(r.PathValue("id"), r.PathValue("version")))
	})
	a.route("GET /api/v1/tools/{id}/versions/{version}/compatibility", false, func(r *http.Request) response {
		if r.URL.Query().Get("device_id") == "" {
			return invalid()
		}
		v, e := a.app.Compatibility(r.URL.Query().Get("device_id"), r.PathValue("id"), r.PathValue("version"))
		if e != nil {
			return failure(e)
		}
		out := []object{}
		for _, c := range v {
			checks := []object{}
			for _, x := range c.Match.Checks {
				checks = append(checks, object{"field": x.Field, "status": x.Status, "reason": x.Reason})
			}
			out = append(out, object{"artifact": c.Artifact, "status": c.Match.Status, "checks": checks})
		}
		return paged(r, out)
	})
	a.route("GET /api/v1/maintenance", false, func(r *http.Request) response {
		if a.app.Maintenance() == nil {
			return response{status: 503, code: "maintenance_disabled"}
		}
		out := []object{}
		state := r.URL.Query().Get("state")
		if state != "" && state != "ready" && state != "closing" && state != "closed" {
			return invalid()
		}
		for _, v := range a.app.Maintenance().List() {
			if (r.URL.Query().Get("device_id") == "" || r.URL.Query().Get("device_id") == v.DeviceID) && (state == "" || state == v.State) {
				out = append(out, maintenanceDTO(v))
			}
		}
		return paged(r, out)
	})
	a.route("POST /api/v1/maintenance", true, func(r *http.Request) response {
		var q struct {
			DeviceID string `json:"device_id"`
			LeaseMS  *int64 `json:"lease_ms"`
		}
		if decode(r, &q) != nil || q.DeviceID == "" {
			return invalid()
		}
		var lease time.Duration
		if q.LeaseMS != nil {
			n := *q.LeaseMS
			if n < 1 || n > int64((1<<63-1)/time.Millisecond) {
				return invalid()
			}
			lease = time.Duration(n) * time.Millisecond
		}
		if a.app.Maintenance() == nil {
			return response{status: 503, code: "maintenance_disabled"}
		}
		if _, e := a.app.Devices().Get(q.DeviceID); e != nil {
			return failure(e)
		}
		v, e := a.app.Maintenance().Create(r.Context(), q.DeviceID, lease)
		if e != nil {
			return failure(e)
		}
		return response{status: 201, data: maintenanceDTO(v), location: "/api/v1/maintenance/" + v.ID}
	})
	a.route("GET /api/v1/maintenance/{id}", false, func(r *http.Request) response {
		if a.app.Maintenance() == nil {
			return response{status: 503, code: "maintenance_disabled"}
		}
		v, e := a.app.Maintenance().Get(r.PathValue("id"))
		return ok(maintenanceDTO(v), e)
	})
	a.route("POST /api/v1/maintenance/{id}/close", true, func(r *http.Request) response {
		if !emptyBody(r) {
			return invalid()
		}
		if a.app.Maintenance() == nil {
			return response{status: 503, code: "maintenance_disabled"}
		}
		return ok(object{"maintenance_id": r.PathValue("id"), "released": true}, a.app.Maintenance().CloseMaintenance(r.PathValue("id")))
	})
}
func validMode(v string) bool {
	if len(v) != 4 || v[0] != '0' {
		return false
	}
	for _, c := range v[1:] {
		if c < '0' || c > '7' {
			return false
		}
	}
	return true
}
