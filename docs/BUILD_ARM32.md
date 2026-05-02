# 构建 vm_interp_arm32.bin

ARM32 VM 解释器 blob 需要 **arm-linux-gnueabihf-gcc** 交叉编译工具链，可用以下方式生成。

## 方式一：macOS 原生（推荐）

使用脚本自动下载 messense 预编译工具链并构建（约 150MB 下载）：

```bash
./scripts/build_stub32_native.sh
```

脚本会：
1. 下载 `armv7-unknown-linux-gnueabihf` 工具链到 `scripts/toolchains/`
2. 执行 `make stub32`
3. 复制产物到 `vmp-gui/backend/api/vm_interp_arm32.bin`

## 方式二：Docker

确保已安装 Docker，然后执行：

```bash
./scripts/build_stub32_docker.sh
```

## 方式三：Linux 原生构建

在 Ubuntu/Debian 上：

```bash
sudo apt-get install gcc-arm-linux-gnueabihf binutils-arm-linux-gnueabihf
make stub32
cp cmd/vmpacker/vm_interp_arm32.bin vmp-gui/backend/api/
```

## 方式四：macOS Homebrew

```bash
brew tap messense/macos-cross-toolchains
brew install messense/macos-cross-toolchains/armv7-unknown-linux-gnueabihf
make stub32 CROSS_ARM32=/opt/homebrew/opt/armv7-unknown-linux-gnueabihf/bin/armv7-unknown-linux-gnueabihf-
cp cmd/vmpacker/vm_interp_arm32.bin vmp-gui/backend/api/
```

## 方式五：Windows（Makefile 使用 PowerShell）

```cmd
make stub32
copy /Y cmd\vmpacker\vm_interp_arm32.bin vmp-gui\backend\api\
```

## 验证

成功的 blob 大小应在 **8KB–12KB** 左右（与 ARM64 的 ~10KB 相近）。若只有几十字节，说明构建失败或产物不完整。
