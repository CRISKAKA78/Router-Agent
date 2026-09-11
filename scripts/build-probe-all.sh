#!/usr/bin/env bash
set -euo pipefail
umask 077
run=${1:?Usage: build-probe-all.sh RUN ARM_TOOLCHAIN ROOT INTERFACES}
toolchain=${2:?Missing ARM toolchain}
root=${3:?Missing workspace root}
interfaces=${4:-br0,eth0,eth1,usb0}
[[ "$root" =~ ^/root/[A-Za-z0-9_-]+$ && "$run" =~ ^$root/runs/[A-Za-z0-9_-]+$ ]] || exit 2
export LC_ALL=C
# One source snapshot and one lock for both builds and publication.
exec 9>"$root/build.lock"
flock 9
for arch in armv7 mipsel; do
    mkdir "$run/$arch"
    ln -s ../input "$run/$arch/input"
done
if [[ -f "$run/gcc-5.4.tar.gz" ]]; then
    ln -s ../gcc-5.4.tar.gz "$run/mipsel/gcc-5.4.tar.gz"
fi
printf '\n=== ARMv7 / GCC 5.2 ===\n'
bash "$run/input/scripts/build-probe-gcc52.sh" "$run/armv7" "$toolchain" "$root" "$interfaces" 2>&1 | tee "$run/armv7/build.log"
printf '\n=== MIPS little-endian / GCC 5.4 ===\n'
bash "$run/input/scripts/build-probe-gcc54.sh" "$run/mipsel" "$root" "$interfaces" 2>&1 | tee "$run/mipsel/build.log"
# Check actual ELF headers before changing either final executable.
python3 - "$run" <<'PY'
import pathlib, struct, sys
run = pathlib.Path(sys.argv[1])
for arch, machine in [('armv7', 40), ('mipsel', 8)]:
    path = run / arch / 'output' / ('router-agent-' + arch)
    with path.open('rb') as f:
        header = f.read(52)
    if len(header) != 52 or header[:6] != b'\x7fELF\x01\x01' or struct.unpack_from('<H', header, 18)[0] != machine:
        raise SystemExit('Unexpected ELF32 little-endian architecture: ' + str(path))
    print('Verified ELF32 little-endian ' + arch + ': ' + str(path.stat().st_size) + ' bytes')
PY
readelf -A "$run/armv7/output/router-agent-armv7" | grep -E 'Tag_CPU_arch: v7$'
# Both compilations and checks passed; replace each executable by rename, not streaming.
cp "$run/armv7/output/MBEDTLS-LICENSE.txt" "$run/armv7/output/THIRD-PARTY.md" "$root/"
for arch in armv7 mipsel; do
    cp "$run/$arch/output/router-agent-$arch" "$run/publish-$arch"
done
for arch in armv7 mipsel; do
    mv -f "$run/publish-$arch" "$root/router-agent-$arch"
done
printf '%s\n' "$run" > "$run/latest-build.txt"
mv -f "$run/latest-build.txt" "$root/latest-build.txt"
printf '\nSUCCESS: %s/router-agent-armv7\nSUCCESS: %s/router-agent-mipsel\nBuild records: %s\n' "$root" "$root" "$run"
