# VM ISA Reference

VMPacker defines a custom Instruction Set Architecture with **randomly mapped opcode values** to increase reverse-engineering difficulty.

## Instruction Categories

| Category | Count | Opcodes |
|----------|-------|---------|
| Data Movement | 3 | `MOV_IMM64`, `MOV_IMM32`, `MOV_REG` |
| Arithmetic/Logic | 21 | `ADD`, `SUB`, `MUL`, `XOR`, `AND`, `OR`, `SHL`, `SHR`, `ASR`, `NOT`, `ROR`, `UMULH` + `_IMM` variants |
| Memory Access | 8 | `LOAD`/`STORE` 8/16/32/64-bit |
| Branch/Jump | 13 | `JMP`, `JE`, `JNE`, `JL`, `JGE`, `JGT`, `JLE`, `JB`, `JAE`, `JBE`, `JA`, `TBZ`, `TBNZ` |
| Compare | 6 | `CMP`, `CMP_IMM`, `CCMP_REG`, `CCMP_IMM`, `CCMN_REG`, `CCMN_IMM` |
| Stack | 10 | `S_PUSH_IMM32`, `S_PUSH_IMM64`, `S_VLOAD`, `S_VSTORE`, `S_DUP`, `S_SWAP`, `S_DROP`, `S_CMP`, `S_TRUNC32`, `S_SEXT32` |
| System/Special | 8 | `NOP`, `HALT`, `RET`, `CALL_NATIVE`, `CALL_REG`, `BR_REG`, `SVC`, `MRS` |
| SIMD | 2 | `VLD16`, `VST16` |
| **Total** | **63+** | |

## Stack-Based Operations

Most ARM64 instructions are translated into stack-based sequences:

```
ARM64:  ADD X0, X1, X2
VM:     S_VLOAD(X1)  S_VLOAD(X2)  S_ADD  S_VSTORE(X0)
```

```
ARM64:  LDR X0, [X1, #8]
VM:     S_VLOAD(X1)  S_PUSH_IMM(8)  S_ADD  S_LD64  S_VSTORE(X0)
```

## Opcode Encryption (OpcodeCryptor)

Each opcode is XOR-encrypted with a position-dependent key:

```
enc[pc] = op[pc] ^ (key ^ (pc * 0x9E3779B9))
```

The key is randomly generated per protection run and stored in the bytecode trailer.

## Bytecode Reversal

Instructions are stored in reverse execution order with size markers. The interpreter traverses backwards through the bytecode, making linear disassembly impossible.

## RTLR (Runtime Relocation)

For PIE/ET_DYN binaries (Android .so), absolute addresses need ASLR fixup at runtime. The RTLR table is embedded in the bytecode payload:

```
[Magic: "RTLR"][Count: u32][{func_id: u64, bc_off: u64, target_addr: u64}...]
```

At runtime, each relocation entry patches the bytecode with `target_addr + slide`.

## Supported ARM64 Instructions

### Arithmetic/Logic
`ADD`, `SUB`, `MUL`, `AND`, `ORR`, `EOR`, `LSL`, `LSR`, `ASR`, `MVN`, `BIC`, `ORN`, `EON`, `ADD_IMM`, `SUB_IMM`, `NEG`, `ROR`

### Multiply-Extend
`MADD`, `MSUB`, `SMADDL`, `SMSUBL`, `UMADDL`, `UMSUBL`, `SMULH`, `UMULH`, `UDIV`, `SDIV`

### Data Movement
`MOV` (register), `MOVZ`, `MOVK`, `MOVN`

### Memory Access
`LDR`, `STR`, `LDP`, `STP`, `LDPSW`, `LDADD`, `CAS` — various widths (B/H/W/X), addressing modes (imm/reg/extended/pre-index/post-index), plus acquire/release variants (`LDAR`, `STLR`, `LDAXR`, `STLXR`)

### Branch
`B`, `BL`, `BR`, `BLR`, `RET`, `B.cond`, `CBZ`, `CBNZ`, `TBZ`, `TBNZ`

### Conditional
`CSEL`, `CSINC`, `CSINV`, `CSNEG`, `CCMP`, `CCMN`

### Bitfield
`UBFM` (LSR/LSL/UXTB/UXTH/UBFX/UBFIZ), `SBFM`, `BFM`, `EXTR`

### System
`SVC`, `MRS`, `MSR`, `ADRP`, `ADR`, `DMB`, `DSB`, `ISB`, `HLT`, `BRK`, `PRFM`, `NOP`

### SIMD
`LD1`, `ST1` (16-bit)
