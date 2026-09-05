package repository

import (
	"routerprobe/internal/device"
	"strings"
)

type Compatibility string

const (
	Compatible   Compatibility = "compatible"
	Incompatible Compatibility = "incompatible"
	Unknown      Compatibility = "unknown"
)

type Check struct {
	Field  string
	Status Compatibility
	Reason string
}
type MatchResult struct {
	Status Compatibility
	Checks []Check
}

func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}

// Match evaluates declarations, not execution success or authorization. Device
// online state is an independent dispatch prerequisite, not compatibility data.
func Match(a Artifact, d device.Registration) MatchResult {
	out := MatchResult{Status: Compatible, Checks: []Check{}}
	add := func(field string, status Compatibility, reason string) {
		out.Checks = append(out.Checks, Check{field, status, reason})
		if status == Incompatible || status == Unknown && out.Status == Compatible {
			out.Status = status
		}
	}
	if _, e := normalizeSpec(ArtifactSpec{a.AssetID, a.Platform, a.Mode, a.Rules}); e != nil {
		add("artifact", Incompatible, "invalid artifact constraints")
		return out
	}
	add("platform", Compatible, "existing Probe platform is linux")
	for _, v := range []struct {
		field, value string
		allowed      []string
		any          bool
	}{{"arch", normalizeArch(d.Arch), a.Rules.Arch, true}, {"libc", strings.ToLower(d.Libc), a.Rules.Libc, true}, {"model", d.Model, a.Rules.Models, false}, {"kernel", d.Kernel, a.Rules.Kernels, false}} {
		allowed, _ := normalizeSet(v.allowed, v.field, v.any)
		if !v.any && len(allowed) == 0 || v.any && len(allowed) == 1 && allowed[0] == "any" {
			add(v.field, Compatible, "explicitly unrestricted")
		} else if v.value == "" {
			add(v.field, Unknown, "device did not declare constrained field")
		} else if contains(allowed, v.value) {
			add(v.field, Compatible, "allowed value")
		} else {
			add(v.field, Incompatible, "declared value is not allowed")
		}
	}
	for _, cap := range a.Rules.RequiredCapabilities {
		if contains(d.Capabilities, cap) {
			add("capability:"+cap, Compatible, "declared")
		} else {
			add("capability:"+cap, Incompatible, "required capability not declared")
		}
	}
	return out
}
