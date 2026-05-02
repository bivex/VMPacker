#!/usr/bin/env bash
# build_stub32_native.sh — 安装工具链并构建 vm_interp_arm32.bin (macOS 原生)
# 用法: ./scripts/build_stub32_native.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TOOLCHAIN_DIR="$SCRIPT_DIR/toolchains/armv7-unknown-linux-gnueabihf"
CROSS_ARM32="$TOOLCHAIN_DIR/bin/armv7-unknown-linux-gnueabihf-"

cd "$REPO_ROOT"

# 1. 安装工具链 (如未安装)
# 支持环境变量 LOCAL_TOOLCHAIN 指定本地已下载的 tar.gz 路径
if [ ! -f "$CROSS_ARM32"gcc ]; then
  echo "[*] Installing ARM32 toolchain ..."
  if [ -n "$LOCAL_TOOLCHAIN" ]; then
    ./scripts/install_arm32_toolchain.sh "$LOCAL_TOOLCHAIN"
  else
    ./scripts/install_arm32_toolchain.sh
  fi
else
  echo "[*] Using toolchain at $TOOLCHAIN_DIR"
fi

# 2. 构建 stub32
echo "[*] Building vm_interp_arm32.bin ..."
make stub32 CROSS_ARM32="$CROSS_ARM32"

# 3. 复制到 vmp-gui
cp -f cmd/vmpacker/vm_interp_arm32.bin vmp-gui/backend/api/vm_interp_arm32.bin
echo "[+] Done: vmp-gui/backend/api/vm_interp_arm32.bin ($(wc -c < vmp-gui/backend/api/vm_interp_arm32.bin) bytes)"
