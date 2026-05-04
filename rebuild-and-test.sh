#!/usr/bin/env bash
# rebuild-and-test.sh — Full rebuild + test on macOS with Docker
# Run this from the REPO ROOT on macOS:
#   chmod +x rebuild-and-test.sh
#   ./rebuild-and-test.sh
#
set -euo pipefail
REPO="$(cd "$(dirname "$0")" && pwd)"
cd "$REPO"

echo "=========================================="
echo " VMPacker: Rebuild Stub + Test"
echo "=========================================="

# 1. Rebuild stub with correct toolchain
echo ""
echo "[1] Building ARM64 stub via Docker (aarch64-linux-gnu-gcc)..."
docker run --rm \
    -v "$REPO:/work" -w /work \
    --platform linux/amd64 \
    ubuntu:22.04 bash -c '
        apt-get update -qq > /dev/null
        apt-get install -y -qq gcc-aarch64-linux-gnu binutils-aarch64-linux-gnu python3 make > /dev/null
        make stub
        echo "[+] Stub OK: $(wc -c < cmd/vmpacker/vm_interp.bin) bytes"
    '

# 2. Build Go packer
echo ""
echo "[2] Building Go packer..."
go build -o build/vmpacker ./cmd/vmpacker
echo "[+] packer: build/vmpacker"

# 3. Run tests
echo ""
echo "[3] Running tests..."

# Test 1: Simple ALU (check_simple(10) should return 37 = 10*3+7)
echo "--- Test 1: Simple ALU (expect 37) ---"
./build/vmpacker -func check_simple -o /tmp/vmp_simple.elf demo/demo_simple 2>/dev/null
RES=$(docker run --rm -v "$REPO:/work" -w /work \
    --platform linux/arm64 ubuntu:22.04 \
    bash -c '/work/tmp/vmp_simple.elf 2>/dev/null; echo "Exit: $?"' 2>/dev/null || \
    docker run --rm -v "/tmp:/tmp" --platform linux/arm64 ubuntu:22.04 \
    bash -c '/tmp/vmp_simple.elf 2>/dev/null; echo "Exit: $?"')
echo "$RES"

# Test 2: Two different ISAs (run packer twice = different random opcodes)
echo "--- Test 2: Dual ISA (both should return 37) ---"
./build/vmpacker -func check_simple -o /tmp/vmp_isa1.elf demo/demo_simple 2>/dev/null
./build/vmpacker -func check_simple -o /tmp/vmp_isa2.elf demo/demo_simple 2>/dev/null
docker run --rm -v "/tmp:/tmp" --platform linux/arm64 ubuntu:22.04 bash -c '
    echo -n "ISA1: "; /tmp/vmp_isa1.elf; echo " (exit $?)"
    echo -n "ISA2: "; /tmp/vmp_isa2.elf; echo " (exit $?)"
'

echo ""
echo "[+] All done!"
