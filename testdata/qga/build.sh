#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"

BUILD_DIR="${PROJECT_ROOT}/.build/qga"
IMAGE="${BUILD_DIR}/debian-qga.qcow2"
CLOUD_INIT_DIR="${SCRIPT_DIR}/cloud-init"
SEED_ISO="${BUILD_DIR}/seed.iso"
BASE_IMAGE="${BUILD_DIR}/debian-base.qcow2"

# Debian 13 (Trixie) generic cloud image.
# Keep this explicit so CI/local builds are reproducible.
DEBIAN_VERSION="13"
DEBIAN_IMAGE_URL="https://cloud.debian.org/images/cloud/trixie/latest/debian-13-genericcloud-amd64.qcow2"

die() {
    echo "error: $*" >&2
    exit 1
}

require_command() {
    command -v "$1" >/dev/null 2>&1 ||
        die "required command not found: $1"
}

cleanup() {
    rm -f "${SEED_ISO}"
}

trap cleanup EXIT

require_command curl
require_command qemu-img
require_command qemu-system-x86_64
require_command xorriso

mkdir -p "${BUILD_DIR}"

echo "==> HYVE QGA integration-test image"
echo
echo "    Debian: ${DEBIAN_VERSION}"
echo "    Output: ${IMAGE}"
echo

# ---------------------------------------------------------------------------
# 1. Download Debian cloud image
# ---------------------------------------------------------------------------

if [[ ! -f "${BASE_IMAGE}" ]]; then
    echo "==> Downloading Debian cloud image..."

    curl \
        --fail \
        --location \
        --show-error \
        --progress-bar \
        --output "${BASE_IMAGE}.download" \
        "${DEBIAN_IMAGE_URL}"

    mv "${BASE_IMAGE}.download" "${BASE_IMAGE}"
else
    echo "==> Using cached Debian base image"
fi

# ---------------------------------------------------------------------------
# 2. Create writable test image
# ---------------------------------------------------------------------------

echo "==> Creating test image..."

rm -f "${IMAGE}"

qemu-img create \
    -f qcow2 \
    -F qcow2 \
    -b "${BASE_IMAGE}" \
    "${IMAGE}" \
    8G >/dev/null

# ---------------------------------------------------------------------------
# 3. Create NoCloud seed ISO
# ---------------------------------------------------------------------------

echo "==> Creating cloud-init seed ISO..."

[[ -f "${CLOUD_INIT_DIR}/user-data" ]] ||
    die "missing ${CLOUD_INIT_DIR}/user-data"

cat > "${BUILD_DIR}/meta-data" <<EOF
instance-id: hyve-qga-test
local-hostname: hyve-qga-test
EOF

cp "${CLOUD_INIT_DIR}/user-data" "${BUILD_DIR}/user-data"

xorriso \
    -as mkisofs \
    -output "${SEED_ISO}" \
    -volid CIDATA \
    -joliet \
    -rock \
    "${BUILD_DIR}/user-data" \
    "${BUILD_DIR}/meta-data" \
    >/dev/null 2>&1

rm -f \
    "${BUILD_DIR}/user-data" \
    "${BUILD_DIR}/meta-data"

# ---------------------------------------------------------------------------
# 4. Boot image once so cloud-init installs QGA
# ---------------------------------------------------------------------------

echo "==> Booting Debian once to provision qemu-guest-agent..."

TMP_SOCKET="${BUILD_DIR}/qga-provision.sock"
rm -f "${TMP_SOCKET}"

qemu-system-x86_64 \
    -machine accel=kvm:tcg \
    -cpu max \
    -m 512M \
    -smp 1 \
    -drive "file=${IMAGE},if=virtio,format=qcow2" \
    -drive "file=${SEED_ISO},media=cdrom,readonly=on" \
    -nic user,model=virtio-net-pci \
    -nographic \
    -serial none \
    -monitor "unix:${TMP_SOCKET},server,nowait" \
    >/dev/null 2>&1 &

QEMU_PID=$!

cleanup_qemu() {
    if kill -0 "${QEMU_PID}" 2>/dev/null; then
        kill "${QEMU_PID}" 2>/dev/null || true
        wait "${QEMU_PID}" 2>/dev/null || true
    fi

    rm -f "${TMP_SOCKET}"
}

trap cleanup_qemu EXIT

# Give cloud-init enough time to finish.
#
# We intentionally don't depend on SSH or networking here. The provisioning
# image contains a systemd service which creates the QGA virtio channel.
echo "==> Waiting for cloud-init provisioning..."

if wait "${QEMU_PID}"; then
    echo "==> Provisioning VM shut down successfully"
else
    status=$?

    # QEMU returns non-zero for some normal shutdown paths.
    echo "==> Provisioning QEMU exited with status ${status}"
fi

rm -f "${TMP_SOCKET}"

trap cleanup EXIT

# ---------------------------------------------------------------------------
# 5. Final image cleanup
# ---------------------------------------------------------------------------

echo "==> Compacting image..."

qemu-img convert \
    -O qcow2 \
    -c \
    "${IMAGE}" \
    "${IMAGE}.tmp"
    
mv "${IMAGE}.tmp" "${IMAGE}"

echo
echo "==> QGA test image ready:"
echo
echo "    ${IMAGE}"
echo
echo "Next:"
echo
echo "    go test ./internal/qemu -run TestGuestPing -v"
echo