package elf

import (
	"bytes"
	"crypto/rand"
	"debug/elf"
	"encoding/binary"
	"fmt"

	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/vmpacker/pkg/arch/arm32"
	"github.com/vmpacker/pkg/arch/arm64"
	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// ELF 解析器 + 修改器 v3
//
// 注入策略: PT_NOTE → PT_LOAD 劫持
//   1. 将 VM 解释器 blob + 加密字节码追加到文件末尾
//   2. 将 PT_NOTE 段转换为 PT_LOAD (RX)，映射追加的数据
//   3. 新 LOAD 段使用独立的虚拟地址 (0x800000 起)
//   4. 原函数改写为跳板 → BL 到新段中的 VM 解释器
//
// 优点: 不移动任何现有数据，不破坏段对齐
// ============================================================

// AddrSpec 按地址指定函数
type AddrSpec struct {
	Addr uint64
	End  uint64 // 0 = 自动检测
	Name string // 可选名称
}

// ParseAddrSpec 解析地址规格: "0xADDR", "0xSTART-0xEND", "0xSTART-0xEND:name"
func ParseAddrSpec(s string) (AddrSpec, error) {
	var spec AddrSpec
	// 分离可选名称 (最后一个冒号后面)
	if idx := strings.LastIndex(s, ":"); idx > 2 {
		candidate := s[idx+1:]
		// 如果不像十六进制数则是名称
		if _, err := strconv.ParseUint(candidate, 0, 64); err != nil {
			spec.Name = candidate
			s = s[:idx]
		}
	}
	// 解析地址范围
	if parts := strings.Split(s, "-"); len(parts) == 2 {
		start, err := strconv.ParseUint(parts[0], 0, 64)
		if err != nil {
			return spec, fmt.Errorf("起始地址无效: %s", parts[0])
		}
		end, err := strconv.ParseUint(parts[1], 0, 64)
		if err != nil {
			return spec, fmt.Errorf("结束地址无效: %s", parts[1])
		}
		if end <= start {
			return spec, fmt.Errorf("结束地址必须大于起始地址")
		}
		spec.Addr = start
		spec.End = end
	} else {
		addr, err := strconv.ParseUint(s, 0, 64)
		if err != nil {
			return spec, fmt.Errorf("地址无效: %s", s)
		}
		spec.Addr = addr
	}
	if spec.Name == "" {
		spec.Name = fmt.Sprintf("sub_%X", spec.Addr)
	}
	return spec, nil
}

// Packer ELF VMP 打包器
type Packer struct {
	inputPath       string
	outputPath      string
	funcNames       []string
	addrSpecs       []AddrSpec
	verbose         bool
	stripSymbols    bool
	debug           bool
	tokenEntry      bool // Token 化入口模式
	data            []byte
	interpBlob      []byte          // ARM64 blob
	interpBlobARM32 []byte          // ARM32 blob (optional)
	isARM32         bool            // detected at Process() time
	thumbFuncs      map[uint64]bool // Thumb-mode function addresses (bit0 stripped)
	relocations     []vm.Relocation // 运行时待修复的重定位 (主要是 .so ASLR)
}

// FuncBytecode 保存单个函数的加密字节码和元信息
type FuncBytecode struct {
	FI          *vm.FuncInfo
	Encrypted   []byte
	XorKey      byte
	Relocations []vm.Relocation
}

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

// FindFunction 在 ELF 中查找函数，并计算其实体在文件中的正确偏移
func (p *Packer) FindFunction(f *elf.File, name string) (*vm.FuncInfo, error) {
	syms, err := f.Symbols()
	if err != nil {
		syms, err = f.DynamicSymbols()
		if err != nil {
			return nil, fmt.Errorf("reading symbol table failed: %v", err)
		}
	}
	for _, sym := range syms {
		if sym.Name == name && elf.ST_TYPE(sym.Info) == elf.STT_FUNC {
			addr := sym.Value
			// ARM32: bit0 indicates Thumb mode
			if p.isARM32 && addr&1 != 0 {
				if p.thumbFuncs == nil {
					p.thumbFuncs = make(map[uint64]bool)
				}
				addr &^= 1
				p.thumbFuncs[addr] = true
			}
			info := &vm.FuncInfo{
				Name: sym.Name,
				Addr: addr,
				Size: sym.Size,
			}

			// 优先使用 Program Headers 定位文件偏移 (Android .so 必备)
			foundOffset := false
			for _, ph := range f.Progs {
				if ph.Type == elf.PT_LOAD && (ph.Flags&elf.PF_X != 0) {
					if addr >= ph.Vaddr && addr < ph.Vaddr+ph.Memsz {
						info.Offset = ph.Off + (addr - ph.Vaddr)
						info.Section = "__LOAD_X"
						foundOffset = true
						break
					}
				}
			}

			// 如果段信息找不到，回退到 Section Headers
			if !foundOffset && int(sym.Section) < len(f.Sections) {
				sec := f.Sections[sym.Section]
				if addr >= sec.Addr {
					info.Section = sec.Name
					info.Offset = sec.Offset + (addr - sec.Addr)
					foundOffset = true
				}
			}

			if !foundOffset {
				return nil, fmt.Errorf("could not determine file offset for function %s at 0x%X", name, addr)
			}
			return info, nil
		}
	}
	return nil, fmt.Errorf("function '%s' not found", name)
}

// FindFunctionByAddr 通过地址查找函数
func (p *Packer) FindFunctionByAddr(f *elf.File, spec AddrSpec) (*vm.FuncInfo, error) {
	// 优先在 .text 段中定位
	textSec := f.Section(".text")

	var secName string
	var secAddr, secOffset, secSize uint64
	var secData []byte

	if textSec != nil {
		secName = ".text"
		secAddr = textSec.Addr
		secOffset = textSec.Offset
		secSize = textSec.Size
		d, err := textSec.Data()
		if err != nil {
			return nil, fmt.Errorf("reading .text failed: %v", err)
		}
		secData = d
	} else {
		// Fallback: 在可执行 LOAD segment 中查找
		found := false
		for _, prog := range f.Progs {
			if prog.Type != elf.PT_LOAD {
				continue
			}
			if prog.Flags&elf.PF_X == 0 {
				continue
			}
			segEnd := prog.Vaddr + prog.Memsz
			if spec.Addr >= prog.Vaddr && spec.Addr < segEnd {
				secName = "__LOAD_X"
				secAddr = prog.Vaddr
				secOffset = prog.Off
				secSize = prog.Filesz
				d := make([]byte, prog.Filesz)
				if _, err := prog.ReadAt(d, 0); err != nil {
					return nil, fmt.Errorf("reading LOAD segment failed: %v", err)
				}
				secData = d
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("address 0x%X not in any executable segment", spec.Addr)
		}
	}

	// 确认地址在范围内
	if spec.Addr < secAddr || spec.Addr >= secAddr+secSize {
		return nil, fmt.Errorf("address 0x%X not in %s (0x%X-0x%X)",
			spec.Addr, secName, secAddr, secAddr+secSize)
	}

	var size uint64
	if spec.End > 0 {
		size = spec.End - spec.Addr
	} else if p.isARM32 {
		// ARM32 RET detection: BX LR (0xE12FFF1E) or POP {..., PC} (0x__BD__xx)
		startOff := spec.Addr - secAddr
		isThumb := p.thumbFuncs[spec.Addr]
		found := false
		if isThumb {
			// Thumb: scan 2 bytes at a time for POP {PC} (0xBDxx) or BX LR (0x4770)
			for i := startOff; i+2 <= uint64(len(secData)); i += 2 {
				hw := binary.LittleEndian.Uint16(secData[i:])
				if hw == 0x4770 { // BX LR
					size = i + 2 - startOff
					found = true
					break
				}
				if hw&0xFF00 == 0xBD00 { // POP {..., PC}
					size = i + 2 - startOff
					found = true
					break
				}
			}
		} else {
			for i := startOff; i+4 <= uint64(len(secData)); i += 4 {
				inst := binary.LittleEndian.Uint32(secData[i:])
				if inst == 0xE12FFF1E { // BX LR
					size = i + 4 - startOff
					found = true
					break
				}
				// POP {..., PC}: cond=AL, 0x08BD8000 mask
				if inst&0xFFFF8000 == 0xE8BD8000 { // LDMIA SP!, {..., PC}
					size = i + 4 - startOff
					found = true
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("cannot detect function size at 0x%X (no BX LR / POP {PC} found)", spec.Addr)
		}
	} else {
		// ARM64: scan for RET (0xD65F03C0)
		startOff := spec.Addr - secAddr
		found := false
		for i := startOff; i+4 <= uint64(len(secData)); i += 4 {
			inst := binary.LittleEndian.Uint32(secData[i:])
			if inst == 0xD65F03C0 { // RET
				size = i + 4 - startOff
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("cannot detect function size at 0x%X (no RET found)", spec.Addr)
		}
	}

	fi := &vm.FuncInfo{
		Name:    spec.Name,
		Addr:    spec.Addr,
		Size:    size,
		Section: secName,
		Offset:  secOffset + (spec.Addr - secAddr),
	}
	// Final sanity check for file offset
	if fi.Offset >= uint64(len(p.data)) {
		return nil, fmt.Errorf("calculated file offset 0x%X for 0x%X is out of bounds (file size 0x%X)",
			fi.Offset, fi.Addr, len(p.data))
	}
	return fi, nil
}

// ExtractFuncCode 提取函数机器码
func (p *Packer) ExtractFuncCode(f *elf.File, fi *vm.FuncInfo) ([]byte, error) {
	if fi.Size == 0 {
		return nil, fmt.Errorf("function %s has zero size", fi.Name)
	}

	if fi.Section == "__LOAD_X" {
		// 无 section headers: 从 LOAD segment 读取
		for _, prog := range f.Progs {
			if prog.Type != elf.PT_LOAD || prog.Flags&elf.PF_X == 0 {
				continue
			}
			segEnd := prog.Vaddr + prog.Filesz
			if fi.Addr >= prog.Vaddr && fi.Addr+fi.Size <= segEnd {
				localOff := fi.Addr - prog.Vaddr
				code := make([]byte, fi.Size)
				if _, err := prog.ReadAt(code, int64(localOff)); err != nil {
					return nil, fmt.Errorf("reading LOAD segment failed: %v", err)
				}
				return code, nil
			}
		}
		return nil, fmt.Errorf("function %s (0x%X) not in any LOAD segment", fi.Name, fi.Addr)
	}

	section := f.Section(fi.Section)
	if section == nil {
		return nil, fmt.Errorf("section %s not found", fi.Section)
	}
	data, err := section.Data()
	if err != nil {
		return nil, fmt.Errorf("reading section data failed: %v", err)
	}
	localOff := fi.Addr - section.Addr
	if localOff+fi.Size > uint64(len(data)) {
		return nil, fmt.Errorf("function exceeds section bounds")
	}
	code := make([]byte, fi.Size)
	copy(code, data[localOff:localOff+fi.Size])
	return code, nil
}

// DecodeFunction 解码 ARM64 指令
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

// DecodeFunctionARM32 解码 ARM32/Thumb 指令
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

					// 尝试从 unsupported 字符串解析偏移 (格式: "偏移 0xNNNN: ...")
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
		reversed, offsetMap := reverseInstructions(result.Bytecode, result.CodeLen)

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

		// Remap RTLR relocation offsets through the reversal offsetMap
		for i := range result.Relocations {
			if newOff, ok := offsetMap[result.Relocations[i].BcOffset]; ok {
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

// stripSections 就地清除符号/调试 section
// 不改变文件布局和 section header 数量，只将目标 section 置空
// 同时修复其他 section 对被删除 section 的 sh_link 引用
func (p *Packer) stripSections() {
	if p.isARM32 {
		p.stripSections32()
		return
	}
	p.stripSections64()
}

func (p *Packer) stripSections64() {
	if len(p.data) < 64 {
		return
	}
	ehdr := readEhdr64(p.data)

	// Validate section header table bounds
	shTableEnd := ehdr.Shoff + uint64(ehdr.Shnum)*uint64(ehdr.Shentsize)
	if ehdr.Shoff == 0 || shTableEnd > uint64(len(p.data)) {
		if p.verbose {
			fmt.Printf("    [strip] Section header table missing or truncated (shoff=0x%X, shnum=%d), skipping strip\n",
				ehdr.Shoff, ehdr.Shnum)
		}
		return
	}

	if uint32(ehdr.Shstrndx) >= uint32(ehdr.Shnum) {
		return
	}

	shstrOff := ehdr.Shoff + uint64(ehdr.Shstrndx)*uint64(ehdr.Shentsize)
	shstrSecOff := binary.LittleEndian.Uint64(p.data[shstrOff+24:])
	shstrSecSz := binary.LittleEndian.Uint64(p.data[shstrOff+32:])

	if shstrSecOff+shstrSecSz > uint64(len(p.data)) {
		return
	}

	getSectionName := func(nameOff uint32) string {
		start := shstrSecOff + uint64(nameOff)
		if start >= uint64(len(p.data)) || start >= shstrSecOff+shstrSecSz {
			return ""
		}
		end := start
		for end < shstrSecOff+shstrSecSz && end < uint64(len(p.data)) && p.data[end] != 0 {
			end++
		}
		return string(p.data[start:end])
	}

	// 要清除的 section 名称
	stripNames := map[string]bool{
		".symtab":            true,
		".strtab":            true,
		".comment":           true,
		".note.GNU-stack":    true,
		".note.gnu.build-id": true,
	}

	// 第一遍: 收集被删除的 section index
	stripped := make(map[int]bool)
	for i := 0; i < int(ehdr.Shnum); i++ {
		shOff := ehdr.Shoff + uint64(i)*uint64(ehdr.Shentsize)
		nameOff := binary.LittleEndian.Uint32(p.data[shOff:])
		name := getSectionName(nameOff)
		if stripNames[name] {
			stripped[i] = true
		}
	}

	// 第二遍: 清零被删除的 section，修复 sh_link 引用
	for i := 0; i < int(ehdr.Shnum); i++ {
		shOff := ehdr.Shoff + uint64(i)*uint64(ehdr.Shentsize)

		if stripped[i] {
			// 读取 section 的文件偏移和大小
			secOff := binary.LittleEndian.Uint64(p.data[shOff+24:])
			secSz := binary.LittleEndian.Uint64(p.data[shOff+32:])

			// 用 0x00 清零 section 内容（等效 strip -s）
			if secOff+secSz <= uint64(len(p.data)) {
				for j := uint64(0); j < secSz; j++ {
					p.data[secOff+j] = 0
				}
			}

			nameOff := binary.LittleEndian.Uint32(p.data[shOff:])
			name := getSectionName(nameOff)

			// 清零整个 section header entry（保留 sh_name 用于调试）
			// sh_type = SHT_NULL
			binary.LittleEndian.PutUint32(p.data[shOff+4:], 0)
			// sh_flags = 0
			binary.LittleEndian.PutUint64(p.data[shOff+8:], 0)
			// sh_addr = 0
			binary.LittleEndian.PutUint64(p.data[shOff+16:], 0)
			// sh_offset = 0
			binary.LittleEndian.PutUint64(p.data[shOff+24:], 0)
			// sh_size = 0
			binary.LittleEndian.PutUint64(p.data[shOff+32:], 0)
			// sh_link = 0
			binary.LittleEndian.PutUint32(p.data[shOff+40:], 0)
			// sh_info = 0
			binary.LittleEndian.PutUint32(p.data[shOff+44:], 0)
			// sh_addralign = 0
			binary.LittleEndian.PutUint64(p.data[shOff+48:], 0)
			// sh_entsize = 0
			binary.LittleEndian.PutUint64(p.data[shOff+56:], 0)

			if p.verbose {
				fmt.Printf("    [strip] %s cleared (off=0x%X, sz=%d)\n", name, secOff, secSz)
			}
		} else {
			// 非被删除的 section: 检查 sh_link 是否指向被删除的 section
			shLink := binary.LittleEndian.Uint32(p.data[shOff+40:])
			if shLink > 0 && stripped[int(shLink)] {
				binary.LittleEndian.PutUint32(p.data[shOff+40:], 0) // 清零 sh_link
				if p.verbose {
					nameOff := binary.LittleEndian.Uint32(p.data[shOff:])
					name := getSectionName(nameOff)
					fmt.Printf("    [strip] %s: sh_link %d → 0 (target stripped)\n", name, shLink)
				}
			}
		}
	}
}

func (p *Packer) stripSections32() {
	if len(p.data) < 52 {
		return
	}
	ehdr := readEhdr32(p.data)

	// Validate section header table bounds
	shTableEnd := uint64(ehdr.Shoff) + uint64(ehdr.Shnum)*uint64(ehdr.Shentsize)
	if ehdr.Shoff == 0 || shTableEnd > uint64(len(p.data)) {
		if p.verbose {
			fmt.Printf("    [strip] Section header table missing or truncated (shoff=0x%X, shnum=%d), skipping strip\n",
				ehdr.Shoff, ehdr.Shnum)
		}
		return
	}

	if uint32(ehdr.Shstrndx) >= uint32(ehdr.Shnum) {
		return
	}

	shstrOff := uint32(ehdr.Shoff) + uint32(ehdr.Shstrndx)*uint32(ehdr.Shentsize)
	shstrSecOff := binary.LittleEndian.Uint32(p.data[shstrOff+16:])
	shstrSecSz := binary.LittleEndian.Uint32(p.data[shstrOff+20:])

	if uint64(shstrSecOff)+uint64(shstrSecSz) > uint64(len(p.data)) {
		return
	}

	getSectionName := func(nameOff uint32) string {
		start := shstrSecOff + nameOff
		if start >= uint32(len(p.data)) || start >= shstrSecOff+shstrSecSz {
			return ""
		}
		end := start
		for end < shstrSecOff+shstrSecSz && end < uint32(len(p.data)) && p.data[end] != 0 {
			end++
		}
		return string(p.data[start:end])
	}

	stripNames := map[string]bool{
		".symtab": true, ".strtab": true, ".comment": true,
		".note.GNU-stack": true, ".note.gnu.build-id": true,
	}

	stripped := make(map[int]bool)
	for i := 0; i < int(ehdr.Shnum); i++ {
		shOff := ehdr.Shoff + uint32(i)*uint32(ehdr.Shentsize)
		nameOff := binary.LittleEndian.Uint32(p.data[shOff:])
		name := getSectionName(nameOff)
		if stripNames[name] {
			stripped[i] = true
		}
	}

	// ELF32 section header layout (40 bytes):
	// +0: sh_name(4), +4: sh_type(4), +8: sh_flags(4), +12: sh_addr(4),
	// +16: sh_offset(4), +20: sh_size(4), +24: sh_link(4), +28: sh_info(4),
	// +32: sh_addralign(4), +36: sh_entsize(4)
	for i := 0; i < int(ehdr.Shnum); i++ {
		shOff := ehdr.Shoff + uint32(i)*uint32(ehdr.Shentsize)

		if stripped[i] {
			secOff := binary.LittleEndian.Uint32(p.data[shOff+16:])
			secSz := binary.LittleEndian.Uint32(p.data[shOff+20:])

			if uint64(secOff)+uint64(secSz) <= uint64(len(p.data)) {
				for j := uint32(0); j < secSz; j++ {
					p.data[secOff+j] = 0
				}
			}

			// Clear section header fields (except sh_name)
			binary.LittleEndian.PutUint32(p.data[shOff+4:], 0)  // sh_type = SHT_NULL
			binary.LittleEndian.PutUint32(p.data[shOff+8:], 0)  // sh_flags
			binary.LittleEndian.PutUint32(p.data[shOff+12:], 0) // sh_addr
			binary.LittleEndian.PutUint32(p.data[shOff+16:], 0) // sh_offset
			binary.LittleEndian.PutUint32(p.data[shOff+20:], 0) // sh_size
			binary.LittleEndian.PutUint32(p.data[shOff+24:], 0) // sh_link
			binary.LittleEndian.PutUint32(p.data[shOff+28:], 0) // sh_info
			binary.LittleEndian.PutUint32(p.data[shOff+32:], 0) // sh_addralign
			binary.LittleEndian.PutUint32(p.data[shOff+36:], 0) // sh_entsize
		} else {
			shLink := binary.LittleEndian.Uint32(p.data[shOff+24:])
			if shLink > 0 && stripped[int(shLink)] {
				binary.LittleEndian.PutUint32(p.data[shOff+24:], 0)
			}
		}
	}
}

// injectVMPBatch — 批量 PT_NOTE hijack 注入
func (p *Packer) injectVMPBatch(funcs []FuncBytecode) error {
	if p.isARM32 {
		return p.injectVMPBatch32(funcs)
	}
	return p.injectVMPBatch64(funcs)
}

// injectVMPBatch64 — ARM64 ELF64 注入
func (p *Packer) injectVMPBatch64(funcs []FuncBytecode) error {
	ehdr := readEhdr64(p.data)

	if len(p.interpBlob) < 24 {
		return fmt.Errorf("token mode requires extended blob header (24 bytes), got %d", len(p.interpBlob))
	}
	entryOff := binary.LittleEndian.Uint64(p.interpBlob[:8])
	tokenEntryOff := binary.LittleEndian.Uint64(p.interpBlob[8:16])
	tokenTableVAOff := binary.LittleEndian.Uint64(p.interpBlob[16:24])
	interpCode := p.interpBlob[24:]

	// 1. 构造 payload: [interpCode][8B RTLr gap][bc0][pad][bc1][pad][...]
	payload := make([]byte, 0, len(interpCode)+1024)
	payload = append(payload, interpCode...)
	// Reserve 8 bytes after code for RTLr offset patch (_token_table_va + 16)
	// so it doesn't overlap with the first bytecode
	payload = append(payload, make([]byte, 8)...)
	for len(payload)%4 != 0 {
		payload = append(payload, 0x00)
	}

	type bcRecord struct {
		payloadOff int
		bcLen      int
	}
	records := make([]bcRecord, len(funcs))

	for i, fb := range funcs {
		records[i].payloadOff = len(payload)
		records[i].bcLen = len(fb.Encrypted)
		payload = append(payload, fb.Encrypted...)
		for len(payload)%4 != 0 {
			payload = append(payload, 0x00)
		}
	}

	// 2. 追加到文件末尾 (页对齐)
	appendOff := uint64(len(p.data))
	padLen := (0x1000 - (appendOff % 0x1000)) % 0x1000
	for i := uint64(0); i < padLen; i++ {
		p.data = append(p.data, 0x00)
	}
	payloadFileOff := uint64(len(p.data))
	var maxVA uint64
	for i := 0; i < int(ehdr.Phnum); i++ {
		phOff := ehdr.Phoff + uint64(i)*uint64(ehdr.Phentsize)
		ph := readPhdr64(p.data, phOff)
		if ph.Type == uint32(elf.PT_LOAD) {
			end := ph.Vaddr + ph.Memsz
			if end > maxVA {
				maxVA = end
			}
		}
	}
	payloadVA := (maxVA + 0xFFFF) &^ 0xFFFF

	p.data = append(p.data, payload...)

	interpVA := payloadVA + entryOff
	_ = interpVA

	fmt.Printf("    Payload at file offset: 0x%X, VA: 0x%X, size: %d\n",
		payloadFileOff, payloadVA, len(payload))

	for i, fb := range funcs {
		bcVA := payloadVA + uint64(records[i].payloadOff)
		fmt.Printf("    [%s] bytecode VA: 0x%X, len: %d\n",
			fb.FI.Name, bcVA, records[i].bcLen)
	}

	// 3. 找到 PT_NOTE 段并劫持为 PT_LOAD
	noteIdx := -1
	for i := 0; i < int(ehdr.Phnum); i++ {
		phOff := ehdr.Phoff + uint64(i)*uint64(ehdr.Phentsize)
		ph := readPhdr64(p.data, phOff)
		if ph.Type == uint32(elf.PT_NOTE) {
			noteIdx = i
			break
		}
	}
	if noteIdx < 0 {
		return fmt.Errorf("PT_NOTE segment not found")
	}

	// 4. PT_NOTE → PT_LOAD (RX)
	notePhdrOff := ehdr.Phoff + uint64(noteIdx)*uint64(ehdr.Phentsize)
	newPhdr := elf64Phdr{
		Type:   uint32(elf.PT_LOAD),
		Flags:  uint32(elf.PF_R | elf.PF_X),
		Off:    payloadFileOff,
		Vaddr:  payloadVA,
		Paddr:  payloadVA,
		Filesz: uint64(len(payload)),
		Memsz:  uint64(len(payload)),
		Align:  0x1000,
	}
	writePhdr64(p.data, notePhdrOff, newPhdr)

	fmt.Printf("    PT_NOTE[%d] -> PT_LOAD RX: off=0x%X va=0x%X sz=0x%X\n",
		noteIdx, payloadFileOff, payloadVA, len(payload))

	// 4b. 按 Vaddr 升序重排所有 PT_LOAD 段
	{
		type phdrSlot struct {
			idx  int
			phdr elf64Phdr
		}
		var loads []phdrSlot
		for i := 0; i < int(ehdr.Phnum); i++ {
			off := ehdr.Phoff + uint64(i)*uint64(ehdr.Phentsize)
			ph := readPhdr64(p.data, off)
			if ph.Type == uint32(elf.PT_LOAD) {
				loads = append(loads, phdrSlot{idx: i, phdr: ph})
			}
		}
		needSort := false
		for k := 1; k < len(loads); k++ {
			if loads[k].phdr.Vaddr < loads[k-1].phdr.Vaddr {
				needSort = true
				break
			}
		}
		if needSort {
			sort.Slice(loads, func(a, b int) bool {
				return loads[a].phdr.Vaddr < loads[b].phdr.Vaddr
			})
			slotIndices := make([]int, len(loads))
			for k := range loads {
				slotIndices[k] = loads[k].idx
			}
			sort.Ints(slotIndices)
			for k, si := range slotIndices {
				off := ehdr.Phoff + uint64(si)*uint64(ehdr.Phentsize)
				writePhdr64(p.data, off, loads[k].phdr)
			}
			fmt.Printf("    [PHDR] Reordered %d PT_LOAD segments by Vaddr ascending\n", len(loads))
			for i := 0; i < int(ehdr.Phnum); i++ {
				off := ehdr.Phoff + uint64(i)*uint64(ehdr.Phentsize)
				ph := readPhdr64(p.data, off)
				if ph.Type == uint32(elf.PT_LOAD) && ph.Vaddr == payloadVA {
					notePhdrOff = off
					break
				}
			}
		}
	}

	// 5. Token 跳板
	for len(payload)%8 != 0 {
		payload = append(payload, 0x00)
	}
	tokenTableOff := len(payload)
	tokenTableVA := payloadVA + uint64(tokenTableOff)

	selfVA := payloadVA + tokenTableVAOff
	for i := range funcs {
		bcVA := payloadVA + uint64(records[i].payloadOff)
		bcLen := uint32(records[i].bcLen)

		var desc [16]byte
		binary.LittleEndian.PutUint64(desc[0:], bcVA-selfVA)
		binary.LittleEndian.PutUint32(desc[8:], bcLen)
		binary.LittleEndian.PutUint32(desc[12:], 0)
		payload = append(payload, desc[:]...)
	}

	// 6. RTLR 重定位表 (主要用于 Android .so ASLR 修复)
	// 格式: [Magic:4B "RTLR"][Count:4B][{func_id:8B, bc_off:8B, target_addr:8B}...]
	rtlrOff := len(payload)
	payload = append(payload, "RTLR"...)
	totalRelocs := 0
	for _, fb := range funcs {
		totalRelocs += len(fb.Relocations)
	}
	tmp32 := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmp32, uint32(totalRelocs))
	payload = append(payload, tmp32...)

	if totalRelocs > 0 {
		fmt.Printf("    [RELOC] Appending %d relocations to RTLR table\n", totalRelocs)
		for i, fb := range funcs {
			for _, rel := range fb.Relocations {
				tmp64 := make([]byte, 8)
				// func_id
				binary.LittleEndian.PutUint64(tmp64, uint64(i))
				payload = append(payload, tmp64...)
				// bc_off
				binary.LittleEndian.PutUint64(tmp64, uint64(rel.BcOffset))
				payload = append(payload, tmp64...)
				// target_addr
				binary.LittleEndian.PutUint64(tmp64, rel.TargetAddr)
				payload = append(payload, tmp64...)
			}
		}
	}

	newPhdr.Filesz = uint64(len(payload))
	newPhdr.Memsz = uint64(len(payload))
	writePhdr64(p.data, notePhdrOff, newPhdr)

	p.data = p.data[:payloadFileOff]
	p.data = append(p.data, payload...)

	tblRelOff := tokenTableVA - selfVA
	binary.LittleEndian.PutUint64(p.data[payloadFileOff+tokenTableVAOff:], tblRelOff)

	// Patch _link_time_self_va (u64 right after _token_table_va) with link-time VA
	// so the stub can compute ASLR slide = runtime_self_va - link_time_self_va
	binary.LittleEndian.PutUint64(p.data[payloadFileOff+tokenTableVAOff+8:], selfVA)

	// Patch RTLR offset into the descriptor table area (fixed position relative to selfVA)
	// so the stub can find it.
	// Actually, let's put the RTLR offset in the 3rd u64 of the header (at offset 24)
	binary.LittleEndian.PutUint64(p.data[payloadFileOff+tokenTableVAOff+16:], uint64(rtlrOff)-(tokenTableVAOff))

	fmt.Printf("    [TOKEN] descriptor table VA: 0x%X, entries: %d\n", tokenTableVA, len(funcs))
	fmt.Printf("    [TOKEN] RTLR table at offset 0x%X in payload, %d relocs\n", rtlrOff, totalRelocs)

	vmEntryTokenVA := payloadVA + tokenEntryOff
	fmt.Printf("    [TOKEN] vm_entry_token VA: 0x%X\n", vmEntryTokenVA)

	for i, fb := range funcs {
		funcID := uint32(i)
		token := (uint32(fb.XorKey) << 24) | (0 << 12) | (funcID & 0xFFF)

		trampoline := BuildTokenTrampoline(fb.FI.Addr, vmEntryTokenVA, token)
		if uint64(len(trampoline)) > fb.FI.Size {
			return fmt.Errorf("token trampoline for %s (%d bytes) exceeds function size (%d bytes)",
				fb.FI.Name, len(trampoline), fb.FI.Size)
		}

		for j := 0; j < len(trampoline); j++ {
			p.data[fb.FI.Offset+uint64(j)] = trampoline[j]
		}

		garbageLen := int(fb.FI.Size) - len(trampoline)
		if garbageLen > 0 {
			garbage := make([]byte, garbageLen)
			rand.Read(garbage)
			copy(p.data[fb.FI.Offset+uint64(len(trampoline)):], garbage)
		}

		fmt.Printf("    [TOKEN] %s: func_id=%d, token=0x%08X, trampoline=%d bytes\n",
			fb.FI.Name, funcID, token, len(trampoline))
	}

	return nil
}

// injectVMPBatch32 — ARM32 ELF32 注入
func (p *Packer) injectVMPBatch32(funcs []FuncBytecode) error {
	ehdr := readEhdr32(p.data)
	blob := p.interpBlobARM32

	// ARM32 blob header: 3 x uint32 = 12 bytes (entryOff, tokenEntryOff, tokenTableVAOff)
	if len(blob) < 12 {
		return fmt.Errorf("ARM32 interp blob too small: %d bytes", len(blob))
	}
	entryOff := uint64(binary.LittleEndian.Uint32(blob[:4]))
	tokenEntryOff := uint64(binary.LittleEndian.Uint32(blob[4:8]))
	tokenTableVAOff := uint64(binary.LittleEndian.Uint32(blob[8:12]))
	interpCode := blob[12:]
	_ = entryOff

	// 1. payload
	payload := make([]byte, 0, len(interpCode)+1024)
	payload = append(payload, interpCode...)
	for len(payload)%4 != 0 {
		payload = append(payload, 0x00)
	}

	type bcRecord struct {
		payloadOff int
		bcLen      int
	}
	records := make([]bcRecord, len(funcs))
	for i, fb := range funcs {
		records[i].payloadOff = len(payload)
		records[i].bcLen = len(fb.Encrypted)
		payload = append(payload, fb.Encrypted...)
		for len(payload)%4 != 0 {
			payload = append(payload, 0x00)
		}
	}

	// 2. 页对齐追加
	appendOff := uint64(len(p.data))
	padLen := (0x1000 - (appendOff % 0x1000)) % 0x1000
	for i := uint64(0); i < padLen; i++ {
		p.data = append(p.data, 0x00)
	}
	payloadFileOff := uint32(len(p.data))

	// 动态计算 payloadVA (ELF32 uses 32-bit addresses)
	var maxVA uint32
	for i := 0; i < int(ehdr.Phnum); i++ {
		phOff := ehdr.Phoff + uint32(i)*uint32(ehdr.Phentsize)
		ph := readPhdr32(p.data, phOff)
		if ph.Type == uint32(elf.PT_LOAD) {
			end := ph.Vaddr + ph.Memsz
			if end > maxVA {
				maxVA = end
			}
		}
	}
	payloadVA := (maxVA + 0xFFFF) &^ 0xFFFF

	p.data = append(p.data, payload...)

	fmt.Printf("    Payload at file offset: 0x%X, VA: 0x%X, size: %d\n",
		payloadFileOff, payloadVA, len(payload))

	for i, fb := range funcs {
		bcVA := payloadVA + uint32(records[i].payloadOff)
		fmt.Printf("    [%s] bytecode VA: 0x%X, len: %d\n",
			fb.FI.Name, bcVA, records[i].bcLen)
	}

	// 3. 找到 PT_NOTE
	noteIdx := -1
	for i := 0; i < int(ehdr.Phnum); i++ {
		phOff := ehdr.Phoff + uint32(i)*uint32(ehdr.Phentsize)
		ph := readPhdr32(p.data, phOff)
		if ph.Type == uint32(elf.PT_NOTE) {
			noteIdx = i
			break
		}
	}
	if noteIdx < 0 {
		return fmt.Errorf("PT_NOTE segment not found")
	}

	// 4. PT_NOTE → PT_LOAD (RX)
	notePhdrOff := ehdr.Phoff + uint32(noteIdx)*uint32(ehdr.Phentsize)
	newPhdr := elf32Phdr{
		Type:   uint32(elf.PT_LOAD),
		Off:    payloadFileOff,
		Vaddr:  payloadVA,
		Paddr:  payloadVA,
		Filesz: uint32(len(payload)),
		Memsz:  uint32(len(payload)),
		Flags:  uint32(elf.PF_R | elf.PF_X),
		Align:  0x1000,
	}
	writePhdr32(p.data, notePhdrOff, newPhdr)

	fmt.Printf("    PT_NOTE[%d] -> PT_LOAD RX: off=0x%X va=0x%X sz=0x%X\n",
		noteIdx, payloadFileOff, payloadVA, len(payload))

	// 4b. 按 Vaddr 升序重排 PT_LOAD
	{
		type phdrSlot struct {
			idx  int
			phdr elf32Phdr
		}
		var loads []phdrSlot
		for i := 0; i < int(ehdr.Phnum); i++ {
			off := ehdr.Phoff + uint32(i)*uint32(ehdr.Phentsize)
			ph := readPhdr32(p.data, off)
			if ph.Type == uint32(elf.PT_LOAD) {
				loads = append(loads, phdrSlot{idx: i, phdr: ph})
			}
		}
		needSort := false
		for k := 1; k < len(loads); k++ {
			if loads[k].phdr.Vaddr < loads[k-1].phdr.Vaddr {
				needSort = true
				break
			}
		}
		if needSort {
			sort.Slice(loads, func(a, b int) bool {
				return loads[a].phdr.Vaddr < loads[b].phdr.Vaddr
			})
			slotIndices := make([]int, len(loads))
			for k := range loads {
				slotIndices[k] = loads[k].idx
			}
			sort.Ints(slotIndices)
			for k, si := range slotIndices {
				off := ehdr.Phoff + uint32(si)*uint32(ehdr.Phentsize)
				writePhdr32(p.data, off, loads[k].phdr)
			}
			fmt.Printf("    [PHDR] Reordered %d PT_LOAD segments by Vaddr ascending\n", len(loads))
			for i := 0; i < int(ehdr.Phnum); i++ {
				off := ehdr.Phoff + uint32(i)*uint32(ehdr.Phentsize)
				ph := readPhdr32(p.data, off)
				if ph.Type == uint32(elf.PT_LOAD) && ph.Vaddr == payloadVA {
					notePhdrOff = off
					break
				}
			}
		}
	}

	// 5. Token 跳板 (ARM32)
	for len(payload)%4 != 0 {
		payload = append(payload, 0x00)
	}
	tokenTableOff := len(payload)
	tokenTableVA32 := payloadVA + uint32(tokenTableOff)

	// ARM32 token_desc_t: bc_off(u32) + bc_len(u32) = 8 bytes per entry
	selfVA32 := payloadVA + uint32(tokenTableVAOff)
	for i := range funcs {
		bcVA := payloadVA + uint32(records[i].payloadOff)
		bcLen := uint32(records[i].bcLen)

		var desc [8]byte
		binary.LittleEndian.PutUint32(desc[0:], bcVA-selfVA32)
		binary.LittleEndian.PutUint32(desc[4:], bcLen)
		payload = append(payload, desc[:]...)
	}

	newPhdr.Filesz = uint32(len(payload))
	newPhdr.Memsz = uint32(len(payload))
	writePhdr32(p.data, notePhdrOff, newPhdr)

	p.data = p.data[:payloadFileOff]
	p.data = append(p.data, payload...)

	tblRelOff := tokenTableVA32 - selfVA32
	binary.LittleEndian.PutUint32(p.data[payloadFileOff+uint32(tokenTableVAOff):], tblRelOff)

	// Patch _link_time_self_va (word right after _token_table_va) with link-time VA
	// so the stub can compute ASLR slide = runtime_self_va - link_time_self_va
	binary.LittleEndian.PutUint32(p.data[payloadFileOff+uint32(tokenTableVAOff)+4:], selfVA32)

	fmt.Printf("    [TOKEN] descriptor table VA: 0x%X, entries: %d\n", tokenTableVA32, len(funcs))
	fmt.Printf("    [TOKEN] _token_table_va patched at blob offset 0x%X → relative offset 0x%X (PIE)\n", tokenTableVAOff, tblRelOff)
	fmt.Printf("    [TOKEN] _link_time_self_va patched → 0x%X\n", selfVA32)

	vmEntryTokenVA := payloadVA + uint32(tokenEntryOff)
	fmt.Printf("    [TOKEN] vm_entry_token VA: 0x%X\n", vmEntryTokenVA)

	for i, fb := range funcs {
		funcID := uint32(i)
		token := (uint32(fb.XorKey) << 24) | (0 << 12) | (funcID & 0xFFF)

		isThumb := p.thumbFuncs[fb.FI.Addr]
		var trampoline []byte
		if isThumb {
			trampoline = BuildTokenTrampolineThumb(uint32(fb.FI.Addr), vmEntryTokenVA, token)
		} else {
			trampoline = BuildTokenTrampolineARM32(uint32(fb.FI.Addr), vmEntryTokenVA, token)
		}
		if uint64(len(trampoline)) > fb.FI.Size {
			return fmt.Errorf("token trampoline for %s (%d bytes) exceeds function size (%d bytes)",
				fb.FI.Name, len(trampoline), fb.FI.Size)
		}

		for j := 0; j < len(trampoline); j++ {
			p.data[fb.FI.Offset+uint64(j)] = trampoline[j]
		}

		garbageLen := int(fb.FI.Size) - len(trampoline)
		if garbageLen > 0 {
			garbage := make([]byte, garbageLen)
			rand.Read(garbage)
			copy(p.data[fb.FI.Offset+uint64(len(trampoline)):], garbage)
		}

		mode := "ARM"
		if isThumb {
			mode = "Thumb"
		}
		fmt.Printf("    [TOKEN] %s: func_id=%d, token=0x%08X, trampoline=%d bytes (%s)\n",
			fb.FI.Name, funcID, token, len(trampoline), mode)
	}

	return nil
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
func reverseInstructions(bytecode []byte, codeLen int) ([]byte, map[int]int) {
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
	var output []byte
	for i := len(insts) - 1; i >= 0; i-- {
		inst := insts[i]
		output = append(output, bytecode[inst.offset:inst.offset+inst.size]...)
		output = append(output, byte(inst.size))
		// offsetMap points to where this instruction ends (after size marker)
		// because the reverse DISPATCH will start here to locate the instruction.
		offsetMap[inst.offset] = len(output)
	}

	return output, offsetMap
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
