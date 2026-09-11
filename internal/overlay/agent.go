package overlay

import "strings"

// AgentRequest is a bounded bootstrap operation. Network configuration and
// network secrets belong to the upstream controller, not this Probe task.
type AgentRequest struct {
	Action       string `json:"action"`
	Directory    string `json:"directory"`
	MachineID    string `json:"machine_id"`
	ConfigServer string `json:"config_server,omitempty"`
}

func (p AgentRequest) Validate() error {
	if !ValidDirectory(p.Directory) || !ValidUUID(p.MachineID) {
		return ErrInvalid
	}
	if p.Action != "prepare" && p.Action != "inspect" && p.Action != "install" && p.Action != "start" {
		return ErrInvalid
	}
	if p.Action == "start" {
		if !ValidConfigServer(p.ConfigServer) {
			return ErrInvalid
		}
	} else if p.ConfigServer != "" {
		return ErrInvalid
	}
	if strings.ContainsAny(p.Directory, ";\"'`$|&<>(){}") {
		return ErrInvalid
	}
	return nil
}

type AgentInfo struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Tun       bool   `json:"tun"`
	Version   string `json:"version"`
	MachineID string `json:"machine_id"`
}
