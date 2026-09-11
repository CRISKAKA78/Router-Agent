"""Build optional forwarding bundles locally; never deploy or restart processes.
Requires the extracted GOST 3.3.0 tree and sibling x v0.16.0 patched with
 tests/poc/gostv3/patches/apply_patches.py. WPF and vendor Probe use their own builds.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[1]
p = argparse.ArgumentParser(description=__doc__)
p.add_argument("--gost-source", required=True, type=Path)
p.add_argument("--output", type=Path, default=ROOT / "build/forwarding-release")
a = p.parse_args()
source = a.gost_source.resolve()
out = a.output.resolve()
if not out.is_relative_to(ROOT / "build"):
    p.error("output must be a directory beneath this worktree's build directory")
manifest = json.loads((ROOT / "tests/poc/gostv3/patches/manifest.json").read_text())
for name, expected in manifest["files"].items():
    raw = (source.parent / "x" / name).read_bytes()
    if hashlib.sha256(raw).hexdigest() != expected["after"]:
        p.error("patch verification failed: " + name)
mod = (source / "go.mod").read_text()
if "github.com/go-gost/x v0.16.0" not in mod or "replace github.com/go-gost/x => ../x" not in mod:
    p.error("expected pinned x v0.16.0 with sibling replacement")

def build(cwd, target, system, arch, package, gost=False):
    target.parent.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, GOOS=system, GOARCH=arch, CGO_ENABLED="0")
    if arch == "arm": env["GOARM"] = "7"
    if gost: env["GOTOOLCHAIN"] = "go1.26.7"
    subprocess.run(["go", "build", "-trimpath", "-ldflags=-s -w", "-o", str(target), package], cwd=cwd, env=env, check=True)

for folder, system, arch in [("server-windows-amd64", "windows", "amd64"), ("server-linux-amd64", "linux", "amd64"), ("device-linux-armv7", "linux", "arm")]:
    d = out / folder
    ext = ".exe" if system == "windows" else ""
    package = "./cmd/forwarding-agent" if arch == "arm" else "./cmd/server"
    name = "router-forwarding-agent" if arch == "arm" else "router-server" + ext
    build(ROOT, d / name, system, arch, package)
    build(source, d / ("gost" + ext), system, arch, "./cmd/gost", gost=True)
    shutil.copy2(source / "LICENSE", d / "GOST-LICENSE")
shutil.copy2(ROOT / "docs/FORWARDING_IMPLEMENTATION.md", out / "README.md")
print("Local bundles:", out)
print("Device bundle still requires the vendor-built router-agent. No deployment performed.")
