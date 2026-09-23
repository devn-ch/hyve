#!/usr/bin/env bash

set -euo pipefail

CONFIG_FILE="/etc/hyve/hyve.conf"

BRIDGE="br0"
INTERFACE=""
METHOD="dhcp"

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "ERROR: Missing $CONFIG_FILE" >&2
    exit 1
fi

section=""

while IFS='=' read -r raw_key raw_value; do
    key="$(echo "${raw_key:-}" | xargs)"
    value="$(echo "${raw_value:-}" | xargs)"

    [[ -z "$key" ]] && continue
    [[ "$key" == \#* ]] && continue
    [[ "$key" == \;* ]] && continue

    if [[ "$key" == \[*\] ]]; then
        section="${key#[}"
        section="${section%]}"
        continue
    fi

    [[ "$section" != "network" ]] && continue

    case "$key" in
        bridge)
            BRIDGE="$value"
            ;;
        interface)
            INTERFACE="$value"
            ;;
        method)
            METHOD="$value"
            ;;
    esac
done < "$CONFIG_FILE"

if [[ -z "$INTERFACE" ]]; then
    echo "ERROR: network.interface is not configured" >&2
    exit 1
fi

if ! command -v ip >/dev/null 2>&1; then
    echo "ERROR: ip command not found" >&2
    exit 1
fi

if ! ip link show "$INTERFACE" >/dev/null 2>&1; then
    echo "ERROR: Interface '$INTERFACE' does not exist" >&2
    exit 1
fi

echo "Configuring HYVE network"
echo "  Interface: $INTERFACE"
echo "  Bridge:    $BRIDGE"
echo "  Method:    $METHOD"

# Create bridge if it does not exist.
if ! ip link show "$BRIDGE" >/dev/null 2>&1; then
    echo "Creating bridge $BRIDGE"
    ip link add name "$BRIDGE" type bridge
fi

# Attach physical interface to bridge if necessary.
current_master="$(readlink -f "/sys/class/net/$INTERFACE/master" 2>/dev/null || true)"

if [[ "$current_master" != "/sys/class/net/$BRIDGE" ]]; then
    echo "Attaching $INTERFACE to $BRIDGE"
    ip link set "$INTERFACE" master "$BRIDGE"
fi

# The physical interface must not hold the host IP.
ip addr flush dev "$INTERFACE"

ip link set "$INTERFACE" up
ip link set "$BRIDGE" up

case "$METHOD" in
    dhcp)
        if ! command -v dhclient >/dev/null 2>&1; then
            echo "ERROR: dhclient not found." >&2
            echo "Install it with: apt install isc-dhcp-client" >&2
            exit 1
        fi

        echo "Requesting DHCP lease on $BRIDGE"

        mkdir -p /run/hyve

        dhclient \
            -pf /run/hyve/dhclient.pid \
            -lf /var/lib/dhcp/dhclient."$BRIDGE".leases \
            "$BRIDGE"
        ;;

    static)
        echo "ERROR: Static networking is not implemented yet" >&2
        exit 1
        ;;

    *)
        echo "ERROR: Unsupported network method '$METHOD'" >&2
        exit 1
        ;;
esac

echo "HYVE network ready"
ip -brief address show "$BRIDGE"