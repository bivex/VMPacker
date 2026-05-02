# VMPacker and Android

## Current Status

**VMPacker currently supports only Linux/ARM64 ELF binaries.** Android `.so` libraries are not supported out of the box.

### Why It Doesn't Work on Android

1. **Bionic vs glibc**: The VM interpreter stub uses Linux syscalls and glibc assumptions; Android uses Bionic with different syscall numbers and library semantics.
2. **Dynamic linking model**: `.so` files on Android are loaded by the dynamic linker (`linker`/`linker64`), not executed directly. The ELF injection must preserve DT_NEEDED, DT_SONAME, relocations, and PLT/GOT entries.
3. **Position-independent code requirements**: DSOs must use RIP-relative addressing and proper GOT/PLT handling; the current stub is built as a flat PIC blob for static ELF injection.
4. **APK signature constraints**: Modifying `.so` files inside an APK breaks the v2/v3 signature scheme; re-signing is required.
5. **No JNI integration**: Native methods (`JNI_OnLoad`, `Java_*` functions) require handling the JNIEnv and jobject parameters correctly across the VM boundary.

---

## What Needs to Change for Android Support

### 1. **VM Interpreter Stub (C code in `stub/`)**

| Change | Reason |
|--------|--------|
| Replace glibc-specific code with Bionic-compatible equivalents | Bionic lacks some glibc internals; use raw syscalls or NDK APIs |
| Use Android syscall numbers (`__NR_*` from `asm/unistd.h`) | Syscall numbers differ from Linux |
| Handle `__system_property_get` if needed | Android property access |
| Ensure no `static` linking of unavailable libc features | Bionic does not support full `-static` for all symbols |
| Add `__attribute__((visibility("default")))` to entry points | Ensure VM entry symbols are exported to Java |

### 2. **ELF Injection (Go code in `pkg/binary/elf/`)**

| Requirement | Implementation |
|-------------|----------------|
| Preserve `.dynamic`, `.dynsym`, `.dynstr`, `.rela*` sections | Required for dynamic linker |
| Keep `DT_NEEDED`, `DT_SONAME`, `DT_RPATH`, `DT_RUNPATH` intact | Android loader relies on these |
| Avoid corrupting PLT/GOT entries | Function pointers must resolve correctly |
| Handle `PT_GNU_RELRO` segment | Read-only after relocation on Android |
| Support `android:debuggable` flag and `app_process` binding | If protecting system apps |

### 3. **Relocation Awareness**

- Static ELF: relocation is resolved at inject time.
- Shared ELF: relocation is performed by Android's dynamic linker at load time.
- The injected VM interpreter must not break relocation processing; typically you can inject into a new **custom PT_LOAD segment** or hijack an unused section (`.note.gnu.build-id` or similar) without altering sections referenced by relocations.

### 4. **JNI Function Protection**

To protect JNI native methods:

```c
JNIEXPORT jstring JNICALL
Java_com_example_MyClass_nativeGetSecret(JNIEnv *env, jobject thiz) {
  // This function becomes a VM-protected entry point
}
```

- The VM entry must accept `JNIEnv*` and `jobject` as arguments.
- Any JNI calls inside the protected function must either be:
  - Made via `(*env)->Call*Method` through a stored JNIEnv pointer, or
  - Performed before/after VM protection, or
  - Redesigned to avoid JNI calls inside protected code.

### 5. **APK Packaging & Signing**

1. Extract APK → `unzip app.apk -d out/`
2. Locate target `.so` in `out/lib/arm64-v8a/`
3. Run VMPacker with an Android-specific mode
4. Replace the modified `.so`
5. Re-sign APK with `apksigner` or `jarsigner`
6. Align with `zipalign`

---

## Target ABI Matrix

| ABI | Architecture | Status |
|-----|--------------|--------|
| `arm64-v8a` | AArch64 (ARM 64-bit) | **Core target** — requires Bionic stub |
| `armeabi-v7a` | ARM 32-bit (Thumb-2) | Not planned — different ISA |
| `x86` | x86 32-bit | Not planned — different ISA |
| `x86_64` | x86-64 | Not planned — different ISA |

**Only ARM64 is relevant** because VMPacker is built specifically around the AArch64 instruction set architecture.

---

## Proposed Architecture for Android Mode

```
+------------------+
|  Android APK     |
|   lib/arm64-v8a/ |
|   libtarget.so   |
+--------+---------+
         |
         v
+------------------+
|  VMPacker-Android|
|  (fork or mode)  |
|                 |
|  - Parse ELF DSO |
|  - Inject VM stub|
|    respecting    |
|    dynamic linker|
+--------+---------+
         |
         v
+------------------+
|  Protected .so   |
|  (modified DSO)  |
+--------+---------+
         |
         v  (load by linker)
+------------------+
|  Android Process |
|  VM runs inside  |
|  native address  |
|  space           |
+------------------+
```

### Key Differences from Linux Mode

| Aspect | Linux Executable | Android Shared Library |
|--------|----------------|------------------------|
| **Entry point** | `_start` (static) | `JNI_OnLoad` or `.init`/`.init_array` |
| **Syscalls** | Direct | Mapped through `__NR_*` in Bionic |
| **Dynamic linking** | None or lazy | Full PLT/GOT, lazy binding |
| **Symbol resolution** | At `execve()` time | At `dlopen()`/first-call time |
| **Addressing** | Fixed load address | ASLR with random base |

---

## Development Roadmap (Android)

1. **Phase 1**: Build a Bionic-compatible VM stub
   - Create `stub/android/arm64/` analogous to `stub/linux/arm64/`
   - Test standalone `.so` that exports `vm_entry` and `vm_entry_token`
   - Verify it loads via `dlopen()` without linker errors

2. **Phase 2**: Extend `pkg/binary/elf` packer
   - Add `DSOMode` vs `ExeMode` in configuration
   - Implement safe injection that does not break dynamic relocations
   - Add `-android` flag to CLI

3. **Phase 3**: JNI-specific support
   - Auto-detect `JNI_OnLoad` and wrap it
   - Support `NativeMethod` protection declared in `@Keep`-annotated Java classes

4. **Phase 4**: APK workflow (optional, separate tool)
   - `vmpacker-apk` subcommand that automates unzip → protect → resign

---

## Alternatives & Workarounds

- **Static-linked native executable**: If your app uses a native helper binary (executable, not `.so`), current VMPacker works as-is on Linux/ARM64; but Android does not allow executing arbitrary binaries from app sandbox without special permissions.
- **Manual stub replacement**: Replace `vm_interp.bin` with a Bionic-built version and pray — not recommended; the injection logic itself may assume Linux-specific ELF semantics.
- **Use on Linux ARM64 devices**: If your target is ARM64 Linux (Raspberry Pi, server, embedded), current VMPacker is ready.

---

## References

- Android NDK: [System Calls](https://developer.android.com/ndk/guides/system-calls)
- ABI: [Android ABI管理](https://developer.android.com/ndk/guides/abis)
- Bionic source: [bionic.git](https://android.googlesource.com/platform/bionic/)
- ELF handling: `linker` source in AOSP (`bionic/linker/`)

---

**Bottom line:**  
Supporting Android `.so` will require a significant rewrite of the stub and careful ELF injection logic. It is doable but not a trivial configuration change.
