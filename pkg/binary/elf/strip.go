package elf

import (
	"encoding/binary"
	"fmt"
)

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
