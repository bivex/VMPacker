package elf

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"github.com/vmpacker/pkg/vm"
)

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
				if inst&0xFFFF8000 == 0xE8BD8000 { // LDMFD SP!, {..., PC}
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
