# VMPacker Test Program

A simple C++ test program used to verify if the program can run normally after VMP protection.

## Program Description

- **simple_app.cpp**: Contains the `log2Console` function, using common instructions (arithmetic, logic, branching, loops, memory access) and `printf` log output
- Protection target: `log2Console` function

## Environment Requirements

- **ARM64 Compilation**: `aarch64-linux-gnu-gcc` (Ubuntu: `apt install gcc-aarch64-linux-gnu`)
- **ARM32 Compilation**: `arm-linux-gnueabihf-gcc` (Ubuntu: `apt install gcc-arm-linux-gnueabihf`)
- **macOS**: can be installed via `brew install messense/macos-cross-toolchains/aarch64-unknown-linux-gnu`

## Compilation

```bash
# ARM64 executable
make arm64

# ARM32 executable (e.g., armeabi-v7a)
make arm32
```

## Protection

```bash
# First ensure vmpacker is built (run make all in the project root directory)
# Protect ARM64 version
go run ./cmd/vmpacker/ -func log2Console -v -debug -o simple_app_protected simple_app_arm64

# Or use Makefile
make protect
```

## Running

Run on Linux ARM64 device or under QEMU:

```bash
# Native execution (on ARM64 Linux)
./simple_app_protected

# Using QEMU (macOS/Linux x86)
qemu-aarch64 -L /usr/aarch64-linux-gnu/ simple_app_protected
```

## Expected Output

```
=== VMPacker Simple App Test ===
[LOG] init: x=10, y=20 => result=...
[LOG] step1: x=5, y=15 => result=...
[LOG] step2: x=100, y=200 => result=...
[LOG] Final result: ...
=== Test completed successfully ===
```

If the output after protection is consistent with the output before protection, it indicates that the VMP protection is working normally.

## Use Docker to verify 32-bit programs

```bash
docker run --rm --platform linux/arm/v7 -v $(pwd):/work -w /work \
  arm32v7/ubuntu:20.04 bash -c "\
    "
```

## Use Docker to verify 64-bit programs

```bash
docker run --rm -v $(pwd):/work -w /work \
  ubuntu:22.04 bash -c "\
    "
```
