# VMPacker Android JNI Test

Verify VMPacker's protection effect on Android `.so` files (ARM32 + ARM64).

## Project Structure

```
test/android/
├── Makefile                          # Independent NDK compilation + VMPacker protection
├── README.md
├── jni/
│   ├── CMakeLists.txt                # CMake build (shared by Gradle and independent compilation)
│   ├── native_bridge.c               # JNI bridge: Java ↔ libvmptest
│   └── test_runner.c                 # adb shell independent test program (using dlopen)
├── app/
│   ├── build.gradle                  # Android module build
│   └── src/main/
│       ├── AndroidManifest.xml
│       ├── java/com/vmpacker/test/
│       │   ├── MainActivity.java     # Test interface
│       │   └── NativeTest.java       # JNI declaration
│       └── res/                      # Layout and resources
├── build.gradle                      # Project-level Gradle
├── settings.gradle
├── gradle.properties
└── scripts/
    └── protect_and_repack.sh         # protect .so in APK + re-sign
```

## Prerequisites

- **Android NDK** (r21+), set environment variables:
  ```bash
  export NDK_HOME=/path/to/android-ndk-rXX
  ```
- **Go 1.21+** (used to run VMPacker)
- **adb** (used for device testing)
- **Android SDK + Gradle** (only required for APK build)

## Method 1: Independent NDK Compilation + adb Testing (Recommended)

Compile `.so` directly with NDK and test on device without Gradle/Android SDK.

```bash
cd test/android

# Compile ARM64 .so + test program
make so64 runner64

# VMPacker protection
make protect64

# Push to device and run
make push64
make run-on-device

# One-key complete all steps
make test64

# Test both ARM32 and ARM64
make test-all
```

### Test Process

1. `make so32/so64` — Use NDK clang to compile `libnative_test.so`
2. `make protect32/protect64` — VMPacker protects target functions in `.so`
3. `make push32/push64` — Push to device via adb to `/data/local/tmp/vmptest/`
4. `make run-on-device` — Run tests with unprotected and protected `.so` respectively
5. Compare output: results should be exactly the same before and after protection

### Protected Functions

| Function | Type | Description |
|------|------|------|
| `vmp_compute` | Pure computation | Hash + Arithmetic + Bitwise operations |
| `vmp_verify_key` | Pure computation | Key verification logic |
| `vmp_md5_hex` | PLT call | MD5 calculation, calls `strlen`/`memcpy`/`memset`/`snprintf` |
| `vmp_get_process_name` | PLT call | Read `/proc/self/comm`, calls `open`/`read`/`close` |

## Method 2: Gradle APK Build

Build a complete Android app and run UI tests on the device.

```bash
cd test/android

# Build debug APK
make apk

# Protect .so in APK and re-sign
make apk-protect

# Install to device
make apk-install
```

APK build requires additional configuration in `local.properties`:
```properties
sdk.dir=/path/to/Android/sdk
ndk.dir=/path/to/android-ndk-rXX
```

## Expected Output

```
=== VMPacker Android SO Test Runner ===
[*] Loading: ./libnative_test.so

--- vmp_compute ---
  compute("hello",0,42) = XXXXX
  [PASS] compute returns non-negative for valid input
  [PASS] compute returns -1 for NULL
  [PASS] different mode gives different result
  [PASS] different input gives different result

--- vmp_md5_hex ---
  md5("hello")     = 5d41402abc4b2a76b9719d911017c592  (rc=0)
  [PASS] md5("hello") matches
  ...

========================================
  PASS: XX    FAIL: 0    TOTAL: XX
========================================
```

The protected `.so` should produce exactly the same output.

## Notes

- ARM64 compilation uses `-mgeneral-regs-only` to avoid NEON/SIMD instructions (VMPacker does not yet support them)
- The test program uses `dlopen` to load the `.so`, consistent with Android runtime behavior
- `/proc/self/comm` returns the process name on Android (i.e., package name or executable filename)
