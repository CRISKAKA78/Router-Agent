#!/bin/sh
# Exercise a script emitted by Desktop.Tests --directory-shell-command /fixture.
# Runs with Linux /bin/sh and existing utilities; no extra runtime or device access.
set -eu
command_file=$1
root=$(mktemp -d /tmp/router-directory-test.XXXXXX)
trap 'rm -rf "$root"' EXIT HUP INT TERM
mkdir "$root/files" "$root/bin"
ln -s "$(command -v ls)" "$root/bin/ls"
sed "s|cd '/fixture'|cd '$root/files'|" "$command_file" > "$root/command.sh"
run() {
 set +e
 PATH="$root/bin" /bin/sh "$root/command.sh" > "$root/out" 2> "$root/err"
 code=$?
 set -e
}
check() { test "$1" = "$2" || { echo "FAIL: $3 ($1 != $2)"; exit 1; }; }
passed() { echo "PASS $1"; }

run; check "$code" 0 empty; test ! -s "$root/out"; passed empty
printf hello > "$root/files/normal"
touch -t 202311142213.20 "$root/files/normal"
ln -s "$(command -v stat)" "$root/bin/stat"
run; check "$code" 0 stat
meta=$(stat -c '%s %Y' "$root/files/normal"); set -- $meta
printf 'f\000%s\000%s\000normal\000' "$1" "$2" > "$root/expected"
cmp "$root/out" "$root/expected"; passed stat_metadata
rm "$root/bin/stat" "$root/files/normal"

for name in 'space name' "quote'name" 'line
name' '-option' '.hidden' '..double' '目录' 'glob*?[]' 'LIMIT'; do
 printf hello > "$root/files/$name"
 run; check "$code" 0 no_stat
 printf 'f\0005\000\000%s\000' "$name" > "$root/expected"
 cmp "$root/out" "$root/expected"
 rm "$root/files/$name"
done
passed no_stat_preserves_unusual_names_and_size

ln -s absent "$root/files/dangling"
run; check "$code" 0 symlink
printf 'f\0006\000\000dangling\000' > "$root/expected"
cmp "$root/out" "$root/expected"
rm "$root/files/dangling"
mkfifo "$root/files/fifo"
# timeout ensures accidental reading of a FIFO cannot hang the test run.
set +e
timeout 5 env PATH="$root/bin" /bin/sh "$root/command.sh" > "$root/out" 2> "$root/err"
code=$?
set -e
check "$code" 0 fifo
printf 'f\0000\000\000fifo\000' > "$root/expected"
cmp "$root/out" "$root/expected"
rm "$root/files/fifo"
mkdir "$root/files/folder"
run; check "$code" 0 directory
check "$(tr '\000' '\n' < "$root/out" | head -n 1)" d directory_kind
rmdir "$root/files/folder"
passed metadata_only_symlink_fifo_directory

i=0; while [ "$i" -lt 251 ]; do : > "$root/files/$i"; i=$((i+1)); done
run; check "$code" 0 cap
check "$(tr '\000' '\n' < "$root/out" | wc -l | tr -d ' ')" 1001 cap_fields
check "$(tr '\000' '\n' < "$root/out" | tail -n 1)" LIMIT cap_marker
passed cap_250

printf '#!/bin/sh\nexit 1\n' > "$root/bin/stat"; chmod +x "$root/bin/stat"
run; check "$code" 2 stat_failure
grep -q 'Cannot stat directory entry' "$root/err"
rm "$root/bin/stat"
passed stat_failure_explained

rm "$root/bin/ls"
run; check "$code" 2 ls_failure
grep -q 'Cannot list directory entry' "$root/err"
passed ls_failure_explained

mv "$root/files" "$root/moved"
run; check "$code" 1 missing_directory
grep -q 'Cannot open directory' "$root/err"
passed missing_directory_explained
