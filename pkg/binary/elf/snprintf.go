package elf

import (
	"debug/elf"
	"encoding/binary"
)

// findSnprintfPLT dynamically finds the PLT stub address for snprintf
func (p *Packer) findSnprintfPLT(f *elf.File) uint64 {
	syms, _ := f.DynamicSymbols()
	if len(syms) == 0 {
		return 0
	}

	// Build symbol index map: Go slice index → ELF .dynsym index
	// Go's DynamicSymbols() includes the null symbol at [0], so slice[i] = .dynsym[i]
	// Iterate .rela.plt entries and resolve symbol names from the index in r_info
	pltRelocs := f.Section(".rela.plt")
	if pltRelocs == nil {
		return 0
	}
	data, _ := pltRelocs.Data()

	var gotOffset uint64
	for i := 0; i+24 <= len(data); i += 24 {
		r_offset := f.ByteOrder.Uint64(data[i : i+8])
		r_info := f.ByteOrder.Uint64(data[i+8 : i+16])
		symIdx := int(r_info >> 32)
		if symIdx < len(syms) && syms[symIdx].Name == "snprintf" {
			gotOffset = r_offset
			break
		}
	}
	if gotOffset == 0 {
		return 0
	}

	// Scan .plt entries for the stub that loads from this GOT slot
	pltSec := f.Section(".plt")
	if pltSec == nil {
		return 0
	}
	pltData, _ := pltSec.Data()
	for i := 0; i+8 <= len(pltData); i += 16 {
		inst1 := binary.LittleEndian.Uint32(pltData[i:])
		inst2 := binary.LittleEndian.Uint32(pltData[i+4:])

		if inst1&0x9F000000 == 0x90000000 { // ADRP
			immhi := (inst1 >> 5) & 0x7FFFF
			immlo := (inst1 >> 29) & 0x3
			imm := (immhi << 2) | immlo
			imm64 := int64(imm) << 12
			if imm64&(1<<32) != 0 {
				imm64 |= -1 << 33
			}
			pc := pltSec.Addr + uint64(i)
			page := pc &^ 0xFFF
			targetPage := uint64(int64(page) + imm64)

			if inst2&0xFFC00000 == 0xF9400000 { // LDR
				imm12 := (inst2 >> 10) & 0xFFF
				if targetPage+uint64(imm12*8) == gotOffset {
					return pc
				}
			} else if inst2&0xFFC00000 == 0x91000000 { // ADD
				imm12 := (inst2 >> 10) & 0xFFF
				if targetPage+uint64(imm12) == gotOffset {
					return pc
				}
			}
		}
	}
	return 0
}
