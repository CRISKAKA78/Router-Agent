package repository

import (
	"routerprobe/internal/device"
	"testing"
)

func TestCompatibilityMatrix(t *testing.T) {
	id, _ := uuid()
	a := Artifact{AssetID: id, Platform: "linux", Mode: "0755", Rules: Rules{Arch: []string{"mipsel"}, Libc: []string{"uclibc"}, Models: []string{"F3"}, Kernels: []string{"3.10.14-vendor"}, RequiredCapabilities: []string{"file", "exec"}}}
	d := device.Registration{Arch: "mipsel", Libc: "uClibc", Model: "F3", Kernel: "3.10.14-vendor", Capabilities: []string{"file", "exec", "future"}}
	tests := []struct {
		name   string
		change func(*Artifact, *device.Registration)
		want   Compatibility
	}{
		{"full", func(*Artifact, *device.Registration) {}, Compatible},
		{"missing libc", func(a *Artifact, d *device.Registration) { d.Libc = "" }, Unknown},
		{"missing kernel", func(a *Artifact, d *device.Registration) { d.Kernel = "" }, Unknown},
		{"missing model", func(a *Artifact, d *device.Registration) { d.Model = "" }, Unknown},
		{"endian", func(a *Artifact, d *device.Registration) { d.Arch = "mips" }, Incompatible},
		{"no arm inference", func(a *Artifact, d *device.Registration) { a.Rules.Arch = []string{"armv7"}; d.Arch = "arm" }, Incompatible},
		{"arm alias", func(a *Artifact, d *device.Registration) { a.Rules.Arch = []string{"arm64"}; d.Arch = "aarch64" }, Compatible},
		{"amd64 alias", func(a *Artifact, d *device.Registration) { a.Rules.Arch = []string{"x86_64"}; d.Arch = "amd64" }, Compatible},
		{"arch case exact", func(a *Artifact, d *device.Registration) { d.Arch = "MIPSEL" }, Incompatible},
		{"libc no inference", func(a *Artifact, d *device.Registration) { d.Libc = "musl" }, Incompatible},
		{"model case", func(a *Artifact, d *device.Registration) { d.Model = "f3" }, Incompatible},
		{"kernel suffix", func(a *Artifact, d *device.Registration) { d.Kernel = "3.10.14-other" }, Incompatible},
		{"kernel newer", func(a *Artifact, d *device.Registration) { d.Kernel = "6.0.0" }, Incompatible},
		{"cap subset", func(a *Artifact, d *device.Registration) { d.Capabilities = []string{"file"} }, Incompatible},
		{"unknown and mismatch", func(a *Artifact, d *device.Registration) { d.Libc = ""; d.Model = "wrong" }, Incompatible},
		{"explicit any", func(a *Artifact, d *device.Registration) {
			a.Rules = Rules{Arch: []string{"any"}, Libc: []string{"any"}}
			d.Libc = ""
			d.Model = ""
			d.Kernel = ""
		}, Compatible},
		{"empty arch invalid", func(a *Artifact, d *device.Registration) { a.Rules.Arch = nil }, Incompatible},
		{"any mixture invalid", func(a *Artifact, d *device.Registration) { a.Rules.Libc = []string{"any", "uclibc"} }, Incompatible},
		{"platform invalid", func(a *Artifact, d *device.Registration) { a.Platform = "windows" }, Incompatible},
		{"or within field", func(a *Artifact, d *device.Registration) { a.Rules.Arch = []string{"x86_64", "mipsel"} }, Compatible},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			aa := cloneVersion(Version{Artifacts: []Artifact{a}}).Artifacts[0]
			dd := d
			test.change(&aa, &dd)
			got := Match(aa, dd)
			if got.Status != test.want || len(got.Checks) == 0 {
				t.Fatalf("%+v want %s", got, test.want)
			}
		})
	}
}
