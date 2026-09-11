#!/bin/sh
# Isolates every network/route/process change; requires no production endpoint.
set -eu
: "${RMP_EASYTIER_TEST_BIN:?Set the absolute official v2.6.4 binary directory}"
: "${RMP_EASYTIER_TEST_DB:?Set a database from overlay-official-seed.py}"
exec unshare -mnpf --mount-proc sh -c '
set -eu
mount --make-rprivate /
ip link set lo up
ip link add et-test type dummy
ip addr add 192.0.2.1/24 dev et-test
ip link set et-test up
ip route add default dev et-test
mount -t sysfs sysfs /sys
mount -t devpts devpts /dev/pts -o newinstance,ptmxmode=0666,mode=0620
go test ./internal/overlay -run TestOfficialEasyTierRuntimeConfig -count=1 -v
'
