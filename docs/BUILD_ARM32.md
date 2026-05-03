# Build vm_interp_arm32.bin

The ARM32 VM interpreter blob requires the **arm-linux-gnueabihf-gcc** cross-compilation toolchain, which can be generated using the following methods.

## Method 1: Native macOS (Recommended)

Use a script to automatically download the messense pre-compiled toolchain and build (approx. 150MB download):

```bash
./scripts/build_stub32_native.sh
```

The script will:
1. Download the `armv7-unknown-linux-gnueabihf` toolchain to `scripts/toolchains/`
2. Execute `make stub32`
3. Copy products to `vmp-gui/backend/api/vm_interp_arm32.bin`

## Method 2: Docker

Ensure Docker is installed, then execute:

```bash
./scripts/build_stub32_docker.sh
```

## Method 3: Native Linux Build

On Ubuntu/Debian:

```bash
sudo apt-get install gcc-arm-linux-gnueabihf binutils-arm-linux-gnueabihf
make stub32
cp cmd/vmpacker/vm_interp_arm32.bin vmp-gui/backend/api/
```

## Method 4: macOS Homebrew

```bash
brew tap messense/macos-cross-toolchains
brew install messense/macos-cross-toolchains/armv7-unknown-linux-gnueabihf
make stub32 CROSS_ARM32=/opt/homebrew/opt/armv7-unknown-linux-gnueabihf/bin/armv7-unknown-linux-gnueabihf-
cp cmd/vmpacker/vm_interp_arm32.bin vmp-gui/backend/api/
```

## Method 5: Windows (Makefile uses PowerShell)

```cmd
make stub32
copy /Y cmd\vmpacker\vm_interp_arm32.bin vmp-gui\backend\api\
```

## Verification

The size of a successful blob should be around **8KB–12KB** (similar to the ~10KB of ARM64). If it's only a few dozen bytes, it indicates a failed build or an incomplete product.
