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

### VM_INDIRECT_DISPATCH Mode (Obfuscation)
*   **Indirect Dispatch Table**: Implemented an alternative execution path using a runtime-initialized function pointer table (`vm_jump_table`) instead of GCC computed-goto. This adds a layer of obfuscation that static analysis tools (IDA Pro) cannot easily trace.
*   **Stack Opcode Support**: Added all stack machine opcodes (`OP_S_VLOAD`, `OP_S_VSTORE`, `OP_S_ADD`, `OP_S_SUB`, ..., `OP_S_CMP`, `OP_S_LD8/16/32/64`, `OP_S_ST8/16/32/64`, `OP_SVLD`, `OP_SVST`) to both the `vm_init_jump_table` and the `dtab` arrays for both dispatch modes.
*   **Bytecode Reversal Optimization**:
    *   Implemented optional bytecode reversal (reverse PC iteration) to further confuse decompilers.
    *   The Packer now reverses the bytecode stream and builds an `offsetMap` to remap all metadata (BR jump maps, RTLR relocation offsets) to the new ordering.
    *   Fixed a critical **RTLR offset overlap bug** where the `_token_table_va + 16` patch was overwriting the first bytecode instruction. The fix reserves an 8-byte gap immediately after the interpreter code in the payload.
    *   Ensured all relocation entries have their `BcOffset` remapped through the `offsetMap` after reversal so `CALL_NAT` and other external references patch the correct offsets.

### Instruction Decoder Completeness
*   **Missing Opcode**: Added `OP_S_NEG` (0x4B) to `vm_insn_size()` which was previously absent, causing size-0 decode failures.

## 2. Testing Status (Android Emulator)

*   **Full Function Coverage**: ✅ All 4 test functions pass:
    *   `vmp_compute` — pure arithmetic/bitwise logic (mode 0/1/2)
    *   `vmp_verify_key` — license-key checksum verifier
    *   `vmp_md5_hex` — MD5 digest with libc PLT calls (`strlen`, `memcpy`, `memset`, `snprintf`)
    *   `vmp_get_process_name` — reads `/proc/self/comm` via `open`/`read`/`close`
*   **VM Execution**: ✅ Verified. Complex control-flow (loops, branches) and stack-machine ALU execute correctly.
*   **Relocations**: ✅ Verified. PLT calls to libc functions resolve correctly under ASLR with both normal and reversed bytecode orders.
*   **Observability**: Debug trace (`-debug` flag) confirms correct opcode stream, relocation patching, and stack machine evaluation.

### Verification Commands used:
```bash
# 1. Rebuild the tool and the stub
make clean && make stub CROSS= && make packer

# 2. Protect the Android test library (all 4 functions)
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

1.  ~~**ABI Compliance**: Fix the register saving/restoration logic in `vm_entry_token` to ensure `X19-X28` are preserved and `X0` is never overwritten by cleanup code (like `munmap`).~~ **Done.**
    - `vm_entry_token` stack frame expanded from 256B to 304B to save/restore callee-saved X19-X28.
    - `vm_ctx_init` now initializes `R[19]-R[28]` from `args[26..35]` so VM virtual registers reflect caller values.
    - X0 return value preserved through cleanup (X19-X28 restores don't touch X0; `munmap` calls remain commented out).
2.  **RTLR for ARM32**: Port the Runtime Relocation logic to the 32-bit interpreter for legacy Android support.
3.  **Section Header Recovery**: Optionally implement a "re-construct section headers" feature for better compatibility with static analysis tools.
4.  **Extended Obfuscation**: Explore additional obfuscation transformations (control-flow flattening, bogus branches, VM_INDIRECT_DISPATCH as default).
---

*Status: Issue #7 Closed. Android .so support fully functional with all tests passing. VM_INDIRECT_DISPATCH mode and bytecode reversal obfuscation validated.*
