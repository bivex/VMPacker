# ARM64 VM Runtime Security Protections

This document describes the runtime security features implemented in the VMPacker ARM64 interpreter stub. These protections are designed to detect and prevent debugging, tampering, and hooking of the virtualized code.

## 1. Anti-Debugging Mechanisms

The VM stub employs multiple layers of detection to identify if it is running under a debugger or being traced.

| Mechanism | Description | Response |
| :--- | :--- | :--- |
| **TracerPid Check** | Scans `/proc/self/status` to check if `TracerPid` is non-zero. | **Aggressive Panic** |
| **Ptrace Check** | Attempts a `PTRACE_TRACEME` call. If it fails, a debugger is already attached. | **Aggressive Panic** |
| **Timing Check** | Measures the time taken for initialization and between instruction intervals using the ARM virtual timer (`CNTVCT_EL0`). | **Aggressive Panic** |
| **Breakpoint Scanning** | Scans the entire bytecode region for software breakpoint instructions (`BRK #0`). | **Aggressive Panic** |

## 2. Anti-Tampering & Integrity

These measures ensure that neither the interpreter stub nor the virtualized bytecode has been modified in memory.

| Mechanism | Description | Exit Code |
| :--- | :--- | :--- |
| **Startup CRC32** | Performs a full CRC32 check of the bytecode trailer upon VM entry. | `103` |
| **Periodic CRC32** | Re-calculates and verifies the bytecode integrity every 1024 virtual instructions. | `110` |
| **Dump Protection** | Uses `madvise(MADV_DONTDUMP)` on the bytecode buffer to prevent it from appearing in core dumps. | N/A |
| **Secure Zeroing** | All sensitive VM context, registers, and stack buffers are zeroed out immediately upon exit or failure. | N/A |

## 3. Anti-Hooking Protection

The VM includes an inline hook scanner that protects critical functions from being intercepted.

| Mechanism | Description | Response |
| :--- | :--- | :--- |
| **Inline Hook Scanner** | Detects branches (`B`), calls (`BL`), or PC-relative loads (`LDR/ADR`) at the start of critical functions like `memcpy`, `mmap`, `ptrace`, and the VM entry point. | **Aggressive Panic** |

## 4. Verification & Testing

The `demo/demo_security_demo.c` file provides a base for testing these protections.

### **Testing Anti-Debug**
Run the protected demo under `strace`:
```bash
vmpacker -func security_test_loop -o demo.vmp demo_app
strace ./demo.vmp
# Should exit with code 101 or 102
```

### **Testing Anti-Tamper**
Manually patch a byte in the `.vmp` file within the bytecode region:
```bash
# Example: flip a byte at a known bytecode offset
printf '\xff' | dd of=demo.vmp bs=1 seek=<offset> conv=notrunc
./demo.vmp
# Should exit with code 103
```

### **Testing Anti-Hook**
Patch the start of `memcpy` in the stub with a NOP or Branch:
```bash
# Detects modification of the interpreter's own code
./demo.vmp
# Should crash due to UDF (SIGILL)
```

## 5. Aggressive Panic (The "UDF" Trap)

When a protection mechanism is triggered, the VM does not perform a clean `exit()`. Instead, it executes an **Undefined Instruction (UDF)**:

```c
__asm__ volatile(".inst 0x00000000"); // Trigger SIGILL
```

This immediately crashes the process, making it significantly harder for an automated script or a researcher to simply "patch out" the exit call. The internal exit codes (101-112) are passed to the `sec_panic` function but are primarily used for internal logging during development; in production, any detection results in an immediate crash.

- **101/112:** Debugger detected (TracerPid).
- **102/111:** Debugger detected (Ptrace).
- **103:** Integrity failure (Startup CRC).
- **104-107:** Hook detected (memcpy, vm_entry, mmap, ptrace).
- **108:** Timing anomaly detected.
- **109:** Software breakpoint detected.
- **110:** Integrity failure (Runtime periodic CRC).
