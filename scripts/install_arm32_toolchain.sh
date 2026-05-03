#!/usr/bin/env bash
# install_arm32_toolchain.sh — Install messense pre-compiled ARM32 toolchain
# Usage: ./scripts/install_arm32_toolchain.sh [local tar.gz path]
#       If a local path is provided, skip download and extract directly to install
# Install the toolchain to scripts/toolchains/armv7-unknown-linux-gnueabihf/

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLCHAIN_DIR="$SCRIPT_DIR/toolchains/armv7-unknown-linux-gnueabihf"
RELEASE_URL="https://github.com/messense/homebrew-macos-cross-toolchains/releases/download/v15.2.0"
LOCAL_TARBALL="${1:-}"

case "$(uname -m)" in
  arm64|aarch64)
    TARBALL_NAME="armv7-unknown-linux-gnueabihf-aarch64-darwin.tar.gz"
    SHA256="afaee575236de63ee492e2778afded2867f2cd014d0c41b27cb61828d2742df0"
    ;;
  x86_64)
    TARBALL_NAME="armv7-unknown-linux-gnueabihf-x86_64-darwin.tar.gz"
    SHA256="a8051c569c4fce90960a20ed057c6e7a1903df7614a81b797a1c05dbb146fab5"
    ;;
  *)
    echo "[!] Unsupported architecture: $(uname -m)"
    exit 1
    ;;
esac

mkdir -p "$(dirname "$TOOLCHAIN_DIR")"
cd "$(dirname "$TOOLCHAIN_DIR")"

if [ -f "$TOOLCHAIN_DIR/bin/armv7-unknown-linux-gnueabihf-gcc" ]; then
  echo "[*] Toolchain already installed at $TOOLCHAIN_DIR"
  exit 0
fi

if [ -n "$LOCAL_TARBALL" ]; then
  if [ ! -f "$LOCAL_TARBALL" ]; then
    echo "[!] Local file not found: $LOCAL_TARBALL"
    exit 1
  fi
  echo "[*] Using local tarball: $LOCAL_TARBALL"
  TARBALL="$LOCAL_TARBALL"
  SKIP_DOWNLOAD=1
else
  TARBALL="$TARBALL_NAME"
  SKIP_DOWNLOAD=0
fi

if [ "$SKIP_DOWNLOAD" = 0 ]; then
  echo "[*] Downloading $TARBALL_NAME ..."
  if command -v curl >/dev/null 2>&1; then
    curl -sL -o "$TARBALL_NAME" "$RELEASE_URL/$TARBALL_NAME"
  else
    wget -q -O "$TARBALL_NAME" "$RELEASE_URL/$TARBALL_NAME"
  fi
  TARBALL="$TARBALL_NAME"
fi

if [ -n "$LOCAL_TARBALL" ]; then
  cp "$LOCAL_TARBALL" "./$TARBALL_NAME"
  TARBALL="./$TARBALL_NAME"
fi

echo "[*] Verifying checksum ..."
if command -v shasum >/dev/null 2>&1; then
  echo "$SHA256  $TARBALL" | shasum -a 256 -c -
elif command -v sha256sum >/dev/null 2>&1; then
  echo "$SHA256  $TARBALL" | sha256sum -c -
else
  echo "[!] Warning: could not verify checksum (shasum/sha256sum not found)"
fi

echo "[*] Extracting ..."
rm -rf .toolchain_extract
mkdir -p .toolchain_extract
tar -xzf "$TARBALL" -C .toolchain_extract
rm -f "$TARBALL_NAME" "$TARBALL" 2>/dev/null || true

# Tarball extracts to a single top-level dir (e.g. armv7-unknown-linux-gnueabihf-aarch64-darwin)
TOPDIR=$(find .toolchain_extract -maxdepth 1 -mindepth 1 -type d | head -1)
if [ -z "$TOPDIR" ]; then
  echo "[!] Unknown tarball structure"
  rm -rf .toolchain_extract
  exit 1
fi
mv "$TOPDIR" "$TOOLCHAIN_DIR"
rmdir .toolchain_extract 2>/dev/null || rm -rf .toolchain_extract

echo "[+] Toolchain installed at $TOOLCHAIN_DIR"
echo "    Use: make stub32 CROSS_ARM32=$TOOLCHAIN_DIR/bin/armv7-unknown-linux-gnueabihf-"
