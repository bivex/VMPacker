package elf

import (
	"bytes"
	"encoding/binary"
	"io"
)

// ============================================================
// ARM64 跳板代码生成 + ELF64 二进制结构读写
// ============================================================

// BuildTrampoline 构造 ARM64 跳板代码（动态参数传递）
//
//	MOV X9, X29                  ; 暂存 caller FP
//	MOV X10, X30                 ; 暂存 caller LR
//	STP X29, X30, [SP, #-96]!   ; 保存 FP/LR + 分配 96B 栈帧
//	MOV X29, SP                  ; 建立栈帧
//	STP X0, X1, [SP, #16]       ; args[0..1]
//	STP X2, X3, [SP, #32]       ; args[2..3]
//	STP X4, X5, [SP, #48]       ; args[4..5]
//	STP X6, X7, [SP, #64]       ; args[6..7]
//	STP X9, X10, [SP, #80]      ; args[8]=callerFP, args[9]=callerLR
//	ADD X0, SP, #16              ; X0 = args 指针 (10 个 u64)
//	MOV X1, bcVA                 ; 加密字节码地址
//	MOV X2, bcLen                ; 字节码长度
//	MOV X3, xorKey               ; XOR 密钥
//	BL  interpVA                 ; 调用 VM 解释器
//	LDP X29, X30, [SP], #96     ; 恢复 FP/LR + 释放栈帧
//	RET                          ; 返回 (结果在 X0)
/* STANDARD_MODE_DISABLED: BuildTrampoline 已禁用，只保留 BuildTokenTrampoline
func BuildTrampoline(funcAddr, interpVA, bcVA uint64, bcLen uint32, xorKey byte) []byte {
	var buf bytes.Buffer

	// MOV X9, X29 (暂存 caller FP)
	writeU32(&buf, 0xAA1D03E9)

	// MOV X10, X30 (暂存 caller LR)
	writeU32(&buf, 0xAA1E03EA)

	// STP X29, X30, [SP, #-96]!
	writeU32(&buf, 0xA9BA7BFD)

	// MOV X29, SP
	writeU32(&buf, 0x910003FD)

	// STP X0, X1, [SP, #16]
	writeU32(&buf, 0xA90107E0)

	// STP X2, X3, [SP, #32]
	writeU32(&buf, 0xA9020FE2)

	// STP X4, X5, [SP, #48]
	writeU32(&buf, 0xA90317E4)

	// STP X6, X7, [SP, #64]
	writeU32(&buf, 0xA9041FE6)

	// STP X9, X10, [SP, #80]
	writeU32(&buf, 0xA9052BE9)

	// ADD X0, SP, #16 (X0 = args 指针)
	writeU32(&buf, 0x910043E0)

	// Load bcVA into X1
	writeARM64MovZ(&buf, 1, uint16(bcVA&0xFFFF), 0)
	writeARM64MovK(&buf, 1, uint16((bcVA>>16)&0xFFFF), 1)
	writeARM64MovK(&buf, 1, uint16((bcVA>>32)&0xFFFF), 2)

	// Load bcLen into X2
	writeARM64MovZ(&buf, 2, uint16(bcLen&0xFFFF), 0)
	if bcLen > 0xFFFF {
		writeARM64MovK(&buf, 2, uint16((bcLen>>16)&0xFFFF), 1)
	}

	// Load xorKey into X3
	writeARM64MovZ(&buf, 3, uint16(xorKey), 0)

	// BL interpVA
	blPC := funcAddr + uint64(buf.Len())
	blOffset := int64(interpVA) - int64(blPC)
	blImm26 := (blOffset >> 2) & 0x03FFFFFF
	blInst := uint32(0x94000000) | uint32(blImm26)
	writeU32(&buf, blInst)

	// LDP X29, X30, [SP], #96
	writeU32(&buf, 0xA8C67BFD)

	// RET
	writeU32(&buf, 0xD65F03C0)

	return buf.Bytes()
}
STANDARD_MODE_DISABLED */

// BuildTokenTrampoline 构造 Token 化入口跳板（3 条 ARM64 指令, 12 字节）
//
//	MOV  W16, #token_lo16          ; token 低 16 位 → W16
//	MOVK W16, #token_hi16, LSL#16  ; token 高 16 位合并
//	B    vm_entry_token             ; 跳转到 Token 入口
//
// X16 (IP0) 传递 token，X0-X7 保持调用方原始参数不变。
func BuildTokenTrampoline(funcAddr, vmEntryTokenVA uint64, token uint32) []byte {
	var buf bytes.Buffer

	// MOV W16, #token_lo16  (MOVZ W16, sf=0, hw=0)
	lo16 := token & 0xFFFF
	writeU32(&buf, 0x52800010|uint32(lo16)<<5)

	// MOVK W16, #token_hi16, LSL#16  (MOVK W16, sf=0, hw=1)
	hi16 := (token >> 16) & 0xFFFF
	writeU32(&buf, 0x72A00010|uint32(hi16)<<5)

	// B vm_entry_token  (PC = funcAddr + 8)
	bPC := funcAddr + 8
	bOffset := int64(vmEntryTokenVA) - int64(bPC)
	bImm26 := (bOffset >> 2) & 0x03FFFFFF
	writeU32(&buf, 0x14000000|uint32(bImm26))

	return buf.Bytes()
}

// ============================================================
// ELF64 二进制结构读写
// ============================================================

type elf64Ehdr struct {
	Phoff     uint64
	Shoff     uint64
	Phentsize uint16
	Phnum     uint16
	Shentsize uint16
	Shnum     uint16
}

func readEhdr64(d []byte) elf64Ehdr {
	return elf64Ehdr{
		Phoff:     binary.LittleEndian.Uint64(d[0x20:]),
		Shoff:     binary.LittleEndian.Uint64(d[0x28:]),
		Phentsize: binary.LittleEndian.Uint16(d[0x36:]),
		Phnum:     binary.LittleEndian.Uint16(d[0x38:]),
		Shentsize: binary.LittleEndian.Uint16(d[0x3A:]),
		Shnum:     binary.LittleEndian.Uint16(d[0x3C:]),
	}
}

type elf64Phdr struct {
	Type   uint32
	Flags  uint32
	Off    uint64
	Vaddr  uint64
	Paddr  uint64
	Filesz uint64
	Memsz  uint64
	Align  uint64
}

func readPhdr64(d []byte, off uint64) elf64Phdr {
	return elf64Phdr{
		Type:   binary.LittleEndian.Uint32(d[off:]),
		Flags:  binary.LittleEndian.Uint32(d[off+4:]),
		Off:    binary.LittleEndian.Uint64(d[off+8:]),
		Vaddr:  binary.LittleEndian.Uint64(d[off+16:]),
		Paddr:  binary.LittleEndian.Uint64(d[off+24:]),
		Filesz: binary.LittleEndian.Uint64(d[off+32:]),
		Memsz:  binary.LittleEndian.Uint64(d[off+40:]),
		Align:  binary.LittleEndian.Uint64(d[off+48:]),
	}
}

func writePhdr64(d []byte, off uint64, ph elf64Phdr) {
	binary.LittleEndian.PutUint32(d[off:], ph.Type)
	binary.LittleEndian.PutUint32(d[off+4:], ph.Flags)
	binary.LittleEndian.PutUint64(d[off+8:], ph.Off)
	binary.LittleEndian.PutUint64(d[off+16:], ph.Vaddr)
	binary.LittleEndian.PutUint64(d[off+24:], ph.Paddr)
	binary.LittleEndian.PutUint64(d[off+32:], ph.Filesz)
	binary.LittleEndian.PutUint64(d[off+40:], ph.Memsz)
	binary.LittleEndian.PutUint64(d[off+48:], ph.Align)
}

// ============================================================
// ARM32 + Thumb Token Trampolines
// ============================================================

// BuildTokenTrampolineARM32 constructs an ARM32 (A32) token trampoline (12 bytes, 3 instructions).
//
//	MOVW R12, #token_lo16     ; R12 = token low 16 bits
//	MOVT R12, #token_hi16     ; R12 |= token high 16 bits << 16
//	B    vm_entry_token        ; branch to VM entry
//
// R12 (IP) is the intra-procedure scratch register (equivalent to ARM64 X16/IP0).
func BuildTokenTrampolineARM32(funcAddr, vmEntryTokenVA uint32, token uint32) []byte {
	var buf bytes.Buffer

	lo16 := token & 0xFFFF
	hi16 := (token >> 16) & 0xFFFF

	// MOVW R12, #lo16: cond=AL(0xE), 0011:0000:imm4:Rd:imm12
	// encoding: 0xE300C000 | (imm4 << 16) | imm12
	imm4Lo := (lo16 >> 12) & 0xF
	imm12Lo := lo16 & 0xFFF
	writeU32(&buf, 0xE300C000|uint32(imm4Lo)<<16|uint32(imm12Lo))

	// MOVT R12, #hi16: 0xE340C000 | (imm4 << 16) | imm12
	imm4Hi := (hi16 >> 12) & 0xF
	imm12Hi := hi16 & 0xFFF
	writeU32(&buf, 0xE340C000|uint32(imm4Hi)<<16|uint32(imm12Hi))

	// B vm_entry_token: cond=AL, 1010:imm24
	// B instruction is at funcAddr+8. ARM32 pipeline: PC = inst_addr + 8.
	bPC := funcAddr + 8 + 8 // B inst addr + ARM pipeline offset
	bOffset := int32(vmEntryTokenVA) - int32(bPC)
	bImm24 := (bOffset >> 2) & 0x00FFFFFF
	writeU32(&buf, 0xEA000000|uint32(bImm24))

	return buf.Bytes()
}

// BuildTokenTrampolineThumb constructs a Thumb-2 token trampoline (12 bytes, 3 wide instructions).
//
//	MOVW R12, #token_lo16     ; Thumb-2 encoding
//	MOVT R12, #token_hi16     ; Thumb-2 encoding
//	B.W  vm_entry_token        ; Thumb-2 branch
func BuildTokenTrampolineThumb(funcAddr, vmEntryTokenVA uint32, token uint32) []byte {
	var buf bytes.Buffer

	lo16 := token & 0xFFFF
	hi16 := (token >> 16) & 0xFFFF

	// Thumb-2 MOVW R12, #lo16
	// Encoding: 11110:i:10:0100:imm4 || 0:imm3:Rd:imm8
	writeThumb32MovW(&buf, 12, lo16)

	// Thumb-2 MOVT R12, #hi16
	// Encoding: 11110:i:10:1100:imm4 || 0:imm3:Rd:imm8
	writeThumb32MovT(&buf, 12, hi16)

	// Thumb-2 B.W: 11110:S:imm10 || 10:J1:1:J2:imm11
	// B.W instruction is at funcAddr+8. Thumb PC = inst_addr + 4.
	bPC := funcAddr + 8 + 4 // B.W inst addr + Thumb pipeline offset
	bOffset := int32(vmEntryTokenVA) - int32(bPC)
	writeThumb32BranchW(&buf, bOffset)

	return buf.Bytes()
}

// writeThumb32MovW writes a Thumb-2 MOVW instruction
func writeThumb32MovW(w io.Writer, rd int, imm16 uint32) {
	imm4 := (imm16 >> 12) & 0xF
	i := (imm16 >> 11) & 1
	imm3 := (imm16 >> 8) & 0x7
	imm8 := imm16 & 0xFF

	hw1 := uint16(0xF240) | uint16(i<<10) | uint16(imm4)
	hw2 := uint16(imm3<<12) | uint16(rd<<8) | uint16(imm8)

	b := make([]byte, 4)
	binary.LittleEndian.PutUint16(b[0:], hw1)
	binary.LittleEndian.PutUint16(b[2:], hw2)
	w.Write(b)
}

// writeThumb32MovT writes a Thumb-2 MOVT instruction
func writeThumb32MovT(w io.Writer, rd int, imm16 uint32) {
	imm4 := (imm16 >> 12) & 0xF
	i := (imm16 >> 11) & 1
	imm3 := (imm16 >> 8) & 0x7
	imm8 := imm16 & 0xFF

	hw1 := uint16(0xF2C0) | uint16(i<<10) | uint16(imm4)
	hw2 := uint16(imm3<<12) | uint16(rd<<8) | uint16(imm8)

	b := make([]byte, 4)
	binary.LittleEndian.PutUint16(b[0:], hw1)
	binary.LittleEndian.PutUint16(b[2:], hw2)
	w.Write(b)
}

// writeThumb32BranchW writes a Thumb-2 B.W (unconditional branch) instruction
func writeThumb32BranchW(w io.Writer, offset int32) {
	// offset is PC-relative; already accounts for pipeline
	imm := offset >> 1 // Thumb branch offset is in halfwords
	s := uint16(0)
	if imm < 0 {
		s = 1
	}
	imm10 := uint16((imm >> 11) & 0x3FF)
	imm11 := uint16(imm & 0x7FF)
	j1 := uint16((^(uint16(imm>>22) ^ s)) & 1)
	j2 := uint16((^(uint16(imm>>21) ^ s)) & 1)

	hw1 := uint16(0xF000) | (s << 10) | imm10
	hw2 := uint16(0x9000) | (j1 << 13) | (j2 << 11) | imm11

	b := make([]byte, 4)
	binary.LittleEndian.PutUint16(b[0:], hw1)
	binary.LittleEndian.PutUint16(b[2:], hw2)
	w.Write(b)
}

// ============================================================
// ELF32 二进制结构读写
// ============================================================

type elf32Ehdr struct {
	Phoff     uint32
	Shoff     uint32
	Phentsize uint16
	Phnum     uint16
	Shentsize uint16
	Shnum     uint16
}

func readEhdr32(d []byte) elf32Ehdr {
	return elf32Ehdr{
		Phoff:     binary.LittleEndian.Uint32(d[0x1C:]),
		Shoff:     binary.LittleEndian.Uint32(d[0x20:]),
		Phentsize: binary.LittleEndian.Uint16(d[0x2A:]),
		Phnum:     binary.LittleEndian.Uint16(d[0x2C:]),
		Shentsize: binary.LittleEndian.Uint16(d[0x2E:]),
		Shnum:     binary.LittleEndian.Uint16(d[0x30:]),
	}
}

type elf32Phdr struct {
	Type   uint32
	Off    uint32
	Vaddr  uint32
	Paddr  uint32
	Filesz uint32
	Memsz  uint32
	Flags  uint32
	Align  uint32
}

func readPhdr32(d []byte, off uint32) elf32Phdr {
	return elf32Phdr{
		Type:   binary.LittleEndian.Uint32(d[off:]),
		Off:    binary.LittleEndian.Uint32(d[off+4:]),
		Vaddr:  binary.LittleEndian.Uint32(d[off+8:]),
		Paddr:  binary.LittleEndian.Uint32(d[off+12:]),
		Filesz: binary.LittleEndian.Uint32(d[off+16:]),
		Memsz:  binary.LittleEndian.Uint32(d[off+20:]),
		Flags:  binary.LittleEndian.Uint32(d[off+24:]),
		Align:  binary.LittleEndian.Uint32(d[off+28:]),
	}
}

func writePhdr32(d []byte, off uint32, ph elf32Phdr) {
	binary.LittleEndian.PutUint32(d[off:], ph.Type)
	binary.LittleEndian.PutUint32(d[off+4:], ph.Off)
	binary.LittleEndian.PutUint32(d[off+8:], ph.Vaddr)
	binary.LittleEndian.PutUint32(d[off+12:], ph.Paddr)
	binary.LittleEndian.PutUint32(d[off+16:], ph.Filesz)
	binary.LittleEndian.PutUint32(d[off+20:], ph.Memsz)
	binary.LittleEndian.PutUint32(d[off+24:], ph.Flags)
	binary.LittleEndian.PutUint32(d[off+28:], ph.Align)
}

// ============================================================
// ARM64 指令编码辅助
// ============================================================

func writeARM64MovZ(w io.Writer, rd int, imm16 uint16, hw int) {
	inst := uint32(0xD2800000) | (uint32(hw) << 21) | (uint32(imm16) << 5) | uint32(rd)
	writeU32(w, inst)
}

func writeARM64MovK(w io.Writer, rd int, imm16 uint16, hw int) {
	inst := uint32(0xF2800000) | (uint32(hw) << 21) | (uint32(imm16) << 5) | uint32(rd)
	writeU32(w, inst)
}

func writeU32(w io.Writer, v uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	w.Write(b)
}
