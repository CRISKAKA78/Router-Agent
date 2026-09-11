package management

import (
	"context"
	"encoding/json"
	"errors"
	"routerprobe/internal/device"
	"routerprobe/internal/enrollment"
	"routerprobe/internal/filetransfer"
	"routerprobe/internal/gateway"
	"routerprobe/internal/overlay"
	"routerprobe/internal/routerconfig"

	"slices"
	"time"
)

// overlayDriver composes the existing task/file/compatibility services. It is
// not a second downloader and never distributes the overlay network secret.
type overlayDriver struct {
	s      *Server
	config overlay.Config
}

func (d *overlayDriver) Online(id string) bool {
	v, e := d.s.Devices().Get(id)
	return e == nil && v.Status == device.Online
}
func (d *overlayDriver) ValidateDevice(id string) error {
	v, e := d.s.Devices().Get(id)
	if e != nil {
		return e
	}
	p, e := d.s.enrollment.Get(id)
	if e != nil {
		return e
	}
	if p.Admission != "managed" {
		return enrollment.ErrNotManaged
	}
	if v.Status != device.Online || v.CurrentSession == nil {
		return ErrOffline
	}
	if !slices.Contains(v.Registration.Capabilities, "network_agent_v1") {
		return routerconfig.ErrUnsupported
	}
	return nil
}
func (d *overlayDriver) run(ctx context.Context, m overlay.Member, action string, report overlay.Reporter) (overlay.AgentInfo, error) {
	var info overlay.AgentInfo
	p := overlay.AgentRequest{Action: action, Directory: d.config.InstallDirectory, MachineID: m.MachineID}
	if action == "start" {
		p.ConfigServer = d.config.ConfigServerURL
	}
	id, e := d.s.gateway.CreateNetworkAgent(ctx, m.DeviceID, p)
	if id != "" {
		if x := report("engine_"+action, id); x != nil {
			return info, overlay.ErrUncertain
		}
	}
	if e != nil {
		if id != "" || errors.Is(e, gateway.ErrDispatchUncertain) {
			return info, overlay.ErrUncertain
		}
		return info, e
	}
	result, e := d.s.gateway.WaitTaskResult(ctx, id)
	if e != nil {
		return info, overlay.ErrUncertain
	}
	if result.Status != "success" || result.Truncated {
		return info, errors.New("engine_bootstrap_failed")
	}
	if e = json.Unmarshal([]byte(result.Stdout), &info); e != nil || info.MachineID != m.MachineID {
		return info, overlay.ErrUncertain
	}
	return info, nil
}
func (d *overlayDriver) Bootstrap(ctx context.Context, m overlay.Member, report overlay.Reporter) error {
	info, e := d.run(ctx, m, "inspect", report)
	if e != nil {
		return e
	}
	if !info.Installed {
		if d.config.ToolID == "" {
			return overlay.ErrDisabled
		}
		// Resolve ambiguity before creating any remote state.
		candidates, e := d.s.Compatibility(m.DeviceID, d.config.ToolID, d.config.ToolVersion)
		if e != nil {
			return e
		}
		selected := ""
		for _, v := range candidates {
			if v.Match.Status == "compatible" {
				if selected != "" {
					return ErrAmbiguous
				}
				selected = v.Artifact.ID
			}
		}
		if selected == "" {
			return ErrIncompatible
		}
		if _, e = d.run(ctx, m, "prepare", report); e != nil {
			return e
		}
		op, e := d.s.Deploy(ctx, DeployRequest{DeviceID: m.DeviceID, ToolID: d.config.ToolID, Version: d.config.ToolVersion, ArtifactID: selected, RemotePath: d.config.InstallDirectory + "/package", Overwrite: true, Timeout: 90 * time.Second})
		if op.TaskID != "" {
			if x := report("package_transfer", op.TaskID); x != nil {
				return overlay.ErrUncertain
			}
		}
		if e != nil {
			if op.TaskID != "" {
				return overlay.ErrUncertain
			}
			return e
		}
		result, e := d.s.gateway.WaitTaskResult(ctx, op.TaskID)
		if e != nil {
			return overlay.ErrUncertain
		}
		if result.Status != "success" {
			return errors.New("engine_package_transfer_failed")
		}
		if e = waitNetworkPackageRelease(ctx, func() (filetransfer.Snapshot, error) { return d.s.FileSnapshot(op.TaskID) }); e != nil {
			return e
		}
		if _, e = d.run(ctx, m, "install", report); e != nil {
			return e
		}
	}
	if !info.Running {
		_, e = d.run(ctx, m, "start", report)
		if e != nil {
			return e
		}
	}
	// core's web client registration is asynchronous. Poll only a harmless GET;
	// the actual configuration write is performed exactly once by the service.
	return nil
}
func (s *Server) Networks() *overlay.Service { return s.networks }

// Keep the compile-time lifecycle contract explicit.
var _ overlay.Driver = (*overlayDriver)(nil)

// Missing tasks after restart remain uncertain: never manufacture replacement
// task IDs when the old Probe may still be executing the original command.
func (d *overlayDriver) TasksSettled(ids []string) bool {
	for _, id := range ids {
		v, e := d.s.TaskSnapshot(id)
		if e != nil || v.Result == nil {
			return false
		}
		if v.Spec.Type == "upload" {
			f, e := d.s.FileSnapshot(id)
			if e != nil || !f.Released {
				return false
			}
		}
	}
	return true
}

// Upload success is validated by Gateway against the expected transfer ID, size
// and SHA-256. Committed/Size/SHA256 in the local snapshot describe downloads,
// not uploads. Wait for the sender to release its handles before installation.
func waitNetworkPackageRelease(ctx context.Context, snapshot func() (filetransfer.Snapshot, error)) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		f, err := snapshot()
		if err != nil || f.Error != "" {
			return overlay.ErrUncertain
		}
		if f.Released {
			return nil
		}
		select {
		case <-ctx.Done():
			return overlay.ErrUncertain
		case <-ticker.C:
		}
	}
}
