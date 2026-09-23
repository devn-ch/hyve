#!/usr/bin/env bash
set -euo pipefail

BRIDGE="br0"
TAP="hyve-tap0"

setup() {
    echo "== HYVE network setup =="

    if [[ $EUID -ne 0 ]]; then
        echo "ERROR: run as root"
        exit 1
    fi

    if ! ip link show "$BRIDGE" >/dev/null 2>&1; then
        echo "Creating bridge: $BRIDGE"
        ip link add name "$BRIDGE" type bridge
    else
        echo "Bridge already exists: $BRIDGE"
    fi

    ip link set "$BRIDGE" up

    if ! ip link show "$TAP" >/dev/null 2>&1; then
        echo "Creating TAP: $TAP"
        ip tuntap add dev "$TAP" mode tap
    else
        echo "TAP already exists: $TAP"
    fi

    ip link set "$TAP" master "$BRIDGE"
    ip link set "$TAP" up

    echo
    echo "Bridge:"
    ip -br link show "$BRIDGE"

    echo
    echo "TAP:"
    ip -br link show "$TAP"

    echo
    echo "Bridge members:"
    bridge link show master "$BRIDGE"

    echo
    echo "Network setup complete."
}

cleanup() {
    echo "== HYVE network cleanup =="

    if [[ $EUID -ne 0 ]]; then
        echo "ERROR: run as root"
        exit 1
    fi

    if ip link show "$TAP" >/dev/null 2>&1; then
        echo "Removing TAP: $TAP"
        ip link del "$TAP"
    fi

    if ip link show "$BRIDGE" >/dev/null 2>&1; then
        echo "Removing bridge: $BRIDGE"
        ip link del "$BRIDGE"
    fi

    echo "Cleanup complete."
}

case "${1:-}" in
    setup)
        setup
        ;;
    cleanup)
        cleanup
        ;;
    *)
        echo "Usage: $0 {setup|cleanup}"
        exit 1
        ;;
esac