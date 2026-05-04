# VMPacker and Android

## Current Status

**VMPacker officially supports ARM64 Android `.so` (Shared Object) libraries.** Through the implementation of the **RTLR (Runtime Relocation Table)** and PIE-aware stub logic, VMPacker can successfully protect native JNI libraries.

### Why it works on Android now:

1.  **RTLR (Runtime Relocation)**: During protection, absolute address references (ADRP, ADR, BL) are captured. At runtime, the VM interpreter patches these addresses in the bytecode by calculating the ASLR slide.
2.  **ASLR Awareness**: The stub dynamically detects the runtime base address of the `.so` library by scanning `/proc/self/maps`.
3.  **Bionic Compatibility**: The VM interpreter stub is built with `-nostdlib` and uses raw Linux/Android syscalls (`__NR_*`), avoiding any direct dependencies on glibc.
4.  **JNI Support**: Native JNI methods can be protected using the `Tokenized Entry` mode, which preserves the `JNIEnv*` and `jobject` arguments across the VM boundary.

---

## How to Protect an Android `.so`

### 1. Build your JNI library
Ensure you build your library using the NDK and enable **PIE** (Position Independent Executable):
```bash
# In your Android.mk or CMakeLists.txt
LOCAL_CFLAGS += -fPIE
LOCAL_LDFLAGS += -fPIE -pie
```

### 2. Run VMPacker
```bash
# Protect JNI methods by name
./build/vmpacker -func "Java_com_example_app_NativeLib_secretMethod" -o libnative_protected.so libnative.so
```

### 3. Integration
1.  Replace the original `.so` in your Android project.
2.  Ensure the APK is re-signed (modifying `.so` files breaks the signature).
3.  Load the library as usual: `System.loadLibrary("native_protected")`.

---

## Target ABI Matrix

| ABI | Architecture | Status |
|-----|--------------|--------|
| `arm64-v8a` | AArch64 (ARM 64-bit) | **✅ Supported (Core Target)** |
| `armeabi-v7a` | ARM 32-bit (Thumb-2) | 🟡 Planned (Porting RTLR) |
| `x86 / x86_64` | Intel/AMD | ❌ Not Supported |

---

## Known Issues on Android

### 1. Stack Alignment (ABI)
Some complex functions (like those using `snprintf` or large variadic calls) may fail on Android due to 16-byte stack alignment requirements in the ARM64 AAPCS. Work is ongoing to ensure the VM's `eval_stk` perfectly mirrors native alignment before `CALL_NAT` transitions.

### 2. Signature Validation
Directly modifying a `.so` file inside an APK will cause `apksigner` to fail validation. You **must** re-sign the APK after protection.

---

## Development Roadmap (Android)

- [x] **RTLR support** for absolute address fixups.
- [x] **ASLR base detection** via `/proc/self/maps`.
- [x] **JNI compatibility** via Tokenized Entry.
- [ ] **ARM32 RTLR Port** for legacy device support.
- [ ] **Automated APK Workflow** (unzip -> protect -> sign).

---

*Last updated: 2026-05-04 (Antigravity)*
