//go:build ignore

package main

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
)

func main() {
	f, err := elf.Open("test/android/build/libnative_test_arm64.so")
	if err != nil { panic(err) }
	defer f.Close()

	syms, _ := f.DynamicSymbols()
	snprintfSymIdx := -1
	for i, sym := range syms {
		if sym.Name == "snprintf" || sym.Name == "__snprintf_chk" {
			snprintfSymIdx = i
			break
		}
	}

	var gotOffset uint64
	pltRelocs := f.Section(".rela.plt")
	if pltRelocs != nil {
		data, _ := pltRelocs.Data()
		for i := 0; i < len(data); i += 24 {
			r_offset := f.ByteOrder.Uint64(data[i:i+8])
			r_info := f.ByteOrder.Uint64(data[i+8:i+16])
			symIdx := r_info >> 32
			if int(symIdx) == snprintfSymIdx {
				gotOffset = r_offset
				break
			}
		}
	}

	fmt.Printf("snprintf GOT offset: 0x%X\n", gotOffset)

	// Now scan .plt to find the stub
	pltSec := f.Section(".plt")
	if pltSec != nil {
		data, _ := pltSec.Data()
		for i := 0; i < len(data); i += 16 { // typically 16 bytes per PLT entry
			if i+8 > len(data) { break }
			
			inst1 := binary.LittleEndian.Uint32(data[i:])
			inst2 := binary.LittleEndian.Uint32(data[i+4:])
			
			// ADRP
			if inst1&0x9F000000 == 0x90000000 {
				immhi := (inst1 >> 5) & 0x7FFFF
				immlo := (inst1 >> 29) & 0x3
				imm := (immhi << 2) | immlo
				imm64 := int64(imm) << 12
				// sign extend from 21 bits
				if imm64 & (1<<32) != 0 {
					imm64 |= -1 << 33
				}
				
				pc := pltSec.Addr + uint64(i)
				page := pc &^ 0xFFF
				targetPage := uint64(int64(page) + imm64)
				
				// LDR or ADD
				if inst2&0xFFC00000 == 0xF9400000 { // LDR (immediate)
					imm12 := (inst2 >> 10) & 0xFFF
					target := targetPage + uint64(imm12*8) // 8 byte scale for 64-bit
					if target == gotOffset {
						fmt.Printf("Found PLT stub for snprintf at 0x%X\n", pc)
					}
				} else if inst2&0xFFC00000 == 0x91000000 { // ADD (immediate)
					imm12 := (inst2 >> 10) & 0xFFF
					target := targetPage + uint64(imm12)
					if target == gotOffset {
						fmt.Printf("Found PLT stub for snprintf at 0x%X\n", pc)
					}
				}
			}
		}
	}
}