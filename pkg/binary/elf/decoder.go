package elf

import (
	"encoding/binary"

	"github.com/vmpacker/pkg/arch/arm32"
	"github.com/vmpacker/pkg/arch/arm64"
	"github.com/vmpacker/pkg/vm"
)

// DecodeFunction decodes ARM64 instructions
func (p *Packer) DecodeFunction(code []byte) []vm.Instruction {
	dec := arm64.NewDecoder()
	var insts []vm.Instruction
	for off := 0; off+4 <= len(code); off += 4 {
		raw := binary.LittleEndian.Uint32(code[off:])
		inst := dec.Decode(raw, off)
		insts = append(insts, inst)
	}
	return insts
}

// DecodeFunctionARM32 decodes ARM32/Thumb instructions
func (p *Packer) DecodeFunctionARM32(code []byte, thumbMode bool) []vm.Instruction {
	var dec *arm32.Decoder
	if thumbMode {
		dec = arm32.NewThumbDecoder()
	} else {
		dec = arm32.NewDecoder()
	}
	var insts []vm.Instruction
	off := 0
	for off < len(code) {
		if thumbMode {
			if off+2 > len(code) {
				break
			}
			hw := binary.LittleEndian.Uint16(code[off:])
			if arm32.IsThumb32(hw) {
				if off+4 > len(code) {
					break
				}
				hw2 := binary.LittleEndian.Uint16(code[off+2:])
				raw32 := (uint32(hw) << 16) | uint32(hw2)
				inst := dec.Decode(raw32, off)
				insts = append(insts, inst)
				off += 4
			} else {
				inst := dec.Decode(uint32(hw), off)
				insts = append(insts, inst)
				off += 2
			}
		} else {
			if off+4 > len(code) {
				break
			}
			raw := binary.LittleEndian.Uint32(code[off:])
			inst := dec.Decode(raw, off)
			insts = append(insts, inst)
			off += 4
		}
	}
	return insts
}
