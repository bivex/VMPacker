#!/usr/bin/env bash
# rebuild-stub.sh — Rebuild the ARM64 VM interpreter stub binary
# Uses pre-built vmpacker-build image for speed.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
echo "[*] Rebuilding stub binary using persistent Docker image..."

docker run --rm \
    -v "$REPO_ROOT:/work" \
    -w /work \
    vmpacker-build \
    bash -c "make clean && make stub EXTRA_CFLAGS='-O0'"

echo "[+] Stub rebuilt: cmd/vmpacker/vm_interp.bin"
echo "[+] Done!"
