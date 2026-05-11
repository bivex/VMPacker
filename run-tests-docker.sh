#!/usr/bin/env bash
# run-tests-docker.sh — Run VMPacker tests in Docker

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "[*] Building test image..."
docker build -t vmpacker-test -f Dockerfile.test "$REPO_ROOT"

echo "[*] Running tests in container..."
docker run --rm -t \
    -v "$REPO_ROOT:/work" \
    -w /work \
    vmpacker-test \
    bash -c "go test ./pkg/vm ./pkg/arch/x86_64 -v && cd test/hybrid && make && ./hybrid_test_x86_64"

echo "[+] Test completed"
