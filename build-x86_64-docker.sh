#!/usr/bin/env bash
# build-x86_64-docker.sh — Build the x86_64 interpreter stub using Docker

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "[*] Building x86_64 build image..."
docker build -t vmpacker-build-x86_64 -f Dockerfile.x86_64 .

echo "[*] Building x86_64 stub binary in Docker..."
docker run --rm \
    -v "$REPO_ROOT:/work" \
    -w /work \
    vmpacker-build-x86_64 \
    bash -c "make stub64 CC64=gcc LD64=ld NM64=nm OBJCOPY64=objcopy"

echo "[+] x86_64 Stub built: cmd/vmpacker/vm_interp_x86_64.bin"
ls -l cmd/vmpacker/vm_interp_x86_64.bin
