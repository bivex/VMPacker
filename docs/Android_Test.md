# Android Platform Verification

This document records the successful verification of VMPacker-protected binaries on the Android platform.

## Environment
- **Host:** macOS (ARM64, M1/M2/M3)
- **Target:** Android Emulator (`emulator-5554`)
- **Architecture:** ARM64 (AArch64)
- **OS:** Android (Linux Kernel)

## Verified Features
- [x] **VMP Stub Compatibility:** The core interpreter runs natively on the Android kernel.
- [x] **ASLR Support:** Successfully handles memory randomization using the `OP_S_LOAD_SLIDE` logic.
- [x] **Linux Syscall Compatibility:** Native `write` and `exit` syscalls work correctly within the Android environment.
- [x] **Tokenized Entry:** The 3-instruction trampoline successfully jumps into the VM.
- [x] **JNI Library Support:** Successfully virtualized complex logic within an Android `.so` file.

## JNI Library Test (Verification 2)

We tested a full JNI library (`libnative_test.so`) containing 4 protected functions:
1. `vmp_compute`: Complex arithmetic and string hashing.
2. `vmp_verify_key`: String pattern matching logic.
3. `vmp_md5_hex`: Full MD5 implementation virtualized.
4. `vmp_get_process_name`: Reading from `/proc/self/cmdline`.

### Execution
The library was built with NDK, protected with `vmpacker`, and executed via a native `test_runner` on the Android emulator.

### Results
| Test Category | Original (.so) | Protected (.vmp.so) | Status |
|---------------|----------------|----------------------|--------|
| Logic Correctness | 11/11 PASS | 11/11 PASS | **SUCCESS** |

## Static Analysis & Hardening Verification

We used `objdump` and `capstone`-based analysis to verify the effectiveness of the protection on the `libnative_test_protected_arm64.so` file.

### 1. Code Destruction
The original ARM64 instructions were successfully removed. The space previously occupied by the function logic was filled with random "garbage" bytes to thwart static analysis and pattern matching.

### 2. Trampoline Analysis
Analysis of the entry point for `vmp_get_process_name` (at offset `0x5C1C`) shows the 12-byte **Tokenized Trampoline**:

```arm64
5c1c:  52800070  mov   w16, #0x3             // Load Function ID (3)
5c20:  72b4a010  movk  w16, #0xa500, lsl #16 // Load XOR Key (0xA5)
5c24:  140029f2  b     103ec                 // Branch to VM Interpreter
```

### 3. Verification of Hidden Logic
Any instructions following the trampoline (offset `0x5C28` and beyond) appeared as `undefined` or invalid random instructions. This confirms that the original logic is entirely absent from the `.text` section and exists only as encrypted bytecode in a separate data segment.

## Deployment & Execution Steps

Follow these steps to reproduce the test on an ARM64 Android device or emulator:

### 1. Push the protected binary
Android allows execution of binaries in specific directories. Use `/data/local/tmp` for testing:

```bash
# From the project root
adb push demo/demo_simple.vmp /data/local/tmp/
```

### 2. Grant execution permissions
```bash
adb shell "chmod +x /data/local/tmp/demo_simple.vmp"
```

### 3. Run and verify
The sample program calculates `10 * 3 + 7 = 37`. It prints the last digit (`7`) and sets the exit code to the full result (`37`).

```bash
adb shell "/data/local/tmp/demo_simple.vmp; echo 'Exit code: '\$?"
```

### Expected Output
```text
7
Exit code: 37
```

## Conclusion
VMPacker is fully compatible with Android ARM64 environments. Protected binaries can be safely integrated into Android applications (e.g., as part of a JNI `.so` library or as standalone native components).
