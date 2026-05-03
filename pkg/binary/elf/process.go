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

// translationResult holds common fields from arch-specific translation
type translationResult struct {
	Bytecode    []byte
	CodeLen     int
	Unsupported []string
	TotalInsts  int
	TransInsts  int
	Relocations []vm.Relocation
}

// Process main entry point
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

	activeBlob, err := p.validateArch(f)
	if err != nil {
		return err
	}

	fmt.Printf("[*] ELF: %s, Type: %s, Class: %s\n", f.Machine, f.Type, f.Class)
	fmt.Printf("[*] VM interp blob: %d bytes (ARM32=%v)\n", len(activeBlob), p.isARM32)

	// Phase 1: collect bytecode for all functions
	entries := p.collectEntries(f)

	var funcs []FuncBytecode
	for _, entry := range entries {
		fmt.Printf("\n[*] Processing: %s\n", entry.name)

		fi, err := entry.finder()
		if err != nil {
			return err
		}
		fmt.Printf("    Addr: 0x%X, Size: %d bytes, Section: %s\n",
			fi.Addr, fi.Size, fi.Section)

		if fi.Size < 12 {
			return fmt.Errorf("function %s is too small (%d bytes) for trampoline injection (minimum 12 bytes); "+
				"consider excluding this function", fi.Name, fi.Size)
		}

		code, err := p.ExtractFuncCode(f, fi)
		if err != nil {
			return err
		}

		isThumbFunc := p.isARM32 && p.thumbFuncs[fi.Addr]
		insts := p.decodeInstructions(code, isThumbFunc)

		result, err := p.translateFunction(fi, code, insts, isThumbFunc)
		if err != nil {
			return err
		}

		fmt.Printf("    Translated: %d/%d\n", result.TransInsts, result.TotalInsts)
		fmt.Printf("    Bytecode: %d bytes\n", len(result.Bytecode))

		if len(result.Unsupported) > 0 {
			p.writeUnsupportedReport(entry.name, fi, result, insts)
			return fmt.Errorf("translation aborted: %d unsupported instruction(s) in %s — cannot produce safe output",
				len(result.Unsupported), entry.name)
		}

		if p.debug {
			p.writeDebugDump(entry.name, fi, result, insts, code, isThumbFunc)
		}

		if len(result.Relocations) > 0 {
			p.relocations = append(p.relocations, result.Relocations...)
		}

		encrypted, xorKey, err := p.postProcessBytecode(result, insts)
		if err != nil {
			return err
		}

		funcs = append(funcs, FuncBytecode{
			FI: fi, Encrypted: encrypted, XorKey: xorKey,
			Relocations: result.Relocations,
		})
	}

	// Phase 2: batch injection (single PT_NOTE hijack)
	fmt.Printf("\n[*] Injecting %d functions...\n", len(funcs))
	err = p.injectVMPBatch(funcs)
	if err != nil {
		return fmt.Errorf("injection failed: %v", err)
	}

	for _, fb := range funcs {
		fmt.Printf("    [+] %s VMP protected\n", fb.FI.Name)
	}

	// Phase 3: strip symbol table (optional)
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

// validateArch detects architecture and returns the active interpreter blob
func (p *Packer) validateArch(f *elf.File) ([]byte, error) {
	switch {
	case f.Machine == elf.EM_AARCH64 && f.Class == elf.ELFCLASS64:
		p.isARM32 = false
	case f.Machine == elf.EM_ARM && f.Class == elf.ELFCLASS32:
		p.isARM32 = true
		if p.thumbFuncs == nil {
			p.thumbFuncs = make(map[uint64]bool)
		}
	default:
		return nil, fmt.Errorf("unsupported arch: machine=%s class=%s (need ARM64/ELF64 or ARM/ELF32)", f.Machine, f.Class)
	}

	activeBlob := p.interpBlob
	if p.isARM32 {
		activeBlob = p.interpBlobARM32
		if len(activeBlob) == 0 {
			return nil, fmt.Errorf("ARM32 ELF detected but no ARM32 interp blob provided")
		}
	}
	return activeBlob, nil
}

type funcEntry struct {
	name   string
	finder func() (*vm.FuncInfo, error)
}

// collectEntries builds the list of functions to process from names and address specs
func (p *Packer) collectEntries(f *elf.File) []funcEntry {
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
	return entries
}

// decodeInstructions dispatches to the correct decoder based on arch
func (p *Packer) decodeInstructions(code []byte, isThumbFunc bool) []vm.Instruction {
	if p.isARM32 {
		return p.DecodeFunctionARM32(code, isThumbFunc)
	}
	return p.DecodeFunction(code)
}

// translateFunction dispatches to ARM32 or ARM64 translator
func (p *Packer) translateFunction(fi *vm.FuncInfo, code []byte, insts []vm.Instruction, isThumbFunc bool) (*translationResult, error) {
	if p.verbose {
		fmt.Printf("    Instructions: %d (Thumb=%v)\n", len(insts), isThumbFunc && p.isARM32)
		p.dumpDisasm(insts)
	} else {
		fmt.Printf("    Instructions: %d\n", len(insts))
	}

	var result translationResult

	if p.isARM32 {
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
			return nil, fmt.Errorf("translation failed: %v", terr)
		}
		result = translationResult{
			Bytecode: r.Bytecode, CodeLen: r.CodeLen,
			Unsupported: r.Unsupported, TotalInsts: r.TotalInsts, TransInsts: r.TransInsts,
			Relocations: r.Relocations,
		}
	} else {
		trans := arm64.NewTranslator(fi.Addr, int(fi.Size))
		if p.debug {
			trans.SetDebug(true)
		}
		r, terr := trans.Translate(insts)
		if terr != nil {
			return nil, fmt.Errorf("translation failed: %v", terr)
		}
		result = translationResult{
			Bytecode: r.Bytecode, CodeLen: r.CodeLen,
			Unsupported: r.Unsupported, TotalInsts: r.TotalInsts, TransInsts: r.TransInsts,
			Relocations: r.Relocations,
		}
	}

	return &result, nil
}

// dumpDisasm prints the instruction disassembly for verbose mode
func (p *Packer) dumpDisasm(insts []vm.Instruction) {
	fmt.Println("    --- Disasm ---")
	if p.isARM32 {
		dec32 := arm32.NewDecoder()
		for _, inst := range insts {
			fmt.Printf("    0x%04X: %-12s raw=0x%08X\n",
				inst.Offset, dec32.InstName(inst.Op), inst.Raw)
		}
	} else {
		dec64 := arm64.NewDecoder()
		for _, inst := range insts {
			fmt.Printf("    0x%04X: %-12s raw=0x%08X\n",
				inst.Offset, dec64.InstName(inst.Op), inst.Raw)
		}
	}
	fmt.Println("    --- End ---")
}

// writeUnsupportedReport generates a debug report file for unsupported instructions
func (p *Packer) writeUnsupportedReport(name string, fi *vm.FuncInfo, result *translationResult, insts []vm.Instruction) {
	fmt.Printf("    [!] Unsupported (%d):\n", len(result.Unsupported))
	for _, u := range result.Unsupported {
		fmt.Printf("        %s\n", u)
	}

	debugPath := p.outputPath + ".debug.txt"
	df, err := os.Create(debugPath)
	if err != nil {
		fmt.Printf("    [!] debug file creation failed: %v\n", err)
		return
	}
	defer df.Close()

	fmt.Fprintf(df, "================================================================\n")
	fmt.Fprintf(df, "Translation failure report — %s @ 0x%X\n", name, fi.Addr)
	fmt.Fprintf(df, "Function size: %d bytes, total insts: %d, translated: %d\n",
		fi.Size, result.TotalInsts, result.TransInsts)
	fmt.Fprintf(df, "================================================================\n\n")
	fmt.Fprintf(df, "Unsupported instructions (%d):\n\n", len(result.Unsupported))

	instMap := make(map[int]vm.Instruction)
	for _, inst := range insts {
		instMap[inst.Offset] = inst
	}

	for idx, u := range result.Unsupported {
		fmt.Fprintf(df, "[%d] %s\n", idx+1, u)

		var off int
		if _, err := fmt.Sscanf(u, "offset 0x%X:", &off); err == nil {
			if inst, ok := instMap[off]; ok {
				raw := inst.Raw
				fmt.Fprintf(df, "    raw bytes: %02X %02X %02X %02X\n",
					byte(raw), byte(raw>>8), byte(raw>>16), byte(raw>>24))
				fmt.Fprintf(df, "    absolute addr: 0x%X\n", fi.Addr+uint64(off))
			}
		}
		fmt.Fprintln(df)
	}

	archName := "arm64"
	if p.isARM32 {
		archName = "arm32"
	}
	fmt.Fprintf(df, "================================================================\n")
	fmt.Fprintf(df, "Fix suggestions:\n")
	fmt.Fprintf(df, "- Write a demo test case for each unsupported insn (see demo/ dir)\n")
	fmt.Fprintf(df, "- Add corresponding case in pkg/arch/%s/translator.go translateOne()\n", archName)
	fmt.Fprintf(df, "- Use -v flag to see full disassembly context\n")
	fmt.Fprintf(df, "================================================================\n")

	fmt.Printf("    [+] translation failure debug file: %s\n", debugPath)
}

// writeDebugDump generates a side-by-side ARM→VM debug comparison file
func (p *Packer) writeDebugDump(name string, fi *vm.FuncInfo, result *translationResult, insts []vm.Instruction, code []byte, isThumbFunc bool) {
	debugPath := p.outputPath + ".debug.txt"
	df, err := os.OpenFile(debugPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("    [!] debug file create failed: %v\n", err)
		return
	}
	defer df.Close()

	fmt.Fprintf(df, "================================================================\n")
	fmt.Fprintf(df, "Function: %s @ 0x%X (size: %d)\n", name, fi.Addr, fi.Size)
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

	fmt.Printf("    [+] Debug: %s\n", debugPath)
}

// postProcessBytecode applies reverse, opcode encryption, RTLR remap, and XOR chain encryption
func (p *Packer) postProcessBytecode(result *translationResult, insts []vm.Instruction) ([]byte, byte, error) {
	// ---- PC reverse traversal: reverse instruction order ----
	reversed, offsetMap, byteMap := reverseInstructions(result.Bytecode, result.CodeLen)

	newCodeLen := len(reversed)
	remapBranchTargets(reversed, newCodeLen, offsetMap, p.verbose)

	// Remap vm_off in addr_map (BR indirect jumps)
	mapCount := binary.LittleEndian.Uint32(result.Bytecode[len(result.Bytecode)-16:])
	trailerStart := result.CodeLen
	for j := 0; j < int(mapCount); j++ {
		entryOff := trailerStart + j*8
		vmOff := binary.LittleEndian.Uint32(result.Bytecode[entryOff+4:])
		if newVmOff, ok := offsetMap[int(vmOff)]; ok {
			binary.LittleEndian.PutUint32(result.Bytecode[entryOff+4:], uint32(newVmOff))
		}
	}

	// Replace original instruction area with reversed bytecode, keep trailer
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

	// ---- OpcodeCryptor: per-instruction opcode encryption ----
	var ocKeyBuf [4]byte
	if _, err := rand.Read(ocKeyBuf[:]); err != nil {
		return nil, 0, fmt.Errorf("generating oc_key failed: %v", err)
	}
	ocKey := binary.LittleEndian.Uint32(ocKeyBuf[:])

	encryptOpcodes(result.Bytecode, result.CodeLen, ocKey, true)

	reverseOffset := result.CodeLen + int(mapCount)*8
	result.Bytecode[reverseOffset] = 1
	ocKeyOffset := reverseOffset + 1
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

	// ---- XOR chain encryption (whole bytecode segment) ----
	xorKey := byte(0xA5)
	encrypted := make([]byte, len(result.Bytecode))
	for i, b := range result.Bytecode {
		encrypted[i] = b ^ xorKey
	}

	return encrypted, xorKey, nil
}
