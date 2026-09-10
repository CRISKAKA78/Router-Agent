// Package api adapts the Management application to HTTP and WebSocket.
package api

import (
	"routerprobe/internal/device"
	"routerprobe/internal/filetransfer"
	"routerprobe/internal/management"
	"routerprobe/internal/task"
	"routerprobe/internal/tunnel"
	"time"
)

type object = map[string]any

func timestamp(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}
func registration(v device.Registration) object {
	return object{"device_id": v.DeviceID, "serial": v.Serial, "model": v.Model, "firmware": v.Firmware, "probe_version": v.ProbeVersion, "hostname": v.Hostname, "arch": v.Arch, "kernel": v.Kernel, "libc": v.Libc, "boot_id": v.BootID, "capabilities": v.Capabilities}
}
func runtimeDTO(v *device.Runtime) any {
	if v == nil {
		return nil
	}
	return object{"uptime_seconds": v.UptimeSeconds, "reported_at": timestamp(v.ReportedAt)}
}
func session(v *device.Session) any {
	if v == nil {
		return nil
	}
	var active any
	var presentation any
	if v.ConfigTemplate != nil {
		active = object{"template_id": v.ConfigTemplate.ID, "name": v.ConfigTemplate.Name, "version": v.ConfigTemplate.Version}
		presentation = v.ConfigTemplate.Presentation
	}
	return object{"active_template": active, "presentation": presentation, "applied_revision": v.ConfigRevision, "source_ip": v.Registration.SourceIP, "effective_metrics": device.EffectiveMetrics(*v, time.Now()), "session_id": v.ID, "registration": registration(v.Registration), "runtime": runtimeDTO(v.Runtime), "started_at": timestamp(v.StartedAt), "last_seen_at": timestamp(v.LastSeenAt), "ended_at": timestamp(v.EndedAt), "end_reason": v.EndReason}
}
func deviceDTO(v device.Snapshot) object {
	return object{"neighbors": device.NeighborSnapshot(v, time.Now()), "source_ip": v.LatestSession.Registration.SourceIP, "effective_metrics": device.EffectiveMetrics(v.LatestSession, time.Now()), "device_id": v.Registration.DeviceID, "registration": registration(v.Registration), "runtime": runtimeDTO(v.LatestSession.Runtime), "status": v.Status, "first_seen_at": timestamp(v.FirstSeenAt), "last_seen_at": timestamp(v.LastSeenAt), "last_online_at": timestamp(v.LastOnlineAt), "last_offline_at": timestamp(v.LastOfflineAt), "current_session": session(v.CurrentSession), "latest_session": session(&v.LatestSession), "total_sessions": v.TotalSessions, "evicted_sessions": v.EvictedSessions}
}
func taskDTO(v task.Snapshot) object {
	var last any
	if len(v.Dispatches) > 0 {
		last = v.Dispatches[len(v.Dispatches)-1].SessionID
	}
	return object{"task_id": v.Spec.ID, "device_id": v.Spec.DeviceID, "type": v.Spec.Type, "state": v.State, "created_at": time.Unix(v.Spec.CreatedAt, 0).UTC(), "timeout_seconds": v.Spec.Timeout, "command": v.Spec.Command, "cwd": v.Spec.Cwd, "env": v.Spec.Env, "params": v.Spec.Params, "last_session_id": last, "dispatch_count": len(v.Dispatches), "result": resultDTO(v.Result)}
}
func resultDTO(v *task.Result) any {
	if v == nil {
		return nil
	}
	return object{"task_id": v.TaskID, "status": v.Status, "started_at": time.Unix(v.StartedAt, 0).UTC(), "finished_at": time.Unix(v.FinishedAt, 0).UTC(), "exit_code": v.ExitCode, "stdout": v.Stdout, "stderr": v.Stderr, "truncated": v.Truncated, "result": v.Details}
}
func fileDTO(v filetransfer.Snapshot) object {
	// Local path and transport diagnostics can contain deployment-private details.
	return object{"task_id": v.TaskID, "transfer_id": v.TransferID, "committed": v.Committed, "released": v.Released, "size": v.Size, "sha256": v.SHA256, "failed": v.Error != ""}
}
func operationDTO(v management.Operation) object {
	return object{"task_id": v.TaskID, "transfer_id": v.TransferID, "device_id": v.DeviceID, "session_id": v.SessionID, "tool_id": v.ToolID, "version": v.Version, "artifact_id": v.ArtifactID, "asset_id": v.AssetID}
}
func maintenanceDTO(v tunnel.Snapshot) object {
	endpoints := []object{}
	for _, e := range v.Endpoints {
		entry := object{"service": e.Service, "host": e.Host, "port": e.Port, "address": e.Address(), "state": e.State}
		if e.Service == "web" {
			entry["url"] = "http://" + e.Address() + "/"
		}
		endpoints = append(endpoints, entry)
	}
	return object{"maintenance_id": v.ID, "device_id": v.DeviceID, "session_id": v.SessionID, "state": v.State, "reason": v.Reason, "created_at": timestamp(v.CreatedAt), "expires_at": timestamp(v.ExpiresAt), "released": v.Released, "reusable_after": timestamp(v.ReusableAfter), "connections": v.Connections, "endpoints": endpoints}
}
