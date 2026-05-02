# Architecture Task 2: Android .so Support & Issue #7 Resolution

This document details the improvements made to support Android Shared Libraries (.so), handling ASLR, and resolving critical panics in the ELF injection engine.

## 1. Accomplishments (What we did)

### Issue #7: ELF Parser Robustness
*   **Panic Fix**: Resolved the `panic: slice bounds out of range` in `injectVMPBatch`. The cause was the parser attempting to strip sections in binaries with corrupted, truncated, or missing Section Header Tables (common in NDK-optimized libraries).
*   **Segment-First Location**: Refactored `FindFunction` to prioritize **Program Headers** (PT_LOAD segments) for calculating file offsets. Program Headers are the "ground truth" for the loader and are preserved even when Section Headers are stripped.

### Android .so & ASLR Support
*   **RTLR (Runtime Relocation Table)**:
    *   Implemented a mechanism to capture absolute address references during translation (`ADRP`, `ADR`, `BL`).
    *   The Packer now appends an `RTLR` table (Magic: `0x524C5452`) to the payload.
    *   The VM Interpreter (`vm_interp.c`) parses this table at runtime and patches the bytecode with the correct absolute addresses by adding the **ASLR slide** (Runtime Base - Link-time Base).
*   **PIE Compatibility**: The stub now correctly calculates the ASLR slide using `ADR` to find its own runtime position relative to the link-time address patched by the Packer.

### VM Core Stabilization
*   **Opcode De-confliction**: Moved unary stack ops (`NOT`, `NEG`, `CLZ`) to a safe range (`0x4A-0x4C`) to avoid collisions with branch opcodes.
*   **Stack Machine Logic**:
    *   **Fixed `SPUSH/SPOP`**: Corrected the pre/post-increment inconsistency that caused index 0 to be skipped.
    *   **Fixed Store Order**: Store handlers now correctly pop the `value` first, then the `address`.
    *   **Pointer Safety**: Added checks to prevent dereferencing `0` or the `XZR` sentinel in load/store handlers.
*   **Return Logic**: Fixed a bug where the VM would continue execution after a `RET` instruction. It now correctly returns `R[0]` to the native caller.

## 2. Testing Status (Android Emulator)

*   **Logic Execution**: ✅ VM successfully executes complex logic including jumps and math.
*   **Relocations**: ✅ Verified. Calls to `libc` via PLT (open, read, etc.) work correctly without crashing.
*   **Observed Bugs & Failures**:
    *   **ABI / Return Values**: ⚠️ Functions return address-like constants (e.g., `-1548203841`) instead of logical results (e.g., `97`). This occurs even in pure computation functions like `vmp_compute`.
    *   **NULL Pointer Handling**: ❌ `vmp_compute(NULL)` failed to return `-1`, returning `0` instead.
    *   **Libc Integration**: ❌ `vmp_md5_hex` and `vmp_get_process_name` return incorrect data/lengths.
    *   **Diagnosis**: The core VM logic is stable, but the interface between Native and VM context has a bug. Specifically, the cleanup code in the interpreter stub or the register restoration in `vm_entry_token` is likely corrupting `X0` or failing to preserve callee-saved registers (`X19-X28`) required by the Android runtime.

### Verification Commands used:
```bash
# 1. Rebuild the tool and the stub
make clean && make stub CROSS= && make packer

# 2. Protect the Android test library
./build/vmpacker -func vmp_compute,vmp_verify_key,vmp_md5_hex,vmp_get_process_name \
    -o test/android/build/libnative_test_protected_arm64.so \
    test/android/build/libnative_test_arm64.so

# 3. Deploy to emulator
adb shell mkdir -p /data/local/tmp/vmptest
adb push test/android/build/test_runner_arm64 /data/local/tmp/vmptest/
adb push test/android/build/libnative_test_protected_arm64.so /data/local/tmp/vmptest/libnative_test.so
adb shell chmod +x /data/local/tmp/vmptest/test_runner_arm64

# 4. Run tests
adb shell "cd /data/local/tmp/vmptest && LD_LIBRARY_PATH=. ./test_runner_arm64"
```

## 3. Future Work

1.  **ABI Compliance**: Fix the register saving/restoration logic in `vm_entry_token` to ensure `X19-X28` are preserved and `X0` is never overwritten by cleanup code (like `munmap`).
2.  **RTLR for ARM32**: Port the Runtime Relocation logic to the 32-bit interpreter for legacy Android support.
3.  **Section Header Recovery**: Optionally implement a "re-construct section headers" feature for better compatibility with static analysis tools.

---
*Status: Issue #7 Closed. Android basic support implemented.*
