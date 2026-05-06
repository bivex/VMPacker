#!/usr/bin/env bash
# docker-test.sh — Fast test using Docker + QEMU (no Android emulator)

set -euo pipefail

echo "[*] Building packer..."
make packer > /dev/null

echo "[*] Rebuilding stub (in Docker)..."
./rebuild-stub.sh > /dev/null

echo "[*] Protecting demo_complex..."
./build/vmpacker -func check_complex -o demo_complex_vmp_final11 demo/demo_complex

echo "[*] Running in Docker (Linux ARM64)..."
docker run --rm -v "/Volumes/External/Code/VMPacker:/work" --platform linux/arm64 ubuntu:22.04 /work/demo_complex_vmp_final11
echo "Exit code: $?"
