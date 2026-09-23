#!/usr/bin/env bash
set -euo pipefail

BRIDGE="br0"
TAP="hyve-tap0"

IMAGE="${1:-.build/qga/debian-qga.qcow2}"

if [[ $EUID -ne 0 ]]; then
    echo "ERROR: run as root"
    exit 1
fi

if [[ ! -f "$IMAGE" ]]; then
    echo "ERROR: disk image not found: $IMAGE"
    exit 1
fi

if ! ip link show "$BRIDGE" >/dev/null 2>&1; then
    echo "ERROR: bridge $BRIDGE does not exist"
    echo "Run:"
    echo "  sudo ./testdata/host/network-test.sh setup"
    exit 1
fi

if ! ip link show "$TAP" >/dev/null 2>&1; then
    echo "ERROR: TAP $TAP does not exist"
    exit 1
fi

echo "Starting QEMU"
echo "  Bridge: $BRIDGE"
echo "  TAP:    $TAP"
echo "  Image:  $IMAGE"

exec qemu-system-x86_64 \
    -enable-kvm \
    -m 512M \
    -smp 1 \
    -drive "file=$IMAGE,format=qcow2,if=virtio" \
    -netdev tap,id=net0,ifname="$TAP",script=no,downscript=no \
    -device virtio-net-pci,netdev=net0 \
    -nographic