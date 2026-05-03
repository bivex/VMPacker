package elf

import (
	"bytes"
	"encoding/binary"
	"io"
)

// ============================================================
// Tokenized Entry Trampoline (ARM64 / ARM32 / Thumb)
// ============================================================

// BuildTokenTrampoline constructs a tokenized entry trampoline (3 ARM64 instructions, 12 bytes)
//
//	MOV  W16, #token_lo16          ; lower 16 bits of token → W16
//	MOVK W16, #token_hi16, LSL#16  ; upper 16 bits of token merged
//	B    vm_entry_token             ; Jump to Token entry
//
// X16 (IP0) passes the token, X0-X7 keep the original caller arguments unchanged.
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
// ELF64 binary structure read/write
// ============================================================

type elf64Ehdr struct {
	Phoff     uint64
	Shoff     uint64
	Phentsize uint16
	Phnum     uint16
	Shentsize uint16
	Shnum     uint16
	Shstrndx  uint16
}

func readEhdr64(d []byte) elf64Ehdr {
	return elf64Ehdr{
		Phoff:     binary.LittleEndian.Uint64(d[0x20:]),
		Shoff:     binary.LittleEndian.Uint64(d[0x28:]),
		Phentsize: binary.LittleEndian.Uint16(d[0x36:]),
		Phnum:     binary.LittleEndian.Uint16(d[0x38:]),
		Shentsize: binary.LittleEndian.Uint16(d[0x3A:]),
		Shnum:     binary.LittleEndian.Uint16(d[0x3C:]),
		Shstrndx:  binary.LittleEndian.Uint16(d[0x3E:]),
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

func BuildTokenTrampolineARM32(funcAddr, vmEntryTokenVA uint32, token uint32) []byte {
	var buf bytes.Buffer
	lo16 := token & 0xFFFF
	hi16 := (token >> 16) & 0xFFFF
	imm4Lo := (lo16 >> 12) & 0xF
	imm12Lo := lo16 & 0xFFF
	writeU32(&buf, 0xE300C000|uint32(imm4Lo)<<16|uint32(imm12Lo))
	imm4Hi := (hi16 >> 12) & 0xF
	imm12Hi := hi16 & 0xFFF
	writeU32(&buf, 0xE340C000|uint32(imm4Hi)<<16|uint32(imm12Hi))
	bPC := funcAddr + 8 + 8
	bOffset := int32(vmEntryTokenVA) - int32(bPC)
	bImm24 := (bOffset >> 2) & 0x00FFFFFF
	writeU32(&buf, 0xEA000000|uint32(bImm24))
	return buf.Bytes()
}

func BuildTokenTrampolineThumb(funcAddr, vmEntryTokenVA uint32, token uint32) []byte {
	var buf bytes.Buffer
	lo16 := token & 0xFFFF
	hi16 := (token >> 16) & 0xFFFF
	writeThumb32MovW(&buf, 12, lo16)
	writeThumb32MovT(&buf, 12, hi16)
	bPC := funcAddr + 8 + 4
	bOffset := int32(vmEntryTokenVA) - int32(bPC)
	writeThumb32BranchW(&buf, bOffset)
	return buf.Bytes()
}

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

func writeThumb32BranchW(w io.Writer, offset int32) {
	imm := offset >> 1
	s := uint16(0)
	if imm < 0 { s = 1 }
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
// ELF32 binary structure read/write
// ============================================================

type elf32Ehdr struct {
	Phoff     uint32
	Shoff     uint32
	Phentsize uint16
	Phnum     uint16
	Shentsize uint16
	Shnum     uint16
	Shstrndx  uint16
}

func readEhdr32(d []byte) elf32Ehdr {
	return elf32Ehdr{
		Phoff:     binary.LittleEndian.Uint32(d[0x1C:]),
		Shoff:     binary.LittleEndian.Uint32(d[0x20:]),
		Phentsize: binary.LittleEndian.Uint16(d[0x2A:]),
		Phnum:     binary.LittleEndian.Uint16(d[0x2C:]),
		Shentsize: binary.LittleEndian.Uint16(d[0x2E:]),
		Shnum:     binary.LittleEndian.Uint16(d[0x30:]),
		Shstrndx:  binary.LittleEndian.Uint16(d[0x32:]),
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
