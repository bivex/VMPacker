#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color
echo -e "${BLUE}[*] Phase 1: Building VMPacker toolchain...${NC}"
make packer

echo -e "${BLUE}[*] Phase 2: Ensuring Docker build environment...${NC}"
if [[ "$(docker images -q vmp-builder 2> /dev/null)" == "" ]]; then
    echo -e "${YELLOW}[!] vmp-builder image not found. Building...${NC}"
    docker build -t vmp-builder .
else
    echo -e "${GREEN}[+] Using existing vmp-builder image.${NC}"
fi

echo -e "${BLUE}[*] Phase 3: Compiling native ARM64 demo...${NC}"
# Use the pre-built image
docker run --rm -v "$(pwd)":/work vmp-builder make -C demo -f Makefile-demo demo_simple CROSS=

echo -e "${BLUE}[*] Phase 4: Protecting demo with VMPacker...${NC}"
./build/vmpacker -func check_simple -v -o demo/demo_simple.vmp demo/demo_simple


echo -e "${BLUE}[*] Phase 4: Running protected binary in Linux (Docker)...${NC}"
echo -e "${YELLOW}Expected Result: The program will print '7' and exit with code 37.${NC}"

# Run and show raw output
echo -e "${BLUE}--- Program Output Begin ---${NC}"
docker run --rm --platform linux/arm64 -v "$(pwd)/demo":/app -w /app vmp-builder bash -c "chmod +x ./demo_simple.vmp && ./demo_simple.vmp; EXIT_CODE=\$?; echo -e '\n--- Program Output End ---'; echo -e 'Exit Code (Result): '\$EXIT_CODE; if [ \$EXIT_CODE -eq 37 ]; then echo -e '\033[0;32m[+] SUCCESS: Calculation correct!\033[0m'; else echo -e '\033[0;31m[x] FAILED: Incorrect result\033[0m'; fi"
