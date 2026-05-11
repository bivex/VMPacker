# VMPacker: x86_64 Support Plan

This document outlines the strategy for adding x86_64 (AMD64) architecture support to VMPacker.

## 1. Core Architecture Support

### 1.1 ELF Machine Detection
Update `pkg/binary/elf/process.go`'s `validateArch` function to recognize `elf.EM_X86_64` and `elf.ELFCLASS64`.

```go
case f.Machine == elf.EM_X86_64 && f.Class == elf.ELFCLASS64:
    p.isARM32 = false
    p.isX86_64 = true
```

### 1.2 x86_64 Instruction Decoding
Create `pkg/arch/x86_64/decoder.go`.
- **Approach:** Use `golang.org/x/arch/x86/x86asm` for robust instruction decoding.
- **Goal:** Map raw bytes to a list of `vm.Instruction` or an intermediate representation that the translator can consume.

### 1.3 x86_64 Instruction Translation
Create `pkg/arch/x86_64/translator.go`.
- **Register Mapping:** Map x86_64 general-purpose registers (RAX, RCX, RDX, RBX, RSP, RBP, RSI, RDI, R8-R15) to VM virtual registers.
- **Stack-based Mapping:**
  - `ADD RAX, RBX` → `S_VLOAD(RBX)`, `S_VLOAD(RAX)`, `S_ADD`, `S_VSTORE(RAX)`
  - `MOV [RAX], RBX` → `S_VLOAD(RAX)`, `S_VLOAD(RBX)`, `S_STORE64`
- **RIP-relative Addressing:** Handle RIP-relative data access common in x86_64.

## 2. Interpreter Stub (`stub/linux/x86_64`)

The interpreter must be implemented in C/Assembly for x86_64, mirroring the ARM64 modular structure.

### 2.1 Assembly Entry (`vm_entry.S`)
- Save all caller-saved registers.
- Setup VM context (virtual registers, stack pointer).
- Call `vm_interp_main`.
- Restore registers and return.

### 2.2 VM Handlers (`vm_handlers/*.h`)
- Implement architecture-specific handlers if needed (though most are stack-based and generic).
- Ensure `S_CALL_NATIVE` handles x86_64 calling conventions (System V AMD64 ABI: RDI, RSI, RDX, RCX, R8, R9).

### 2.3 Build Rules
Update `Makefile` to compile the x86_64 stub using `gcc`.

## 3. ELF Integration & Trampolines

### 3.1 Token Trampoline
In `pkg/binary/elf/trampoline.go`, implement `BuildTokenTrampolineX86_64`.
```assembly
; Passing token in R11 (IP register equivalent)
mov r11, <token>
jmp <vm_entry_token>
```
Size: ~12-15 bytes.

### 3.2 Payload Injection
Update `pkg/binary/elf/inject.go` to support 64-bit injection for x86_64, potentially reusing `injectVMPBatch64` with minor adjustments for architecture-specific fields.

## 4. Tooling & UI Updates

### 4.1 CLI Updates (`cmd/vmpacker/main.go`)
- Embed the x86_64 interpreter blob:
  ```go
  //go:embed vm_interp_x86_64.bin
  var interpBlobX86_64 []byte
  ```
- Pass the blob to the packer:
  ```go
  packer.SetInterpBlobX86_64(interpBlobX86_64)
  ```

### 4.2 GUI Updates (`vmp-gui/`)
- Update the architecture selection or automatic detection display in the GUI.
- Ensure the backend (`app.go`) correctly handles the new architecture flag.

## 5. Testing & Validation


### 4.1 Unit Tests
- `pkg/arch/x86_64/decoder_test.go`: Verify correct decoding of common instructions.
- `pkg/arch/x86_64/translator_test.go`: Verify translation of complex sequences (control flow, memory access).

### 4.2 Integration Tests
- Create `demo/x86_64/` with sample C programs.
- Verify execution on Linux x86_64 using the protected binaries.

## 5. Implementation Phases

1. **Phase 1: Basic Infrastructure** (Detection, Stub skeleton, Trampoline)
2. **Phase 2: Decoder & Translator** (Common ALU and Data movement)
3. **Phase 3: Control Flow & Memory** (Jumps, Calls, Load/Store)
4. **Phase 4: Full Stub implementation** (All 63+ VM opcodes)
5. **Phase 5: Refinement & Obfuscation** (CFF, MBA support for x86_64)
