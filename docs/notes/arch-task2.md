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
*   **Relocations**: ✅ Verified. Calls to `libc` via PLT work correctly without crashing.
*   **ABI / Return Values**: ⚠️ Current results return address-like constants. Investigation suggests a mismatch in the calling convention during the VM-to-Native transition (specifically the restoration of callee-saved registers or `X0` corruption).

## 3. Future Work

1.  **ABI Compliance**: Fix the register saving/restoration logic in `vm_entry_token` to ensure `X19-X28` are preserved and `X0` is never overwritten by cleanup code (like `munmap`).
2.  **RTLR for ARM32**: Port the Runtime Relocation logic to the 32-bit interpreter for legacy Android support.
3.  **Section Header Recovery**: Optionally implement a "re-construct section headers" feature for better compatibility with static analysis tools.

---
*Status: Issue #7 Closed. Android basic support implemented.*
