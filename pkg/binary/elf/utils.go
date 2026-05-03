package elf

import (
	"encoding/binary"
	"fmt"
	"os"

	"debug/elf"

	"github.com/vmpacker/pkg/vm"
)

// NewPacker 创建 ELF 打包器
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

// PrintELFInfo 打印 ELF 信息
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

// branchTargetOffset 返回分支指令中 target32 相对于 pc 的字节偏移
// 标准分支: [op(1B)][target32(4B)] = 5B → offset=1
// TBZ/TBNZ: [op(1B)][reg(1B)][bit(1B)][target32(4B)] = 7B → offset=3
// 非分支指令返回 0
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
		output = append(output, byte(inst.size))

		// offsetMap points to where this instruction ends (after size marker)
		// because the reverse DISPATCH will start here to locate the instruction.
		offsetMap[inst.offset] = len(output)

		// byteMap provides a 1:1 mapping for every byte within the instruction
		for k := 0; k < inst.size; k++ {
			byteMap[inst.offset+k] = newInstStart + k
		}
	}

	return output, offsetMap, byteMap
}

// remapBranchTargets 重映射反转后字节码中的分支目标
//
// 扫描 reversed bytecode，找到所有分支指令，
// 将其 target32 从旧偏移替换为新偏移 (使用 offsetMap)
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
				if verbose {
					fmt.Printf("      [REMAP] pc=0x%04X op=0x%02X target: 0x%04X → 0x%04X\n",
						pc, op, oldTarget, newTarget)
				}
				binary.LittleEndian.PutUint32(bytecode[pc+toff:], uint32(newTarget))
			} else if verbose {
				fmt.Printf("      [REMAP] pc=0x%04X op=0x%02X target: 0x%04X → NOT FOUND!\n",
					pc, op, oldTarget)
			}
		}
		// 跳过指令 + size 标记 (反转后每条指令后有 1B size)
		pc += sz + 1
	}
}

// encryptOpcodes 逐指令加密 opcode 字节 (OpcodeCryptor)
//
// 遍历 bytecode[0:codeLen]，使用 vm.InstructionSize 确定每条指令的大小，
// 只加密每条指令的第一个字节 (opcode)，操作数不变。
//
// reversed=true 时，每条指令后有 1B size 标记，步进为 size+1
//
// 加密公式: encrypted_opcode[pc] = opcode[pc] ^ (u8)(ocKey ^ (pc * 0x9E3779B9))
func encryptOpcodes(bytecode []byte, codeLen int, ocKey uint32, reversed bool) {
	pc := 0
	for pc < codeLen {
		op := bytecode[pc]
		size := vm.InstructionSize(op)
		if size == 0 {
			// 未知 opcode，跳过 1 字节 (不应发生)
			pc++
			continue
		}
		// 加密 opcode 字节
		mask := byte(ocKey ^ (uint32(pc) * 0x9E3779B9))
		bytecode[pc] = op ^ mask
		// 跳到下一条指令
		if reversed {
			pc += size + 1 // +1 for size marker byte
		} else {
			pc += size
		}
	}
}
