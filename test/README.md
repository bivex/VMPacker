# VMPacker 测试程序

用于验证 VMP 保护后程序能否正常运行的简单 C++ 测试程序。

## 程序说明

- **simple_app.cpp**: 包含 `log2Console` 函数，使用常用指令（算术、逻辑、分支、循环、内存访问）及 `printf` 日志输出
- 保护目标: `log2Console` 函数

## 环境要求

- **ARM64 编译**: `aarch64-linux-gnu-gcc` (Ubuntu: `apt install gcc-aarch64-linux-gnu`)
- **ARM32 编译**: `arm-linux-gnueabihf-gcc` (Ubuntu: `apt install gcc-arm-linux-gnueabihf`)
- **macOS**: 可安装 `brew install messense/macos-cross-toolchains/aarch64-unknown-linux-gnu`

## 编译

```bash
# ARM64 可执行文件
make arm64

# ARM32 可执行文件（如 armeabi-v7a）
make arm32
```

## 保护

```bash
# 先确保已构建 vmpacker (在项目根目录执行 make all)
# 保护 ARM64 版本
go run ./cmd/vmpacker/ -func log2Console -v -debug -o simple_app_protected simple_app_arm64

# 或使用 Makefile
make protect
```

## 运行

在 Linux ARM64 设备或 QEMU 下运行：

```bash
# 原生运行（在 ARM64 Linux 上）
./simple_app_protected

# 使用 QEMU（macOS/Linux x86）
qemu-aarch64 -L /usr/aarch64-linux-gnu/ simple_app_protected
```

## 预期输出

```
=== VMPacker Simple App Test ===
[LOG] init: x=10, y=20 => result=...
[LOG] step1: x=5, y=15 => result=...
[LOG] step2: x=100, y=200 => result=...
[LOG] Final result: ...
=== Test completed successfully ===
```

若保护后输出与保护前一致，则说明 VMP 保护正常工作。

## 使用 Docker 验证32位程序

```bash
docker run --rm --platform linux/arm/v7 -v $(pwd):/work -w /work \
  arm32v7/ubuntu:20.04 bash -c "\
    "
```

## 使用 Docker 验证64位程序

```bash
docker run --rm -v $(pwd):/work -w /work \
  ubuntu:22.04 bash -c "\
    "
```
