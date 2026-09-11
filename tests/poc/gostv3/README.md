# GOST v3.3.0 isolated PoC

These are evaluation programs, **not a product backend**. The original local runners do not
install GOST, change Router-Agent processes/configuration, access SSH credentials, or open physical
serial devices. The explicitly authorized device runner below uses SSH and an isolated temporary
GOST copy; it never changes Router-Agent, routing/firewall settings, or physical UARTs.
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

Current scope: serial is TCP-only and requires client authentication; LAN forwarding retains
TCP and UDP. Serial UDP cases were removed from current runners at the user's explicit request;
previous UDP-serial evidence is historical, not a current acceptance gate. Raw TCP/PTY runners
remain backend diagnostics, not proof that a publicly accessible serial endpoint is safe.
The historical TCP local-bridge result preserves the existing owner but did not test the next
owner; the authentication run below now shows that re-acceptance failure. Preserve both facts.

Do not claim a one-hour soak, real UART/USB unplug, source allowlist, WAN-loss latency,
Router-Agent lease/session integration, or ARM execution from the original local runners.
The authoritative result and review gates are in docs/GOST_V3_POC.md.

## Explicitly authorized ARM device test

`device_smoke.py` and `devicehelper/main_linux.go` are separate from the namespace runner.
**Never run the original `main_linux.go` directly on the production router.** The device fixture
opens loopback TCP/UDP echo sockets and three independent PTYs; its HTTP API is loopback-only.
Separate PTYs isolate baud and ownership cases; they do not prove reopening the same UART works.
The helper exits after 12 minutes. Device GOST runs under existing BusyBox timeout for at most
600 seconds. The Python runner also cleans up by exact owned executable path in `finally`, then
stops only its own local GOST/SSH children. Logs and package files remain in the remote directory.

Preconditions:
- User has explicitly authorized the target device and SSH endpoint.
- Inspect archive entries, available tmpfs/RAM and existing serial owners before extraction.
- Extract only regular `gost` and license entries into a new private `/tmp/root/gost-poc-*`
  directory. Keep the supplied archive and all existing product files unchanged.
- Device loopback ports 19400–19405 must be unused; preflight refuses existing socket entries.
- Existing `/tmp/root/busybox-ipq` must support `timeout -s TERM`.
- Windows OpenSSH supports the selected device's SSH forwarding. An AskPass helper must read
  `RMP_POC_SSH_PASSWORD`; supply that variable transiently, never commit or log its value.

Build the bounded helper with the available Go toolchain (record the exact version):

```powershell
$env:GOOS='linux'; $env:GOARCH='arm'; $env:GOARM='7'; $env:CGO_ENABLED='0'
go build -trimpath -ldflags='-s -w' -o build/device-poc/device-helper tests/poc/gostv3/devicehelper/main_linux.go
go vet ./tests/poc/gostv3/devicehelper
Remove-Item Env:GOOS,Env:GOARCH,Env:GOARM,Env:CGO_ENABLED
```

Invoke `python tests/poc/gostv3/device_smoke.py --help` for required explicit SSH host/port,
remote directory, helper, Windows GOST, AskPass and fresh evidence output paths. The fixture is
uploaded compressed (gzip integrity and decompressed length checked). A failed upload is not
executed; preserve partial evidence and use a new remote directory for another run.

Topology: Windows synthetic TCP/UDP clients → Windows GOST loopback reverse listener → GOST
relay over an SSH `-R` TCP channel → ARM GOST → device loopback echo or PTY. Local `-L` channels
expose only the device fixture/GOST APIs to the runner. UDP datagrams are encapsulated within
GOST before SSH transport; this is not raw UDP forwarding by SSH. No public GOST listener,
management Server change, direct LAN target or physical serial write is involved. These results
prove ARM execution and byte handling in this path, not direct WAN throughput, physical UART
operation or a new real-LAN gateway test. SSH itself travels through the pre-existing SSH
endpoint supplied by the user; this PoC topology is not the proposed production data plane.

`results.json` separates `kind=observation` from checks. Exit 2 means a check failed, including
infrastructure/cleanup failure; preserve these in the report. CPU samples report raw process ticks
and wall time, not an assumed kernel tick rate or a one-hour soak.

## TCP serial authentication evaluation (local only)

`auth_smoke.py` runs the pinned Windows GOST against an observed TCP sink, not a physical UART.
It compares raw TCP `handler.auth` with an authenticated Relay handler using a fixed target.
It verifies that missing/wrong auth, malformed input, partial handshakes and idle sockets never
connect to the sink; checks fragmented/coalesced auth plus binary data; and checks handshake
bytes never reach the sink and a requested target cannot override the configured target.

```powershell
python tests/poc/gostv3/auth_smoke.py --gost build/device-poc/gost.exe --output build/auth-poc/fresh-run
```

All sockets are loopback, credentials are freshly generated synthetic values, and only the test's
own processes are stopped. Configuration/evidence stays under ignored build. Plain TCP auth
bypass is an observation of an unsuitable configuration, not an accepted product behavior.
The final acceptance check intentionally preserves the known next-owner failure after the stock
connection limiter rejects a competing authenticated connection (exit 2). This confirms that a
configuration-only authentication+exclusive-owner combination is not yet sufficient.

The raw handshake encoder exercises the existing Relay v0.7.0 framing. It is not a new product
wire protocol or an implementation of a custom text-token gateway. Public-side TLS, real reverse
mapping bypass prevention, resource-specific secrets/rotation/revocation, bad-client quotas and
real UART behavior still need integration validation. See docs/GOST_V3_POC.md section 4.3.

## Selected registration gateway and bounded GOST patch (ADR-063)

The external client is a Windows network-debugging tool, **not a GOST/Relay client**.
Send ASCII `AUTH <64 lowercase hex token>` followed by real CRLF (LF also accepted),
wait for `OK\r\n`, then send raw text or binary. No banner. Never send the visible
characters `\r\n` as a substitute for the line-ending bytes. This bearer credential
is not encryption and does not prevent sniffing/replay.

`internal/serialauth` is the tested implementation. `registration` is an isolated command,
not a production Server endpoint: both listen/backend must be literal loopback addresses.
It reads `RMP_SERIAL_REGISTRATION_TOKEN` from its environment (never CLI arguments/logs),
uses 240 minutes by default and accepts `-minutes 0`. Tests generate synthetic credentials.
Product credential issuance, device Session binding, remote process supervision and UI are
not implemented by this command. OK acknowledges only internal TCP connection, not UART ready.

```powershell
go test ./internal/serialauth ./tests/poc/gostv3/registration -count=10 -timeout=60s
go vet ./internal/serialauth ./tests/poc/gostv3/registration
go build -o build/device-poc/registration.exe ./tests/poc/gostv3/registration
```

### Rebuild the tested minimal patch

Use a **writable unpacked copy** of official GOST tag v3.3.0 and go-gost/x v0.16.0,
not the module cache. Arrange sibling directories `gost-3.3.0` and `x` under an ignored
build directory. No product dependency is added. Source versions must remain pinned;
the patcher verifies every affected original byte sequence and refuses mismatches or
already-patched sources. It needs Python 3.9+ and Git; this is developer tooling only.

```powershell
$src = (Resolve-Path build/gost-patched-src).Path
python tests/poc/gostv3/apply_patches.py --source "$src/x"
Push-Location "$src/gost-3.3.0"
$env:GOTOOLCHAIN='go1.26.7'
go mod edit -replace github.com/go-gost/x=../x
# Preserve upstream licenses; these are patched PoC builds, not official binaries.
go build -trimpath -ldflags='-s -w' -o ../gost-patched.exe ./cmd/gost
$env:GOOS='linux'; $env:GOARCH='arm'; $env:GOARM='7'; $env:CGO_ENABLED='0'
go build -trimpath -ldflags='-s -w' -o ../gost-patched-arm ./cmd/gost
Remove-Item Env:GOOS,Env:GOARCH,Env:GOARM,Env:CGO_ENABLED
Pop-Location
Push-Location "$src/x"
go test ./limiter/conn/... ./internal/util/relay ./handler/forward/remote -timeout=60s
go test ./internal/net -run 'TestPipe|TestPOC' -count=3 -timeout=60s
Pop-Location
Remove-Item Env:GOTOOLCHAIN
```

The checked-in unified patch contains the minimal counter-alignment, Accept-loop and
UDP framing/buffer fixes **and regression tests**. `patches/LICENSE.go-gost-x` preserves
the MIT notice for the derived source hunks. This is not a complete dependency licensing
or redistribution audit. The unrelated upstream `TestTransport_ReadError` failed in an
initial full internal/net package run; this is preserved in the report, not deleted or
advertised as a full upstream test pass.

```powershell
python tests/poc/gostv3/windows_smoke.py --gost build/gost-patched-src/gost-patched.exe --registration build/device-poc/registration.exe --full-udp --output build/device-poc/windows-patched-new
python tests/poc/gostv3/auth_smoke.py --gost build/gost-patched-src/gost-patched.exe --output build/device-poc/relay-patched-new
```

`--full-udp` explicitly configures relay `udp.bufferSize=65535` and rudp
`readBufferSize=65535`; the patch preserves full datagrams through remote UDP Pipe.
TCP defaults are unchanged. This mode checks zero and large datagrams, concurrency and
next-frame recovery rather than silently truncating or removing LAN UDP requirements.
The original native Relay test remains a comparison, not the selected external protocol.

### Device continuation

The existing explicit SSH runner accepts `--full-udp` and `--registration <gateway.exe>`.
The gateway is a local **server-side PoC process**, never an external-client requirement.
It wraps the private reverse TCP mapping; the remote side uses isolated PTYs. Added checks
cover bad token/partial line/busy connections with zero PTY writes, stripped registration,
bidirectional data and next-owner reopening. No physical UART is opened.

The September 11 device evidence below used SSH **port20007**. The user changed it back
to **port20001** on September 12; see the diagnostic section below for current results. The device
registration branch has now actually run: run05 kept 5 failures; after the API metadata
and Linux serial-lifecycle fixes, run06 passed 44/45 checks. The remaining failure is the
new 8-source burst of 32000-byte UDP datagrams (6 received, 2 timed out). Do not label the
entire device suite passed or retry packets invisibly. Single-source 65000 bytes and zero
datagrams, serial registration, exclusivity and five rapid reopen cycles passed on ARM PTYs.

**API metadata distinction:** send `readBufferSize` as JSON string `"65535"`, not number
`65535`. The pinned upstream GetInt helper ignores the float64 produced by JSON map decoding;
CLI values are already strings. `--full-udp` now generates the correct API representation.

The patch also includes Linux serial cancellation: use SyscallConn.Control for ioctl and
retain the descriptor in Go's poller; File.Fd/SetNonblock(false) previously made idle Read
block Close. Flush preserves cancellability. Configured read timeout still uses the existing
100ms quantization and EOF-on-zero-byte-timeout behavior. Linux-only regression opens its own
PTY; it never touches a physical UART. The original driver fails Close-with-idle-reader;
the fix passed ten iterations on WSL and the actual ARM device.

```powershell
# In the patched x source, compile isolated Linux regression binaries.
$env:GOTOOLCHAIN='go1.26.7'; $env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
go test -c ./internal/util/serial -o ../serial-fixed-linux.test
$env:GOARCH='arm'; $env:GOARM='7'
go test -c ./internal/util/serial -o ../serial-fixed-arm.test
go vet ./internal/util/serial
Remove-Item Env:GOTOOLCHAIN,Env:GOOS,Env:GOARCH,Env:GOARM,Env:CGO_ENABLED
```

Run each binary only in its matching Linux environment with
`-test.run TestPOC -test.count 10 -test.timeout 25s -test.v`; the outer device watchdog was
30 seconds. Use a fresh isolated output directory for each device-smoke invocation. The
latest tested device binary is `build/gost-patched-src/gost-serial-fixed-arm`, stored remotely
as `/tmp/root/gost-poc-20260911-a6/gost`; do not confuse it with the earlier ARM binary.
All owned processes exited; only our a5/a6 upload intermediate archives were removed to
recover tmpfs space. Binaries, logs and the user's original archive remain. See the report
section4.5 for checks, hashes, memory and remaining real-UART/long-soak/UDP-burst limitations.


## UDP burst diagnosis (2026-09-12; current SSH port20001)

Use `udp_burst_smoke.py` with a fresh output directory. Remote mode requires the same
SSH password environment and AskPass as device_smoke, a fresh isolated
`/tmp/root/gost-poc-*` directory containing the already validated GOST executable, and
an ARM helper built from `./tests/poc/gostv3/devicehelper`. The runner checks live test
port occupancy (not closed TCP TIME_WAIT), uploads the helper, runs bounded fixtures,
and cleans up only owned processes. Device helpers self-exit after 12 minutes; GOST
has a 600-second watchdog. SSH loss is reported as unconfirmed cleanup until rechecked.

```powershell
# No device or SSH required; records Windows loopback evidence only.
python tests/poc/gostv3/udp_burst_smoke.py --local --gost build/gost-patched-src/gost-patched.exe --output build/device-poc/new-local-run --rounds 5 --late-window 12
```

Remote flags additionally require `--host admin@47.119.168.150 --port 20001`,
`--remote-dir`, `--helper` and `--askpass`; use a fresh directory, not the historical
runs. `--rounds` is bounded to 20, clients to 16, payload size to 12..65000 bytes.
`--late-window` defaults to 0 and is bounded to 12 seconds **after** the original
3-second failure. It keeps the same socket open and reports late_exact/late_seconds,
never retries or converts an overdue result into a pass. Exit 2 includes failed
3-second checks even when all replies eventually arrive. No RSS admission check is
applied, per the user's explicit instruction; lifecycle and queue limits remain.

Observed evidence: Windows udp-local-02 157/160 on time (not all passed); device a8
51/80 on time; device a9 with late observation 27/40 on time plus 13 exact late
replies, max 4.821s. All 40 reached and were echoed by the device fixture, socket drops
remained zero and every round recorded 8 inbound/8 outbound Relay trace events.
The SSH-carried test route is not a direct production Relay benchmark. Do not assume
all historic timeouts were late replies or claim Windows packet loss is resolved.
See GOST_V3_POC section4.6 for evidence, cleanup and test commands. The metrics fixture
retains only 128 synthetic 12-byte identifiers, not full payloads; its two Linux tests
passed ten iterations, and ARM build/vet passed.
