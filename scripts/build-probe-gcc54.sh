#!/usr/bin/env bash
set -euo pipefail
umask 077
run=${1:?Usage: build-probe-gcc54.sh RUN ROOT}
root=${2:?Missing dedicated build root}
[[ "$root" =~ ^/root/[A-Za-z0-9_-]+$ && "$run" =~ ^$root/runs/[A-Za-z0-9_-]+$ ]] || exit 2
for cmd in cmake make python3 file readelf flock diff; do command -v "$cmd" >/dev/null; done
mkdir -p "$root/toolchain" "$run/output" "$run/tmp"
# Serialize SDK install and output publication for repeated double-clicks.
exec 9>"$root/build.lock"
flock 9
sdk="$root/toolchain/gcc-5.4"
if [[ ! -f "$sdk/.router-agent-installed" ]]; then
    [[ ! -e "$sdk" ]] || { echo 'SDK directory exists without installation marker; refusing to overwrite it.' >&2; exit 1; }
    mkdir "$run/sdk-stage"
    python3 - "$run/gcc-5.4.tar.gz" "$run/sdk-stage" <<'PY'
import sys, tarfile, posixpath
with tarfile.open(sys.argv[1], 'r:gz') as archive:
    for m in archive.getmembers():
        parts=m.name.split('/')
        if m.name.startswith('/') or '..' in parts or parts[0]!='gcc-5.4' or not (m.isfile() or m.isdir() or m.issym() or m.islnk()):
            raise SystemExit('Unsafe SDK archive member: '+m.name)
        if m.issym() or m.islnk():
            target=posixpath.normpath(posixpath.join(posixpath.dirname(m.name),m.linkname) if m.issym() else m.linkname)
            if m.linkname.startswith('/') or not (target=='gcc-5.4' or target.startswith('gcc-5.4/')):
                raise SystemExit('SDK link escapes installation directory: '+m.name)
    archive.extractall(sys.argv[2], filter='data')
PY
    compiler="$run/sdk-stage/gcc-5.4/bin/mipsel-openwrt-linux-uclibc-g++"
    [[ "$("$compiler" -dumpversion)" == 5.4* ]]
    [[ "$("$compiler" -dumpmachine)" == mipsel-openwrt-linux-uclibc ]]
    touch "$run/sdk-stage/gcc-5.4/.router-agent-installed"
    mv "$run/sdk-stage/gcc-5.4" "$sdk"
fi
compiler="$sdk/bin/mipsel-openwrt-linux-uclibc-g++"
export STAGING_DIR="$sdk" TMPDIR="$run/tmp"
"$compiler" --version
[[ "$("$compiler" -dumpversion)" == 5.4* ]]
[[ "$("$compiler" -dumpmachine)" == mipsel-openwrt-linux-uclibc ]]
mkdir "$run/src"
cp -a "$run/input/probe" "$run/src/probe"
# This SDK omits C99 C++ wrappers. Adapt only the per-run source copy.
python3 - "$run/src/probe" <<'PY'
import pathlib
import sys

probe = pathlib.Path(sys.argv[1])
header = probe / 'include/rmp/gcc54_compat.h'
if header.exists():
    raise SystemExit('gcc54_compat.h now exists in source; review the build adapter before overwriting it.')
header.write_text('''#ifndef RMP_GCC54_COMPAT_H
#define RMP_GCC54_COMPAT_H
#include <locale>
#include <sstream>
#include <string>
#include <type_traits>
namespace rmp {
template <typename Integer>
std::string Gcc54DecimalString(Integer value) {
    static_assert(std::is_integral<Integer>::value, "GCC 5.4 adapter supports integer to_string only");
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
        data = b'#include "rmp/gcc54_compat.h"' + newline + data.replace(b'std::to_string(', b'rmp::Gcc54DecimalString(')
    if b'std::strtoull(' in data:
        data = b'#include <stdlib.h>' + newline + data.replace(b'std::strtoull(', b'::strtoull(')
    if b'std::snprintf(' in data:
        data = b'#include <stdio.h>' + newline + data.replace(b'std::snprintf(', b'::snprintf(')
    if path.name == 'file_manager.cpp' and b'#include <stdio.h>' not in data:
        data = b'#include <stdio.h>' + newline + data
    if data != original:
        path.write_bytes(data)
        print('GCC 5.4 compatibility:', path.name)
PY
diff -ruN "$run/input/probe" "$run/src/probe" > "$run/gcc54-compat.patch" || [[ $? == 1 ]]
cat > "$run/toolchain.cmake" <<EOF
set(CMAKE_SYSTEM_NAME Linux)
set(CMAKE_SYSTEM_PROCESSOR mipsel)
set(CMAKE_CXX_COMPILER $compiler)
set(CMAKE_C_COMPILER $sdk/bin/mipsel-openwrt-linux-uclibc-gcc)
set(CMAKE_FIND_ROOT_PATH $sdk)
set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_PACKAGE ONLY)
set(CMAKE_EXE_LINKER_FLAGS_INIT "-Wl,-rpath-link,$sdk/lib")
EOF
cmake -S "$run/src/probe" -B "$run/build" -DCMAKE_TOOLCHAIN_FILE="$run/toolchain.cmake" -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTING=OFF
cmake --build "$run/build" --target router-probe --parallel 2
"$sdk/bin/mipsel-openwrt-linux-uclibc-strip" -o "$run/output/router-probe" "$run/build/router-probe"
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
mkdir -p "$root/output"
cp "$run/output/MBEDTLS-LICENSE.txt" "$run/output/THIRD-PARTY.md" "$root/output/"
cp "$run/output/router-probe" "$run/publish-router-probe"
mv -f "$run/publish-router-probe" "$root/output/router-probe"
printf '%s\n' "$run" > "$run/latest-build.txt"
mv -f "$run/latest-build.txt" "$root/latest-build.txt"
printf '\nSUCCESS: %s/output/router-probe\nBuild records: %s\n' "$root" "$run"
