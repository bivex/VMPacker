#!/usr/bin/env bash
# rebuild-stub.sh — Rebuild the ARM64 VM interpreter stub binary
# Run this on macOS or Linux with Docker installed.
#
# Usage:
#   chmod +x rebuild-stub.sh
#   ./rebuild-stub.sh
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
echo "[*] Rebuilding stub binary using Docker..."
echo "[*] Repo: $REPO_ROOT"

docker run --rm \
    -v "$REPO_ROOT:/work" \
    -w /work \
    --platform linux/amd64 \
    ubuntu:22.04 \
    bash -c '
        set -euo pipefail
        echo "[*] Installing toolchain..."
        apt-get update -qq > /dev/null
        apt-get install -y -qq gcc-aarch64-linux-gnu binutils-aarch64-linux-gnu python3 make > /dev/null
        echo "[*] Building stub..."
        make stub
        echo "[+] Stub rebuilt: cmd/vmpacker/vm_interp.bin"
    '

echo "[+] Done! Now rebuild Go packer with: go build -o build/vmpacker ./cmd/vmpacker"
