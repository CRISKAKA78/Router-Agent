#!/usr/bin/env bash
set -euo pipefail
umask 077

run=${1:?Usage: build-probe-gcc52.sh RUN TOOLCHAIN ROOT}
toolchain=${2:?Missing toolchain directory}
root=${3:?Missing workspace root}
interfaces=${4:--}
[[ "$interfaces" == "-" ]] && interfaces=""
[[ "$interfaces" =~ ^[A-Za-z0-9_,.-]*$ ]] || exit 2
[[ "$root" =~ ^/root/[A-Za-z0-9_-]+$ && "$run" == "$root"/runs/* ]] || exit 2
[[ "$toolchain" =~ ^/root/[A-Za-z0-9_.-]+$ ]] || exit 2
export TMPDIR="$run/tmp"
export STAGING_DIR="$toolchain"
mkdir -p "$TMPDIR" "$run/src" "$run/output"
compiler="$toolchain/bin/arm-openwrt-linux-uclibcgnueabi-g++"
strip="$toolchain/bin/arm-openwrt-linux-uclibcgnueabi-strip"
for command in cmake make python3 diff file readelf; do command -v "$command" >/dev/null; done
if [[ ! -x "$compiler" || ! -x "$strip" ]]; then
    printf 'Missing GCC 5.2 ARM compiler or strip tool under %s/bin\n' "$toolchain" >&2
    exit 1
fi
"$compiler" --version
[[ "$("$compiler" -dumpversion)" == 5.2* ]]
cp -a "$run/input/probe" "$run/src/probe"

# This SDK omits C99 C++ wrappers. Adapt only the per-run source copy.
python3 - "$run/src/probe" <<'PY'
import pathlib
import sys

probe = pathlib.Path(sys.argv[1])
header = probe / 'include/rmp/gcc52_compat.h'
if header.exists():
    raise SystemExit('gcc52_compat.h now exists in source; review the build adapter before overwriting it.')
header.write_text('''#ifndef RMP_GCC52_COMPAT_H
#define RMP_GCC52_COMPAT_H
#include <locale>
#include <sstream>
#include <string>
#include <type_traits>
namespace rmp {
template <typename Integer>
std::string Gcc52DecimalString(Integer value) {
    static_assert(std::is_integral<Integer>::value, "GCC 5.2 adapter supports integer to_string only");
    std::ostringstream stream;
    stream.imbue(std::locale::classic());
    stream << value;
    return stream.str();
}
}
#endif
''', encoding='utf-8')
for path in sorted((probe / 'src').glob('*.cpp')):
    original = path.read_bytes()
    data = original
    newline = b'\r\n' if b'\r\n' in data else b'\n'
    if b'std::to_string(' in data:
        data = b'#include "rmp/gcc52_compat.h"' + newline + data.replace(b'std::to_string(', b'rmp::Gcc52DecimalString(')
    if b'std::strtoull(' in data:
        data = b'#include <stdlib.h>' + newline + data.replace(b'std::strtoull(', b'::strtoull(')
    if b'std::snprintf(' in data:
        data = b'#include <stdio.h>' + newline + data.replace(b'std::snprintf(', b'::snprintf(')
    if path.name == 'file_manager.cpp' and b'#include <stdio.h>' not in data:
        data = b'#include <stdio.h>' + newline + data
    if data != original:
        path.write_bytes(data)
        print('GCC 5.2 compatibility:', path.name)
PY
diff -ruN "$run/input/probe" "$run/src/probe" > "$run/gcc52-compat.patch" || [[ $? == 1 ]]
cat > "$run/toolchain.cmake" <<EOF
set(CMAKE_SYSTEM_NAME Linux)
set(CMAKE_SYSTEM_PROCESSOR arm)
set(CMAKE_CXX_COMPILER $compiler)
set(CMAKE_C_COMPILER $toolchain/bin/arm-openwrt-linux-uclibcgnueabi-gcc)
set(CMAKE_FIND_ROOT_PATH $toolchain)
set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_PACKAGE ONLY)
set(CMAKE_EXE_LINKER_FLAGS_INIT "-Wl,-rpath-link,$toolchain/lib")
EOF
cmake -S "$run/src/probe" -B "$run/build" \
    -DCMAKE_TOOLCHAIN_FILE="$run/toolchain.cmake" \
    -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTING=OFF -DRMP_NETWORK_INTERFACES="$interfaces"
cmake --build "$run/build" --parallel 2
"$strip" -o "$run/output/router-probe" "$run/build/router-probe"
cp "$run/input/probe/third_party/mbedtls/LICENSE" "$run/output/MBEDTLS-LICENSE.txt"
cp "$run/input/probe/third_party/mbedtls/README.router-agent.md" "$run/output/THIRD-PARTY.md"
{
    file "$run/output/router-probe"
    stat -c 'size=%s bytes' "$run/output/router-probe"
    readelf -h "$run/output/router-probe"
    readelf -A "$run/output/router-probe"
    readelf -l "$run/output/router-probe" | grep interpreter
    readelf -d "$run/output/router-probe" | grep -E 'NEEDED|RPATH|RUNPATH'
} | tee "$run/verification.log"

# Preserve the last successful executable if upload, configuration or compilation fails.
mkdir -p "$root/output"
cp "$run/output/MBEDTLS-LICENSE.txt" "$run/output/THIRD-PARTY.md" "$root/output/"
cp "$run/output/router-probe" "$run/publish-router-probe"
mv -f "$run/publish-router-probe" "$root/output/router-probe"
printf '%s\n' "$run" > "$run/latest-build.txt"
mv -f "$run/latest-build.txt" "$root/latest-build.txt"
printf '\nSUCCESS: %s/output/router-probe\nBuild records: %s\n' "$root" "$run"
