# VMPacker Code Analysis - Key Findings

**Date:** 2026-05-03
**Tool:** Go AST Analyzer (expert-octo-sniffle)
**Scope:** 62 files, 502 functions
**Total Issues:** 157 (89 high severity)

---

## Refactoring Progress

| Target | Before | After | Status |
|--------|--------|-------|--------|
| `packer.go` Process() | 1 file, cyclomatic=58, 377 lines | Split into 8 files (finder, inject, process, strip, trampoline, types, utils, decoder) | **DONE** (d777970) |
| `process.go` Process() | cyclomatic=58, cognitive=1807, 377 lines | cyclomatic=18, cognitive=161, 106 lines | **DONE** — extracted validateArch, collectEntries, translateFunction, dumpDisasm, writeUnsupportedReport, writeDebugDump, postProcessBytecode |
| `tr_stack.go` | 1 file, 1986 lines, 49 functions | 6 files: tr_stack.go (381), tr_stack_load.go (299), tr_stack_store.go (202), tr_stack_pair.go (400), tr_stack_bitfield.go (197), tr_stack_misc.go (550) | **DONE** (f87b39b) |

---

## Remaining Hotspots

| Function | File | Cyclomatic | Cognitive | Lines |
|----------|------|-------------|-----------|-------|
| `translateOne` (arm64) | `pkg/arch/arm64/translator.go:243` | 24 | 325 | 389 |
| `FindFunctionByAddr` | `pkg/binary/elf/finder.go:112` | 26 | 499 | 127 |
| `injectVMPBatch64` | `pkg/binary/elf/inject.go:21` | 33 | 584 | 248 |
| `injectVMPBatch32` | `pkg/binary/elf/inject.go:271` | 31 | 521 | 222 |
| `decodeThumb32DPModImm` | `pkg/arch/arm32/decode_thumb32.go:67` | 24 | 479 | 100 |
| `decodeThumb32DPShifted` | `pkg/arch/arm32/decode_thumb32.go:246` | 24 | 479 | 109 |
| `decodeThumb32LdStSingle` | `pkg/arch/arm32/decode_thumb32.go:502` | 24 | 475 | 117 |
| `trStackLoad` | `pkg/arch/arm64/tr_stack_load.go:11` | 23 | 378 | 138 |
| `stripSections64` | `pkg/binary/elf/strip.go:19` | 22 | 212 | 116 |

---

## Code Smells - Structs with Too Many Fields

| Struct | File | Fields | Max Recommended |
|--------|------|--------|-----------------|
| `Packer` | `pkg/binary/elf/types.go:23` | 14 | 10 |
| `Translator` (arm32) | `pkg/arch/arm32/translator.go:30` | 12 | 10 |
| `Instruction` | `pkg/vm/types.go:20` | 12 | 10 |

---

## Code Smells - Oversized Functions (80+ lines)

| Function | File | Lines |
|----------|------|-------|
| `translateOne` (arm64) | `pkg/arch/arm64/translator.go:243` | 389 |
| `injectVMPBatch64` | `pkg/binary/elf/inject.go:21` | 248 |
| `translateOne` (arm32) | `pkg/arch/arm32/translator.go:319` | 241 |
| `injectVMPBatch32` | `pkg/binary/elf/inject.go:271` | 222 |
| `Process` | `pkg/binary/elf/process.go:27` | 106 |
| `trStackLoad` | `pkg/arch/arm64/tr_stack_load.go:11` | 138 |
| `FindFunctionByAddr` | `pkg/binary/elf/finder.go:112` | 127 |
| `DisasmOne` | `pkg/vm/disasm.go:200` | 126 |
| `trCondLoad` | `pkg/arch/arm32/tr_loadstore.go:74` | 124 |
| `stripSections64` | `pkg/binary/elf/strip.go:19` | 116 |
| `decodeThumb32LdStSingle` | `pkg/arch/arm32/decode_thumb32.go:502` | 117 |

---

## Concurrency Bug Issues

| Severity | Function | File | Issue |
|----------|----------|------|-------|
| critical | `testDeadlockPotential` | `expert-octo-sniffle/concurrency_test.go:92` | Deadlock pattern - mutex 'mu' usage may cause deadlock |
| warning | `properGoroutineCleanup` | `expert-octo-sniffle/concurrency_correct.go:157` | Select without default/timeout may block |
| warning | `testSelectStatementLeak` | `expert-octo-sniffle/concurrency_test.go:24` | Select without default/timeout may block |
| warning | `testBlockingBug` | `expert-octo-sniffle/concurrency_test.go:61` | Channel 'ch' operations may cause blocking |

---

## Notes

- Low-level binary translation code naturally has high complexity due to instruction decoding switch/if chains
- The analyzer's own code also shows issues (expected for a research/analysis tool)
- ARM translator files may need architectural changes (table-driven dispatch) rather than simple extraction
- `translateOne` is a pure dispatch switch (97 cases, each 1-2 lines delegating to helpers) — refactoring adds indirection without reducing real complexity
