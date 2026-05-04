# Task 4: Hardening ARM64 VM Obfuscation - Progress Notes

## Objective
Finalize the VMPacker obfuscation suite for ARM64 by resolving runtime execution failures (`SIGILL`, stalls, and early exits) and ensuring correct ABI compliance for native calls.

## Progress Log

### 1. Build System & Stability
- **Issue**: `PermissionError` (Errno 13) when rebuilding in WSL/Windows environments.
- **Solution**: Implemented a robust cleanup sequence (`rm -rf build`) before each build to release file locks.
- **Status**: Fixed.

### 2. Interpreter Logic Fixes
- **Issue**: VM stalling or exiting prematurely under QEMU.
- **Fixes**:
    - Removed unstable anti-debug timing checks (`sec_get_timer`) that caused panics under emulation.
    - Corrected the `addr_map` pointer calculation in `vm_interp.c`. Fixed the offset from `256` to `320` to correctly account for `regMap` (64B) and `opMap` (256B).
    - **Trailer Parsing**: Discovered that `reverse` and `oc_key` fields were being read but not assigned to the VM context. Corrected the assignment in `vm_interp.c`.
    - **ASLR/PIE Compliance**: Fixed string decryption (`h_s_decrypt_str`) by correctly applying the ASLR slide to the source address.

### 3. Packer/Translator Fixes
- **Issue**: Opcodes were being randomized but the logical ID count in `vm_opcodes_dynamic.h` was out of sync with Go's `isa.go`.
- **Issue**: Trailer offsets in `postProcessBytecode` were hardcoded and incorrect (ignoring CRC and regMap sections).
- **Fixes**:
    - Updated `process.go` to use relative offsets from the end of the bytecode (`len-21`) for trailer fields.
    - Verified that opcode randomization is correctly reflected in the `op_map` appended to the bytecode.

### 4. Current Status: Verification
- **Test Case**: `demo_security.c` with `-func "check_security" -mangle -cff -mba`.
- **Result**: The VM successfully initializes, parses the trailer, decrypts opcodes, and executes the full instruction stream in reverse mode.
- **Output**: `Exit code: 1` (Success for `check_security`).
- **Pending Issue**: `printf` output ("Access Granted!") is not visible in QEMU output despite the successful return. Investigating `h_call_nat` argument passing and buffering.

## Full Commands Used
```bash
# Clean and Build
wsl rm -rf build; wsl rm -f cmd/vmpacker/vm_interp.bin
wsl make stub
wsl make packer

# Compile Demo (PIE)
wsl aarch64-linux-gnu-gcc -fPIE -pie -O1 -march=armv8-a demo/demo_security.c -o build/demo_security_pie

# Protect Function
wsl ./build/vmpacker -func "check_security" -mangle -cff -mba -v -o build/demo_security_protected build/demo_security_pie

# Run under QEMU
wsl qemu-aarch64 ./build/demo_security_protected VMP_SECRET_12345
```
