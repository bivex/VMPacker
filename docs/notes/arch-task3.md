# Architecture Task 3: Android PLT Call ABI Compliance

## Current Status

**Fixed (Task 2 completion):**
- RTLR double-slide bug in `h_call_nat` — VM no longer adds slide (packer already patches with final runtime address `target_va + slide`)
- SIGSEGV on CALL_NAT resolved — all 4 protected functions reach completion

## Remaining Issues

| Function | Status | Issue |
|----------|--------|-------|
| `vmp_compute` | ✅ PASS | Pure VM, no external calls |
| `vmp_verify_key` | ✅ PASS | Pure VM, no external calls |
| `vmp_md5_hex` | ❌ FAIL | Returns "B" instead of 32-char MD5 hex |
| `vmp_get_process_name` | ❌ FAIL | Returns empty string instead of process name |

## Root Cause Analysis

**Hypothesis:** VM-to-native ABI bridge has stack alignment or register preservation issues.

ARM64 AAPCS requires 16-byte stack alignment before `BL` instructions. The VM's `eval_stk` may not maintain this invariant across `CALL_NAT` transitions, causing undefined behavior in variadic functions (`snprintf`) and register clobbering.

### Failure Pattern
- **Pure-VM functions** (no external calls): PASS
- **PLT-calling functions**: FAIL

This implicates the native transition bridge, not bytecode translation or RTLR.

## Next Steps

1. **Stack alignment verification** in `vm_entry_token` / `vm_exit_token`
   - Ensure `SP % 16 == 0` before native calls
   - Check if `eval_stk` preserves alignment

2. **Register preservation audit** across native calls
   - Verify X0-X7 (argument registers) are correctly set before CALL_NAT
   - Ensure callee-saved registers (X19-X29) are preserved across VM-exit/VM-entry

3. **Debug vmp_md5_hex specifically**
   - `snprintf` receives misaligned stack or corrupted buffer pointer
   - Buffer allocated on eval_stk may have wrong alignment

4. **Debug vmp_get_process_name specifically**
   - `open()`/`read()` sequence loses file descriptor
   - Check X0 (return value from `open`) is not clobbered

## Key Files to Investigate

- `stub/linux/arm64/vm_interp.c` — RTLR parsing and CALL_NAT dispatch
- `stub/linux/arm64/vm_handlers/h_system.h` — `h_call_nat` handler
- `pkg/arch/arm64/translator.go` — PLT call translation and RTLR generation
- `pkg/binary/elf/packer.go` — RTLR table construction

## Reproduction Steps

```bash
# Build and test
make clean && make stub && make packer
./build/vmpacker -func vmp_compute,vmp_verify_key,vmp_md5_hex,vmp_get_process_name \
    -o test/android/build/libnative_test_protected_arm64.so \
    test/android/build/libnative_test_arm64.so
adb shell "cd /data/local/tmp/vmptest && LD_LIBRARY_PATH=. ./test_runner_arm64"
```