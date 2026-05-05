package elf

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"github.com/vmpacker/pkg/vm"
)

// execSection holds info about an executable code region (.text or LOAD_X segment)
type execSection struct {
	name   string
	addr   uint64
	offset uint64
	size   uint64
	data   []byte
}

// ParseAddrSpec parses address spec: "0xADDR", "0xSTART-0xEND", "0xSTART-0xEND:name"
func ParseAddrSpec(s string) (AddrSpec, error) {
	var spec AddrSpec
	// Separate optional name (after last colon)
	if idx := strings.LastIndex(s, ":"); idx > 2 {
		candidate := s[idx+1:]
		// If not a hex number, treat as name
		if _, err := strconv.ParseUint(candidate, 0, 64); err != nil {
			spec.Name = candidate
			s = s[:idx]
		}
	}
	// Parse address range
	if parts := strings.Split(s, "-"); len(parts) == 2 {
		start, err := strconv.ParseUint(parts[0], 0, 64)
		if err != nil {
			return spec, fmt.Errorf("invalid start address: %s", parts[0])
		}
		end, err := strconv.ParseUint(parts[1], 0, 64)
		if err != nil {
			return spec, fmt.Errorf("invalid end address: %s", parts[1])
		}
		if end <= start {
			return spec, fmt.Errorf("end address must be greater than start address")
		}
		spec.Addr = start
		spec.End = end
	} else {
		addr, err := strconv.ParseUint(s, 0, 64)
		if err != nil {
			return spec, fmt.Errorf("invalid address: %s", s)
		}
		spec.Addr = addr
	}
	if spec.Name == "" {
		spec.Name = fmt.Sprintf("sub_%X", spec.Addr)
	}
	return spec, nil
}

// FindFunction finds a function in ELF and computes its correct file offset
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

			offset, secName, found := resolveFileOffset(f, addr, sym.Section)
			if !found {
				return nil, fmt.Errorf("could not determine file offset for function %s at 0x%X", name, addr)
			}
			info.Offset = offset
			info.Section = secName
			return info, nil
		}
	}
	return nil, fmt.Errorf("function '%s' not found", name)
}

// resolveFileOffset finds file offset for a virtual address, checking PT_LOAD first then sections
func resolveFileOffset(f *elf.File, addr uint64, secIdx elf.SectionIndex) (uint64, string, bool) {
	// Prefer Program Headers (required for Android .so)
	for _, ph := range f.Progs {
		if ph.Type == elf.PT_LOAD && (ph.Flags&elf.PF_X != 0) {
			if addr >= ph.Vaddr && addr < ph.Vaddr+ph.Memsz {
				return ph.Off + (addr - ph.Vaddr), "__LOAD_X", true
			}
		}
	}

	// Fallback to Section Headers
	if int(secIdx) < len(f.Sections) {
		sec := f.Sections[secIdx]
		if addr >= sec.Addr {
			return sec.Offset + (addr - sec.Addr), sec.Name, true
		}
	}

	return 0, "", false
}

// FindFunctionByAddr finds function by address
func (p *Packer) FindFunctionByAddr(f *elf.File, spec AddrSpec) (*vm.FuncInfo, error) {
	sec, err := findExecSection(f, spec.Addr)
	if err != nil {
		return nil, err
	}

	if spec.Addr < sec.addr || spec.Addr >= sec.addr+sec.size {
		return nil, fmt.Errorf("address 0x%X not in %s (0x%X-0x%X)",
			spec.Addr, sec.name, sec.addr, sec.addr+sec.size)
	}

	var size uint64
	if spec.End > 0 {
		size = spec.End - spec.Addr
	} else {
		startOff := spec.Addr - sec.addr
		size, err = p.detectFunctionSize(sec.data, startOff, spec.Addr)
		if err != nil {
			return nil, err
		}
	}

	fi := &vm.FuncInfo{
		Name:    spec.Name,
		Addr:    spec.Addr,
		Size:    size,
		Section: sec.name,
		Offset:  sec.offset + (spec.Addr - sec.addr),
	}
	if fi.Offset >= uint64(len(p.data)) {
		return nil, fmt.Errorf("calculated file offset 0x%X for 0x%X is out of bounds (file size 0x%X)",
			fi.Offset, fi.Addr, len(p.data))
	}
	return fi, nil
}

// findExecSection locates the executable section (.text or LOAD_X segment) containing addr
func findExecSection(f *elf.File, addr uint64) (*execSection, error) {
	// Try .text section first, but only if the address is within it
	textSec := f.Section(".text")
	if textSec != nil && addr >= textSec.Addr && addr < textSec.Addr+textSec.Size {
		d, err := textSec.Data()
		if err != nil {
			return nil, fmt.Errorf("reading .text failed: %v", err)
		}
		return &execSection{
			name: ".text", addr: textSec.Addr,
			offset: textSec.Offset, size: textSec.Size, data: d,
		}, nil
	}

	// Fallback: executable LOAD segment containing the address
	for _, prog := range f.Progs {
		if prog.Type != elf.PT_LOAD || prog.Flags&elf.PF_X == 0 {
			continue
		}
		segEnd := prog.Vaddr + prog.Memsz
		if addr >= prog.Vaddr && addr < segEnd {
			d := make([]byte, prog.Filesz)
			if _, err := prog.ReadAt(d, 0); err != nil {
				return nil, fmt.Errorf("reading LOAD segment failed: %v", err)
			}
			return &execSection{
				name: "__LOAD_X", addr: prog.Vaddr,
				offset: prog.Off, size: prog.Filesz, data: d,
			}, nil
		}
	}

	return nil, fmt.Errorf("address 0x%X not in any executable segment", addr)
}

// detectFunctionSize scans code for the function's return instruction to determine its size
func (p *Packer) detectFunctionSize(secData []byte, startOff uint64, addr uint64) (uint64, error) {
	if p.isARM32 {
		return detectARM32FunctionEnd(secData, startOff, addr, p.thumbFuncs[addr])
	}
	return detectARM64FunctionEnd(secData, startOff, addr)
}

func detectARM64FunctionEnd(secData []byte, startOff uint64, addr uint64) (uint64, error) {
	for i := startOff; i+4 <= uint64(len(secData)); i += 4 {
		inst := binary.LittleEndian.Uint32(secData[i:])
		if inst == 0xD65F03C0 { // RET
			return i + 4 - startOff, nil
		}
	}
	return 0, fmt.Errorf("cannot detect function size at 0x%X (no RET found)", addr)
}

func detectARM32FunctionEnd(secData []byte, startOff uint64, addr uint64, isThumb bool) (uint64, error) {
	if isThumb {
		for i := startOff; i+2 <= uint64(len(secData)); i += 2 {
			hw := binary.LittleEndian.Uint16(secData[i:])
			if hw == 0x4770 { // BX LR
				return i + 2 - startOff, nil
			}
			if hw&0xFF00 == 0xBD00 { // POP {..., PC}
				return i + 2 - startOff, nil
			}
		}
	} else {
		for i := startOff; i+4 <= uint64(len(secData)); i += 4 {
			inst := binary.LittleEndian.Uint32(secData[i:])
			if inst == 0xE12FFF1E { // BX LR
				return i + 4 - startOff, nil
			}
			if inst&0xFFFF8000 == 0xE8BD8000 { // LDMFD SP!, {..., PC}
				return i + 4 - startOff, nil
			}
		}
	}
	return 0, fmt.Errorf("cannot detect function size at 0x%X (no BX LR / POP {PC} found)", addr)
}

// ExtractFuncCode extracts function machine code
func (p *Packer) ExtractFuncCode(f *elf.File, fi *vm.FuncInfo) ([]byte, error) {
	if fi.Size == 0 {
		return nil, fmt.Errorf("function %s has zero size", fi.Name)
	}

	if fi.Section == "__LOAD_X" {
		// No section headers: read from LOAD segment
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
