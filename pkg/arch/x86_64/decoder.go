package x86_64

import (
	"fmt"

	"golang.org/x/arch/x86/x86asm"

	"github.com/vmpacker/pkg/vm"
)

// Decoder for x86_64 architecture
type Decoder struct{}

// NewDecoder creates a new x86_64 decoder
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Decode translates raw bytes into vm.Instruction representation
func (d *Decoder) Decode(code []byte, baseVA uint64) ([]vm.Instruction, error) {
	var insts []vm.Instruction
	var offset int
	
	for offset < len(code) {
		inst, err := x86asm.Decode(code[offset:], 64)
		if err != nil {
			return nil, fmt.Errorf("decode error at offset %X (VA=0x%X): %v", offset, baseVA+uint64(offset), err)
		}

		vmInst := d.mapToVMInstruction(inst, code[offset:offset+inst.Len], baseVA+uint64(offset), offset)
		insts = append(insts, vmInst)
		offset += inst.Len
	}
	
	return insts, nil
}

func (d *Decoder) mapToVMInstruction(xInst x86asm.Inst, raw []byte, va uint64, offset int) vm.Instruction {
	return vm.Instruction{
		Offset:   offset,
		RawBytes: raw,
		Size:     uint32(xInst.Len),
		Str:      x86asm.GNUSyntax(xInst, va, nil), 
		Op:       int(xInst.Op),
	}
}

// InstName returns the instruction name
func (d *Decoder) InstName(op int) string {
	return x86asm.Op(op).String()
}
