#!/bin/sh
# Run in an isolated Linux network namespace with loopback, devpts, OpenSSH and
# busybox-extras (the Phase 4 real-service tests bind TCP 80/22/23).
set -eu
mkdir -p build/phase5-go/cache build/phase5-go/tmp build/phase5-logs build/server
export GOCACHE="$PWD/build/phase5-go/cache"
export GOTMPDIR="$PWD/build/phase5-go/tmp"
case "${1:-release}" in
release)
  cmake -S probe -B build/phase5-probe -DCMAKE_BUILD_TYPE=Release
  cmake --build build/phase5-probe --parallel 2
  ctest --test-dir build/phase5-probe --output-on-failure
  RMP_PROBE_BIN="$PWD/build/phase5-probe/router-probe" go test ./cmd/... ./internal/... ./tests/... -count=1
  go vet ./cmd/... ./internal/... ./tests/...
  go build -o build/server/router-server-phase5-linux ./cmd/server
  ;;
asan)
  cmake -S probe -B build/phase5-asan -DCMAKE_BUILD_TYPE=Debug -DCMAKE_CXX_FLAGS=-fsanitize=address,undefined -DCMAKE_EXE_LINKER_FLAGS=-fsanitize=address,undefined
  cmake --build build/phase5-asan --parallel 2
  ASAN_OPTIONS=detect_leaks=1 ctest --test-dir build/phase5-asan --output-on-failure
  ASAN_OPTIONS=detect_leaks=1 RMP_PROBE_BIN="$PWD/build/phase5-asan/router-probe" go test ./tests/integration -run 'TestTunnel|TestPhase5' -count=1
  ;;
race)
  cmake -S probe -B build/phase5-tsan -DCMAKE_BUILD_TYPE=Debug -DCMAKE_CXX_FLAGS=-fsanitize=thread -DCMAKE_EXE_LINKER_FLAGS=-fsanitize=thread
  cmake --build build/phase5-tsan --parallel 2
  TSAN_OPTIONS=halt_on_error=1 ctest --test-dir build/phase5-tsan --output-on-failure
  TSAN_OPTIONS=halt_on_error=1 RMP_PROBE_BIN="$PWD/build/phase5-tsan/router-probe" go test -race ./cmd/... ./internal/... ./tests/... -count=1
  ;;
*) echo 'usage: verify-phase5.sh release|asan|race' >&2; exit 2 ;;
esac
