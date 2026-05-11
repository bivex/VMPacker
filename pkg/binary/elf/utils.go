package elf

import (
	"encoding/binary"
	"fmt"
	"os"

	"debug/elf"

	"github.com/vmpacker/pkg/vm"
)

// NewPacker creates an ELF packer
func NewPacker(input, output string, funcs []string, addrSpecs []AddrSpec, verbose, strip, debug, tokenEntry bool, interpBlob []byte) *Packer {
	return &Packer{
		inputPath:    input,
		outputPath:   output,
		funcNames:    funcs,
		addrSpecs:    addrSpecs,
		verbose:      verbose,
		stripSymbols: strip,
		debug:        debug,
		tokenEntry:   tokenEntry,
		interpBlob:   interpBlob,
	}
}

// SetInterpBlobARM32 sets the ARM32 VM interpreter blob
func (p *Packer) SetInterpBlobARM32(blob []byte) {
	p.interpBlobARM32 = blob
}

// SetInterpBlobX86_64 sets the x86_64 VM interpreter blob
func (p *Packer) SetInterpBlobX86_64(blob []byte) {
	p.interpBlobX86_64 = blob
}

func (p *Packer) SetCFF(enabled bool) {
	p.cff = enabled
}

func (p *Packer) SetMBA(enabled bool) {
	p.mba = enabled
}

func (p *Packer) SetMangle(enabled bool) {
	p.mangleSymbols = enabled
}

// PrintELFInfo prints ELF information
func PrintELFInfo(path string) error {
	f, err := elf.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Printf("ELF: %s\n", path)
	fmt.Printf("  Arch: %s, Type: %s, Class: %s, Entry: 0x%X\n", f.Machine, f.Type, f.Class, f.Entry)

	fmt.Println("\n  Sections:")
	for _, s := range f.Sections {
		if s.Size > 0 {
			fmt.Printf("    %-16s  Addr=0x%08X  Size=0x%X  Off=0x%X\n",
				s.Name, s.Addr, s.Size, s.Offset)
		}
	}

	fmt.Println("\n  Program Headers:")
	raw, _ := os.ReadFile(path)
	if f.Class == elf.ELFCLASS32 && len(raw) >= 52 {
		ehdr := readEhdr32(raw)
		for i := 0; i < int(ehdr.Phnum); i++ {
			ph := readPhdr32(raw, ehdr.Phoff+uint32(i)*uint32(ehdr.Phentsize))
			flags := ""
			if ph.Flags&uint32(elf.PF_R) != 0 {
				flags += "R"
			}
			if ph.Flags&uint32(elf.PF_W) != 0 {
				flags += "W"
			}
			if ph.Flags&uint32(elf.PF_X) != 0 {
				flags += "X"
			}
			fmt.Printf("    [%d] Type=0x%X Flags=%s Off=0x%X VA=0x%X FileSz=0x%X MemSz=0x%X\n",
				i, ph.Type, flags, ph.Off, ph.Vaddr, ph.Filesz, ph.Memsz)
		}
	} else if len(raw) >= 64 {
		ehdr := readEhdr64(raw)
		for i := 0; i < int(ehdr.Phnum); i++ {
			ph := readPhdr64(raw, ehdr.Phoff+uint64(i)*uint64(ehdr.Phentsize))
			flags := ""
			if ph.Flags&uint32(elf.PF_R) != 0 {
				flags += "R"
			}
			if ph.Flags&uint32(elf.PF_W) != 0 {
				flags += "W"
			}
			if ph.Flags&uint32(elf.PF_X) != 0 {
				flags += "X"
			}
			fmt.Printf("    [%d] Type=0x%X Flags=%s Off=0x%X VA=0x%X FileSz=0x%X MemSz=0x%X\n",
				i, ph.Type, flags, ph.Off, ph.Vaddr, ph.Filesz, ph.Memsz)
		}
	}

	fmt.Println("\n  Functions:")
	syms, err := f.Symbols()
	if err != nil {
		fmt.Println("  (no symbol table)")
		return nil
	}
	count := 0
	for _, sym := range syms {
		if elf.ST_TYPE(sym.Info) == elf.STT_FUNC && sym.Size > 0 {
			fmt.Printf("    %-24s  Addr=0x%08X  Size=%d\n",
				sym.Name, sym.Value, sym.Size)
			count++
		}
	}
	fmt.Printf("  Total: %d functions\n", count)
	return nil
}

// branchTargetOffset returns the byte offset of target32 relative to pc in a branch instruction.
// Standard branch: [op(1B)][target32(4B)] = 5B → offset=1
// TBZ/TBNZ: [op(1B)][reg(1B)][bit(1B)][target32(4B)] = 7B → offset=3
// Non-branch instructions return 0.
func branchTargetOffset(op byte) int {
	switch op {
	case vm.OpJmp, vm.OpJe, vm.OpJne, vm.OpJl, vm.OpJge,
		vm.OpJgt, vm.OpJle, vm.OpJb, vm.OpJae, vm.OpJbe, vm.OpJa,
		vm.OpJvs, vm.OpJvc:
		return 1
	case vm.OpTbz, vm.OpTbnz:
		return 3
	}
	return 0
}

// reverseInstructions reverses the instruction order and appends a size marker.
func reverseInstructions(bytecode []byte, codeLen int) ([]byte, map[int]int, map[int]int) {
	type instInfo struct {
		offset int
		size   int
	}
	var insts []instInfo
	pc := 0
	for pc < codeLen {
		op := bytecode[pc]
		sz := vm.InstructionSize(op)
		if sz == 0 {
			sz = 1
		}
		if pc+sz > codeLen {
			break
		}
		insts = append(insts, instInfo{offset: pc, size: sz})
		pc += sz
	}

	offsetMap := make(map[int]int)
	byteMap := make(map[int]int)
	var output []byte
	for i := len(insts) - 1; i >= 0; i-- {
		inst := insts[i]
		newInstStart := len(output)
		output = append(output, bytecode[inst.offset:inst.offset+inst.size]...)
		// Record the position of the size marker (right after instruction bytes)
		offsetMap[inst.offset] = len(output)
		output = append(output, byte(inst.size))

		// Record per-byte mapping for relocations
		for k := 0; k < inst.size; k++ {
			byteMap[inst.offset+k] = newInstStart + k
		}
	}

	// Append a trailing NOP so that no remapped branch target equals bc_len.
	// Without this, branches targeting the first original instruction would have
	// offsetMap[0]+1 == len(output) == bc_len, which fails BRANCH_TARGET_VALID(t < bc_len)
	// in the C VM, suppressing the branch and potentially creating infinite cycles
	// in nested loops. The C VM executes this NOP harmlessly at startup.
	if len(output) > 0 {
		output = append(output, vm.OpNop)
		output = append(output, byte(1)) // size marker for the NOP
	}

	return output, offsetMap, byteMap
}

// remapBranchTargets remaps branch targets in reversed bytecode.
//
// Scans reversed bytecode to find all branch instructions,
// replaces target32 from old offset to new offset (using offsetMap).
func remapBranchTargets(bytecode []byte, codeLen int, offsetMap map[int]int, verbose bool) {
	pc := 0
	for pc < codeLen {
		op := bytecode[pc]
		sz := vm.InstructionSize(op)
		if sz == 0 {
			sz = 1
		}
		if toff := branchTargetOffset(op); toff > 0 && pc+toff+4 <= codeLen {
			oldTarget := binary.LittleEndian.Uint32(bytecode[pc+toff:])
			if newTarget, ok := offsetMap[int(oldTarget)]; ok {
				// +1: offsetMap gives size-marker position; reverse-mode
				// dispatch does pc-- first, so target must be past the marker.
				remapped := uint32(newTarget + 1)
				if verbose {
					fmt.Printf("      [REMAP] pc=0x%04X op=0x%02X target: 0x%04X -> 0x%04X\n",
						pc, op, oldTarget, remapped)
				}
				binary.LittleEndian.PutUint32(bytecode[pc+toff:], remapped)
			} else {
				fmt.Printf("      [REMAP] pc=0x%04X op=0x%02X target: 0x%04X -> NOT FOUND! (offsetMap size=%d)\n",
					pc, op, oldTarget, len(offsetMap))
			}
		}
		// Skip instruction + size marker (every instruction has a 1B size marker after reversal)
		pc += sz + 1
	}
}

// encryptOpcodes encrypts the opcode byte of each instruction (OpcodeCryptor).
//
// When reversed=true, traverses backward using embedded size markers — exactly
// matching the C dispatch loop — so positions always align regardless of
// vm.InstructionSize quirks.
//
// Encryption formula: encrypted_opcode[pc] = opcode[pc] ^ (u8)(ocKey ^ (pc * 0x9E3779B9))
func encryptOpcodes(bytecode []byte, codeLen int, ocKey uint32, reversed bool) {
	if reversed {
		// Backward traversal using size markers, mirrors C dispatch:
		//   pc--; sz = bc[pc]; pc -= sz;   → opcode at pc
		pc := codeLen
		for pc > 0 {
			pc--
			sz := int(bytecode[pc])
			pc -= sz
			if pc < 0 {
				break
			}
			mask := byte(ocKey ^ (uint32(pc) * 0x9E3779B9))
			bytecode[pc] ^= mask
		}
		return
	}

	// Forward traversal (non-reversed mode)
	pc := 0
	for pc < codeLen {
		op := bytecode[pc]
		size := vm.InstructionSize(op)
		if size == 0 {
			pc++
			continue
		}
		mask := byte(ocKey ^ (uint32(pc) * 0x9E3779B9))
		bytecode[pc] = op ^ mask
		pc += size
	}
}
