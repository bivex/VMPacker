# VMPacker Integration Work Plan

## Source: revercc/VMPacker fork (commit 3caff19)
**Status:** 13 commits ahead, 11 behind LeoChen-CoreMind/master

---

## ✅ Completed Merges

### 1. `pkg/arch/arm64/translator.go`
- [x] Add `Relocation` struct definition
- [x] Add `currentFuncName string` and `relocations []Relocation` to `Translator` struct
- [x] Update `NewTranslator(funcAddr, funcSize, funcName)` signature
- [x] Append `result.Relocations = t.relocations` in `Translate()`
- [x] Add `Relocations []Relocation` to `TranslateResult`

### 2. `pkg/arch/arm64/tr_branch.go`
- [x] Replace `trBL()` with relocation-aware implementation:
  - Emit `MovImm + 0 + CallReg` instead of `CallNative`
  - Record `Relocation{BcOffset, TargetAddr, IsInternal, FuncName}`

### 3. `pkg/arch/arm64/tr_special.go`
- [x] In `trADRP()`: emit zero immediate and record relocation
- [x] In `trADR()`: emit zero immediate and record relocation
- [x] Preserve ADRP+ADD optimization with reloc entry

---

## ⏳ Pending Merges (Critical)

### 4. `pkg/binary/elf/packer.go` (MASSIVE)

**Required changes:**

#### A. Structs & Imports
- Add `soName string` to `Packer`
- Add `relocations []arm64.Relocation` to `Packer`
- Add `RuntimeReloc{WritePos, Offset, FuncId}` struct
- Import `"path/filepath"` (already used in fork)

#### B. `FindFunction()`
```go
syms, err := f.Symbols()
if err != nil {
    // Fallback to dynamic symbols for .so
    syms, err = f.DynamicSymbols()
}
```

#### C. `NewPacker()` — no changes needed (fields added to struct)

#### D. In `Process()` — multiple updates:
1. After ELF open:
   ```go
   p.soName = filepath.Base(p.inputPath)
   fmt.Printf("[*] ELF: %s, Type: %s, Name: %s\n", ..., p.soName)
   ```
2. Translator creation:
   ```go
   trans := arm64.NewTranslator(fi.Addr, int(fi.Size), fi.Name)
   ```
3. After `trans.Translate()` (before reversing):
   ```go
   if len(result.Relocations) > 0 {
       p.relocations = append(p.relocations, result.Relocations...)
   }
   ```
4. Add `reverseOffsetMap` to `FuncBytecode`:
   ```go
   type FuncBytecode struct {
       FI               *vm.FuncInfo
       Encrypted        []byte
       XorKey           byte
       reverseOffsetMap map[int]int  // NEW
   }
   ```
5. Store map after `reverseInstructions`:
   ```go
   reversed, offsetMap := reverseInstructions(...)
   // later:
   funcs = append(funcs, FuncBytecode{
       FI: fi,
       Encrypted: encrypted,
       XorKey: xorKey,
       reverseOffsetMap: offsetMap,  // ADD
   })
   ```

#### E. `injectVMPBatch()` — extend Token mode:
After inserting `so_name` info, add:

```go
// Add runtime relocation table if any
if len(p.relocations) > 0 {
    fmt.Printf("    [RELOC] Processing %d relocations...\n", len(p.relocations))

    var runtimeRelocs []RuntimeReloc

    for i, fb := range funcs {
        funcRelocs := p.getRelocationsForFunc(fb.FI.Name)
        for _, reloc := range funcRelocs {
            reOff := uint64(fb.reverseOffsetMap[int(reloc.BcOffset)])
            writePos := reOff - 9  // pointing to the 8-byte operand
            runtimeRelocs = append(runtimeRelocs, RuntimeReloc{
                WritePos: writePos,
                Offset:   reloc.TargetAddr,
                FuncId:   uint64(i),
            })
        }
    }

    table := p.appendRuntimeRelocTable(runtimeRelocs)
    payload = append(payload, table...)

    fmt.Printf("\n重定位表总大小: %d 字节\n", len(table))
}
```

Also **update PT_LOAD size** after adding table:
```go
newPhdr.Filesz = uint64(len(payload))
newPhdr.Memsz = uint64(len(payload))
```

#### F. Add helper methods:
```go
// getRelocationsForFunc returns relocs for a given function name
func (p *Packer) getRelocationsForFunc(funcName string) []arm64.Relocation

// appendRuntimeRelocTable generates binary table:
// [magic:4][count:4][entries...]
// entry: [func_id:8][write_pos:8][offset:8]
func (p *Packer) appendRuntimeRelocTable(relocs []RuntimeReloc) []byte
```

---

### 5. `stub/vm_interp_clean.c` → `stub/linux/arm64/vm_interp.c`

**Option A (clean):** Replace current `vm_interp.c` with fork's `vm_interp_clean.c` content.

**Option B (merge):** Manually apply these diffs:

#### Add syscalls & helpers:
```c
// openat, read, close, mprotect
// enable_text_write(), disable_text_write()
```

#### Global var:
```c
__attribute__((section(".data.entry"), used)) volatile u64 _so_base = 0;
```

#### Change `vm_entry` signature:
```c
u64 vm_entry(u64 *args,
             u64 table_addr, u64 table_num,
             u32 current_func_id,
             u8 *enc_bc, u32 bc_len, u8 xor_key);
```

#### Update `vm_entry_token_inner()`:
```c
u64 table_addr = (u64)table;
u64 table_num = *((u64*)(((u8*)(table)) - 8));
return vm_entry(args, table_addr, table_num, func_id, enc_bc, bc_len, xor_key);
```

#### In `vm_entry()` — after `addr_map` setup:
```c
// 1. Get so name from token_desc_t table
u8 *so_name_info = (u8*)(table_addr + sizeof(token_desc_t) * table_num);
u8 so_name_len = so_name_info[0];
u8 *so_name = so_name_info + 1;

// 2. Find so base if not cached
if(_so_base == 0) {
    // mprotect temporary write
    // _so_base = get_so_base_by_name((char*)so_name);
    // mprotect restore
}

// 3. Parse relocation table (RTLR magic)
u8 *reloc_table_start = (u8*)((u64)so_name_info + so_name_len + 2);
u32 magic = rd32(reloc_table_start);
if (magic == 0x524C5452) { // "RTLR"
    u32 reloc_count = rd32(reloc_table_start + 4);
    u8 *reloc_entry = reloc_table_start + 8;
    for (u32 i = 0; i < reloc_count; i++) {
        u64 func_id = rd64(reloc_entry);
        if (func_id == current_func_id) {
            u64 *write_pos = (u64*)(bc_buf + rd64(reloc_entry + 8));
            u64 offset = rd64(reloc_entry + 16);
            *write_pos = _so_base + offset;
        }
        if (func_id > current_func_id) break;
        reloc_entry += 24;
    }
}
```

#### Add `get_so_base_by_name()`:
Read `/proc/self/maps`, find line with `so_name` and `r-xp` permissions, return start address.

---

### 6. `Makefile` (root)

**Fork version uses portable shell (macOS/Linux).**  
Current main Makefile has Windows PowerShell commands.

**Recommended:** Replace with fork Makefile, but keep Windows support optional.

Option: Create conditional Makefile:
```makefile
# Detect OS
UNAME_S := $(shell uname -s)

ifeq ($(OS),Windows_NT)
    # Windows commands (PowerShell)
else
    # Unix-like (macOS/Linux) commands
endif
```

But fork simplified to Android NDK focus. If Windows GUI not needed, just use fork Makefile.

---

## 🗒️ Notes

- The fork's relocation support (`3caff19`) is the **KILLER FEATURE** for Android `.so` protection.
- Without it, `ADRP/ADR/BL` absolute addresses would be wrong when .so is loaded at runtime.
- Runtime reloc table (`RTLR` magic) allows stub to fix up absolute pointers after `dlopen()`.
- `get_so_base_by_name()` scans `/proc/self/maps` (Linux-specific, but works on Android).

---

## 📌 Merge Order

1. **`translator.go`** (done)
2. **`tr_branch.go`** (done)
3. **`tr_special.go`** (done)
4. **`packer.go`** (next)
5. **`vm_interp.c`** (after packer)
6. **`Makefile`** (last, test build)

---

## 🧪 Testing After Merge

- Build stub: `make stub` (Linux/macOS)
- Build packer: `make packer`
- Run on test `.so` with ADRP/ADR/BL instructions
- Verify:
  - Function protected successfully
  - No crashes when executing protected function
  - `_so_base` correctly resolved
  - Relocations applied (absolute jumps to correct runtime addresses)

---

## 📝 Android Support Status

After merging relocation:
- ✅ Shared object (`.so`) can be protected
- ✅ Runtime base address detection via `/proc/self/maps`
- ✅ ADRP/ADR/BL relocations fixed at runtime
- ⚠️ Still Linux-only (syscalls: `mmap`, `mprotect`, `openat`, `read`, `close`)
- ⚠️ Not yet tested on actual Android device/emulator

Next step after merge: test with simple JNI `.so` library.

---

*Last updated: 2025-04-06 (Kilo)*
