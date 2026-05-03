package elf

import (
	"bytes"
	"crypto/rand"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/vmpacker/pkg/arch/arm32"
	"github.com/vmpacker/pkg/arch/arm64"
	"github.com/vmpacker/pkg/vm"
)

// Process 主入口
func (p *Packer) Process() error {
	var err error
	p.data, err = os.ReadFile(p.inputPath)
	if err != nil {
		return fmt.Errorf("reading file failed: %v", err)
	}

	f, err := elf.NewFile(bytes.NewReader(p.data))
	if err != nil {
		return fmt.Errorf("parsing ELF failed: %v", err)
	}
	defer f.Close()

	switch {
	case f.Machine == elf.EM_AARCH64 && f.Class == elf.ELFCLASS64:
		p.isARM32 = false
	case f.Machine == elf.EM_ARM && f.Class == elf.ELFCLASS32:
		p.isARM32 = true
		if p.thumbFuncs == nil {
			p.thumbFuncs = make(map[uint64]bool)
		}
	default:
		return fmt.Errorf("unsupported arch: machine=%s class=%s (need ARM64/ELF64 or ARM/ELF32)", f.Machine, f.Class)
	}

	activeBlob := p.interpBlob
	if p.isARM32 {
		activeBlob = p.interpBlobARM32
		if len(activeBlob) == 0 {
			return fmt.Errorf("ARM32 ELF detected but no ARM32 interp blob provided")
		}
	}

	fmt.Printf("[*] ELF: %s, Type: %s, Class: %s\n", f.Machine, f.Type, f.Class)
	fmt.Printf("[*] VM interp blob: %d bytes (ARM32=%v)\n", len(activeBlob), p.isARM32)

	// 第一阶段: 收集所有函数的字节码
	type funcEntry struct {
		name   string
		finder func() (*vm.FuncInfo, error)
	}
	var entries []funcEntry
	for _, funcName := range p.funcNames {
		fn := funcName
		entries = append(entries, funcEntry{fn, func() (*vm.FuncInfo, error) {
			return p.FindFunction(f, fn)
		}})
	}
	for _, spec := range p.addrSpecs {
		s := spec
		entries = append(entries, funcEntry{s.Name, func() (*vm.FuncInfo, error) {
			return p.FindFunctionByAddr(f, s)
		}})
	}

	var funcs []FuncBytecode
	for _, entry := range entries {
		fmt.Printf("\n[*] Processing: %s\n", entry.name)

		fi, err := entry.finder()
		if err != nil {
			return err
		}
		fmt.Printf("    Addr: 0x%X, Size: %d bytes, Section: %s\n",
			fi.Addr, fi.Size, fi.Section)

		// Trampoline: ARM64=12B, ARM32=12B, Thumb=12B
		var minTrampolineSize uint64 = 12
		if p.isARM32 {
			minTrampolineSize = 12
		}
		if fi.Size < minTrampolineSize {
			return fmt.Errorf("function %s is too small (%d bytes) for trampoline injection (minimum %d bytes); "+
				"consider excluding this function", fi.Name, fi.Size, minTrampolineSize)
		}

		code, err := p.ExtractFuncCode(f, fi)
		if err != nil {
			return err
		}

		// Common translation result fields extracted from arch-specific types
		type translationResult struct {
			Bytecode    []byte
			CodeLen     int
			Unsupported []string
			TotalInsts  int
			TransInsts  int
			Relocations []vm.Relocation
		}

		var insts []vm.Instruction
		var result translationResult
		isThumbFunc := p.isARM32 && p.thumbFuncs[fi.Addr]

		if p.isARM32 {
			insts = p.DecodeFunctionARM32(code, isThumbFunc)
			fmt.Printf("    Instructions: %d (Thumb=%v)\n", len(insts), isThumbFunc)

			if p.verbose {
				dec32 := arm32.NewDecoder()
				fmt.Println("    --- Disasm ---")
				for _, inst := range insts {
					fmt.Printf("    0x%04X: %-12s raw=0x%08X\n",
						inst.Offset, dec32.InstName(inst.Op), inst.Raw)
				}
				fmt.Println("    --- End ---")
			}

			var tr *arm32.Translator
			if isThumbFunc {
				tr = arm32.NewThumbTranslator(fi.Addr, int(fi.Size), code)
			} else {
				tr = arm32.NewTranslator(fi.Addr, int(fi.Size), code)
			}
			if p.debug {
				tr.SetDebug(true)
			}
			r, terr := tr.Translate(insts)
			if terr != nil {
				return fmt.Errorf("translation failed: %v", terr)
			}
			result = translationResult{
				Bytecode: r.Bytecode, CodeLen: r.CodeLen,
				Unsupported: r.Unsupported, TotalInsts: r.TotalInsts, TransInsts: r.TransInsts,
				Relocations: r.Relocations,
			}
			if len(r.Relocations) > 0 {
				p.relocations = append(p.relocations, r.Relocations...)
			}
		} else {
			dec64 := arm64.NewDecoder()
			insts = p.DecodeFunction(code)
			fmt.Printf("    Instructions: %d\n", len(insts))

			if p.verbose {
				fmt.Println("    --- Disasm ---")
				for _, inst := range insts {
					fmt.Printf("    0x%04X: %-12s raw=0x%08X\n",
						inst.Offset, dec64.InstName(inst.Op), inst.Raw)
				}
				fmt.Println("    --- End ---")
			}

			trans := arm64.NewTranslator(fi.Addr, int(fi.Size))
			if p.debug {
				trans.SetDebug(true)
			}
			r, terr := trans.Translate(insts)
			if terr != nil {
				return fmt.Errorf("translation failed: %v", terr)
			}
			result = translationResult{
				Bytecode: r.Bytecode, CodeLen: r.CodeLen,
				Unsupported: r.Unsupported, TotalInsts: r.TotalInsts, TransInsts: r.TransInsts,
				Relocations: r.Relocations,
			}
			if len(r.Relocations) > 0 {
				p.relocations = append(p.relocations, r.Relocations...)
			}
		}

		fmt.Printf("    Translated: %d/%d\n", result.TransInsts, result.TotalInsts)
		fmt.Printf("    Bytecode: %d bytes\n", len(result.Bytecode))

		if len(result.Unsupported) > 0 {
			fmt.Printf("    [!] Unsupported (%d):\n", len(result.Unsupported))
			for _, u := range result.Unsupported {
				fmt.Printf("        %s\n", u)
			}

			// 生成翻译失败 debug 文件
			debugPath := p.outputPath + ".debug.txt"
			df, derr := os.Create(debugPath)
			if derr != nil {
				fmt.Printf("    [!] debug 文件创建失败: %v\n", derr)
			} else {
				fmt.Fprintf(df, "================================================================\n")
				fmt.Fprintf(df, "翻译失败报告 — %s @ 0x%X\n", entry.name, fi.Addr)
				fmt.Fprintf(df, "函数大小: %d bytes, 总指令数: %d, 已翻译: %d\n",
					fi.Size, result.TotalInsts, result.TransInsts)
				fmt.Fprintf(df, "================================================================\n\n")
				fmt.Fprintf(df, "不支持的指令 (%d):\n\n", len(result.Unsupported))

				// 构建 offset→Instruction 索引，用于提取原始字节
				instMap := make(map[int]vm.Instruction)
				for _, inst := range insts {
					instMap[inst.Offset] = inst
				}

				for idx, u := range result.Unsupported {
					fmt.Fprintf(df, "[%d] %s\n", idx+1, u)

					// 尝试从 unsupported 字符串解析偏移 (格式: "偏移 0xNNNN: ....")
					var off int
					if _, err := fmt.Sscanf(u, "偏移 0x%X:", &off); err == nil {
						if inst, ok := instMap[off]; ok {
							raw := inst.Raw
							fmt.Fprintf(df, "    原始字节: %02X %02X %02X %02X\n",
								byte(raw), byte(raw>>8), byte(raw>>16), byte(raw>>24))
							fmt.Fprintf(df, "    绝对地址: 0x%X\n", fi.Addr+uint64(off))
						}
					}
					fmt.Fprintln(df)
				}

				archName := "arm64"
				if p.isARM32 {
					archName = "arm32"
				}
				fmt.Fprintf(df, "================================================================\n")
				fmt.Fprintf(df, "修复建议:\n")
				fmt.Fprintf(df, "- 为每条不支持的指令编写 demo 测试用例 (参考 demo/ 目录)\n")
				fmt.Fprintf(df, "- 在 pkg/arch/%s/translator.go translateOne() 中添加对应 case\n", archName)
				fmt.Fprintf(df, "- 使用 -v 标志查看完整反汇编上下文\n")
				fmt.Fprintf(df, "================================================================\n")

				df.Close()
				fmt.Printf("    [+] 翻译失败 debug 文件: %s\n", debugPath)
			}

			return fmt.Errorf("translation aborted: %d unsupported instruction(s) in %s — cannot produce safe output",
				len(result.Unsupported), entry.name)
		}

		// debug: 生成对照文件 (必须在反转/加密之前, 使用原始正向字节码)
		if p.debug {
			debugPath := p.outputPath + ".debug.txt"
			df, derr := os.OpenFile(debugPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if derr != nil {
				fmt.Printf("    [!] debug file create failed: %v\n", derr)
			} else {
				fmt.Fprintf(df, "================================================================\n")
				fmt.Fprintf(df, "Function: %s @ 0x%X (size: %d)\n", entry.name, fi.Addr, fi.Size)
				fmt.Fprintf(df, "VM bytecode: %d bytes (pre-reverse)\n", len(result.Bytecode))
				fmt.Fprintf(df, "================================================================\n\n")

				if p.isARM32 {
					var tr32 *arm32.Translator
					if isThumbFunc {
						tr32 = arm32.NewThumbTranslator(fi.Addr, int(fi.Size), code)
					} else {
						tr32 = arm32.NewTranslator(fi.Addr, int(fi.Size), code)
					}
					tr32.SetDebug(true)
					tr32.Translate(insts)
					for _, dbg := range tr32.DebugLog() {
						vmLines := vm.DisasmRange(result.Bytecode, dbg.VMStart, dbg.VMEnd)
						fmt.Fprintf(df, "ARM32  %04X: %-16s  (raw=0x%08X)\n",
							dbg.ARM32Offset, dbg.ARM32Asm, dbg.ARM32Raw)
						for _, vl := range vmLines {
							fmt.Fprintf(df, "  VM   %s\n", vl)
						}
						fmt.Fprintln(df)
					}
				} else {
					trans64 := arm64.NewTranslator(fi.Addr, int(fi.Size))
					trans64.SetDebug(true)
					trans64.Translate(insts)
					for _, dbg := range trans64.DebugLog() {
						vmLines := vm.DisasmRange(result.Bytecode, dbg.VMStart, dbg.VMEnd)
						fmt.Fprintf(df, "ARM64  %04X: %-16s  (raw=0x%08X)\n",
							dbg.ARM64Offset, dbg.ARM64Asm, dbg.ARM64Raw)
						for _, vl := range vmLines {
							fmt.Fprintf(df, "  VM   %s\n", vl)
						}
						fmt.Fprintln(df)
					}
				}

				df.Close()
				fmt.Printf("    [+] Debug: %s\n", debugPath)
			}
		}

		// ---- PC 反向遍历: 反转指令顺序 ----
		// 必须在 OpcodeCryptor 之前执行 (加密使用最终 pc 位置)
		reversed, offsetMap, byteMap := reverseInstructions(result.Bytecode, result.CodeLen)

		// 重映射分支目标 (使用反转后的偏移)
		newCodeLen := len(reversed)
		remapBranchTargets(reversed, newCodeLen, offsetMap, p.verbose)

		// 重映射 addr_map 中的 vm_off (BR 间接跳转)
		mapCount := binary.LittleEndian.Uint32(result.Bytecode[len(result.Bytecode)-16:])
		trailerStart := result.CodeLen
		for j := 0; j < int(mapCount); j++ {
			entryOff := trailerStart + j*8
			vmOff := binary.LittleEndian.Uint32(result.Bytecode[entryOff+4:])
			if newVmOff, ok := offsetMap[int(vmOff)]; ok {
				binary.LittleEndian.PutUint32(result.Bytecode[entryOff+4:], uint32(newVmOff))
			}
		}

		// 用反转后的字节码替换原始指令区，保留 trailer
		trailer := result.Bytecode[result.CodeLen:]
		finalBytecode := make([]byte, 0, newCodeLen+len(trailer))
		finalBytecode = append(finalBytecode, reversed...)
		finalBytecode = append(finalBytecode, trailer...)
		result.Bytecode = finalBytecode
		result.CodeLen = newCodeLen

		if p.verbose {
			fmt.Printf("    [REV] reversed: %d insts, newCodeLen=%d (was %d), offsetMap entries=%d\n",
				len(offsetMap), newCodeLen, result.CodeLen, len(offsetMap))
		}

		// ---- OpcodeCryptor: 逐指令 opcode 加密 ----
		var ocKeyBuf [4]byte
		if _, err := rand.Read(ocKeyBuf[:]); err != nil {
			return fmt.Errorf("generating oc_key failed: %v", err)
		}
		ocKey := binary.LittleEndian.Uint32(ocKeyBuf[:])

		// 加密字节码 (reversed=true: 每条指令后有 1B size 标记)
		encryptOpcodes(result.Bytecode, result.CodeLen, ocKey, true)

		// 将 reverse 标志 + oc_key 写入 trailer
		reverseOffset := result.CodeLen + int(mapCount)*8
		result.Bytecode[reverseOffset] = 1
		ocKeyOffset := reverseOffset + 1                  // reverse(1B) 之后
		binary.LittleEndian.PutUint32(result.Bytecode[ocKeyOffset:], ocKey)

		if p.verbose {
			fmt.Printf("    [OC] oc_key=0x%08X, codeLen=%d, mapCount=%d, reverseOff=%d, keyOff=%d\n",
				ocKey, result.CodeLen, mapCount, reverseOffset, ocKeyOffset)
		}

		// Remap RTLR relocation offsets through the reversal byteMap
		for i := range result.Relocations {
			if newOff, ok := byteMap[result.Relocations[i].BcOffset]; ok {
				if p.verbose {
					fmt.Printf("    [RELOC] remap BcOffset %d -> %d\n",
						result.Relocations[i].BcOffset, newOff)
				}
				result.Relocations[i].BcOffset = newOff
			}
		}

		// ---- XOR chain 加密 (整段字节码) ----
		xorKey := byte(0xA5)
		encrypted := make([]byte, len(result.Bytecode))
		for i, b := range result.Bytecode {
			encrypted[i] = b ^ xorKey
		}

		funcs = append(funcs, FuncBytecode{
			FI: fi, Encrypted: encrypted, XorKey: xorKey,
			Relocations: result.Relocations,
		})
	}

	// 第二阶段: 批量注入 (一次 PT_NOTE 劫持)
	fmt.Printf("\n[*] Injecting %d functions...\n", len(funcs))
	err = p.injectVMPBatch(funcs)
	if err != nil {
		return fmt.Errorf("injection failed: %v", err)
	}

	for _, fb := range funcs {
		fmt.Printf("    [+] %s VMP protected\n", fb.FI.Name)
	}

	// 第三阶段: 清除符号表 (可选)
	if p.stripSymbols {
		p.stripSections()
		fmt.Println("[*] Symbols stripped")
	}

	err = os.WriteFile(p.outputPath, p.data, 0755)
	if err != nil {
		return fmt.Errorf("writing output failed: %v", err)
	}

	fmt.Printf("\n[+] Output: %s\n", p.outputPath)
	return nil
}
