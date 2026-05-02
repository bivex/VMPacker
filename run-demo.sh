#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}[*] Phase 1: Building VMPacker toolchain...${NC}"
make packer

echo -e "${BLUE}[*] Phase 2: Compiling native ARM64 demo...${NC}"
# Use Docker to ensure we have a proper ARM64 Linux toolchain
docker run --rm -v "$(pwd)":/work -w /work/demo debian:latest bash -c "apt-get update && apt-get install -y gcc make && make -f Makefile-demo demo_simple CROSS="

echo -e "${BLUE}[*] Phase 3: Protecting demo with VMPacker...${NC}"
./build/vmpacker -func check_simple -v -o demo/demo_simple.vmp demo/demo_simple

echo -e "${BLUE}[*] Phase 4: Running protected binary in Linux (Docker)...${NC}"
echo -e "${YELLOW}Expected Result: The program will print '7' and exit with code 37.${NC}"

# Run and capture exit code
docker run --rm --platform linux/arm64 -v "$(pwd)/demo":/app -w /app debian:latest bash -c "chmod +x ./demo_simple.vmp && ./demo_simple.vmp; EXIT_CODE=\$?; echo -e '\n--------------------------------'; echo -e 'Program Output Char: (none if empty)'; echo -e 'Exit Code (Result): '\$EXIT_CODE; if [ \$EXIT_CODE -eq 37 ]; then echo -e '\033[0;32m[+] SUCCESS: Calculation correct!\033[0m'; else echo -e '\033[0;31m[x] FAILED: Incorrect result\033[0m'; fi"
