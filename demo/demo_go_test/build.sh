#!/bin/bash
# Cross-compile Go program to ARM64 Linux ELF (with symbol table)
# 
# Usage: bash demo/demo_go_test/build.sh
#
# Note: 
#   -ldflags="-compressdwarf=false" Keep full debug information
#   CGO_ENABLED=0 Ensure pure Go compilation, no C dependencies
#   -gcflags="-N -l" Disable optimization and inlining to ensure functions are not optimized away

set -e

cd "$(dirname "$0")"

echo "[*] Building Go ARM64 ELF..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -gcflags="-N -l" \
    -o ../../build/demo_go_test \
    .

echo "[+] Output: build/demo_go_test"
echo ""
echo "[*] Checking symbols..."
# Use Go's objdump or nm to view symbols
go tool nm ../../build/demo_go_test 2>/dev/null | grep -i "checkKey" || echo "(use 'go tool nm' or 'aarch64-linux-gnu-nm' to inspect)"
echo ""
echo "[*] Done. Try:"
echo "  ./build/vmpacker.exe -info build/demo_go_test"
echo "  ./build/vmpacker.exe -func main.checkKey -v -debug build/demo_go_test"
