#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}[*] Building ARM64 VM interpreter stub...${NC}"
# Attempt native build if on ARM64, else try cross-compiler or Docker
if [[ $(uname -m) == "arm64" && $(uname -s) == "Darwin" ]]; then
    # On macOS ARM64, we still need a Linux toolchain for the ELF stub.
    # Check if aarch64-linux-gnu-gcc is available
    if command -v aarch64-linux-gnu-gcc &> /dev/null; then
        make stub
    elif command -v docker &> /dev/null; then
        echo -e "${BLUE}[*] Ensuring Docker build environment...${NC}"
        if [[ "$(docker images -q vmp-builder 2> /dev/null)" == "" ]]; then
            docker build -t vmp-builder .
        fi
        echo -e "${BLUE}[*] Using Docker to build Linux ARM64 stub...${NC}"
        docker run --rm -v "$(pwd)":/work vmp-builder make stub CROSS=
    else
        echo -e "\033[0;31m[!] Error: No cross-compiler or Docker found. Cannot build Linux ARM64 stub.${NC}"
        exit 1
    fi
else
    make stub
fi

echo -e "${BLUE}[*] Building and running GUI...${NC}"
# Check if Wails is installed
if ! command -v wails &> /dev/null; then
    echo -e "${BLUE}[*] Wails CLI not found. Installing...${NC}"
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    export PATH=$PATH:$(go env GOPATH)/bin
fi

# Build and run
make gui

OS="$(uname)"
case "$OS" in
    "Darwin")
        echo -e "${GREEN}[+] Opening vmp-gui.app...${NC}"
        open vmp-gui/build/bin/vmp-gui.app
        ;;
    "Linux")
        echo -e "${GREEN}[+] Running vmp-gui...${NC}"
        ./vmp-gui/build/bin/vmp-gui
        ;;
    *)
        echo -e "${GREEN}[+] GUI built successfully. Find it in vmp-gui/build/bin/${NC}"
        ;;
esac
