# GOST v3.3.0 isolated PoC

These are evaluation programs, **not a product backend**. They do not install GOST,
change Router-Agent processes/configuration, access SSH credentials, or open physical serial devices.
The full Linux run includes deliberately unmet product requirements; individual JSON `pass` values,
not process exit 0 or the number of records, determine the result. Exit 0 currently means evidence
collection finished; setup exceptions return 1. Samples/observations are not functional test counts.

## Fixed inputs

- GOST v3.3.0 release, source revision `cb76f63754768c7b5d68895a0d51635b0141b80f`.
- Official build: Go 1.26.7, go-gost/x v0.16.0. Use artifact `go version -m` to verify.
- Fetcher requires Python 3.12+ (tar safe extraction); smoke runner requires Windows Python 3.8+.
- Linux runner uses only Go standard library, Linux root, `unshare`, `nsenter`, BusyBox `ip`, mount and devpts.
- No Docker, package installation or third-party Python libraries are needed.

Run from the repository root:

```powershell
python tests/poc/gostv3/fetch_release.py --output build/gost-poc-20260911
$env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
go build -o build/gost-poc-20260911/runner tests/poc/gostv3/main_linux.go
Remove-Item Env:GOOS,Env:GOARCH,Env:CGO_ENABLED
python tests/poc/gostv3/windows_smoke.py --gost build/gost-poc-20260911/windows-amd64/gost.exe --output build/gost-poc-20260911/windows-smoke-new
```

The fetcher verifies upstream SHA256 before extraction; it does not independently authenticate the
publisher or perform a complete dependency-license audit. Failed/partial downloads are not used.

## Linux execution

Reuse RouterAgentTest as documented in docs/WSL_TEST_ENVIRONMENT.md. Copy the official AMD64 `gost`
and the compiled `runner` into a new `/work-runs/gost-poc-...` directory using the documented UNC
copy or a Python subprocess binary stdin to `wsl ... tee <exact-output-file>` with stdout discarded.
Do not bind-mount the Windows workspace or run the binary on the production router/server.

Run **only in the private network/mount/PID namespaces** below. A different output directory is
required for every run so old evidence is not overwritten. Set GOST_POC_FOCUS=1 for only the
configuration-workaround/parent-death follow-up, otherwise omit it for the full evaluation.

```sh
unshare -mnpf --mount-proc sh -c '
  set -e
  mount --make-rprivate /
  ip link set lo up
  mount -t devpts devpts /dev/pts -o newinstance,ptmxmode=0666,mode=0620
  export GOST_BIN=/work-runs/gost-poc-20260911/gost
  /work-runs/gost-poc-20260911/runner /work-runs/gost-poc-20260911/another-run
'
```

The runner sets the **private namespace's** ephemeral source range to 45000–60999 to keep test
service ports out of its way. It creates a second network namespace and veth pair using BusyBox
ip's default peer name `veth0`; do not substitute this with an unverified production interface.
Target IP is 198.18.10.2/24 and Probe-side IP is 198.18.10.1/24. UDP/TCP echo servers are actual
sockets in the separate target namespace, not mocked GOST responses. Traffic and TLS keys are
synthetic test data. The generated `test-key.pem` is only a one-hour local test certificate key.

All directly spawned processes are stopped on completion. The parent-death follow-up explicitly
checks the PID and executable of its own child before killing it; exiting the outer PID namespace
is the final containment boundary for any remaining descendants. No system-wide pkill is used.
Do not infer that GOST itself offers this containment.

## Evidence and scope

- `results.json`: named exact-byte checks and descriptive observations.
- `*-config.json`, `tuned-config.json`, and serial JSON files: configurations actually tested.
- `*.log`: GOST status/errors; `target.log`: observed LAN source IP.
- `/proc` samples: AMD64 RSS/high-water mark/fd/thread count, not ARM RAM measurements.
- PTY termios confirms configured 9600/115200, not physical timing or electrical compatibility.
- Windows smoke covers reverse TCP/UDP and deletion of an active connection on loopback only.

The full run exposes default large/empty UDP behavior, absent quarantine, default serial
multi-writer behavior and a reverse-listener connection-limit disruption. The focused run tries
a second local service inside the same GOST process; TCP ownership works in the recorded test,
UDP ownership does not. Preserve those failures rather than reducing byte-equality assertions.

Do not claim a one-hour soak, real UART/USB unplug, source allowlist, WAN-loss latency,
Router-Agent lease/session integration, or ARM execution from these tests.
The authoritative result and review gates are in docs/GOST_V3_POC.md.
