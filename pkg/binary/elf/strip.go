package elf

import (
	"encoding/binary"
	"fmt"
)

// shLayout describes the section header field layout for ELF32 or ELF64
type shLayout struct {
	entrySize   int  // section header entry size (40 or 64)
	offsetOff   int  // sh_offset field offset within entry
	sizeOff     int  // sh_size field offset within entry
	linkOff     int  // sh_link field offset within entry
	clearFields []int // field offsets to zero when stripping
}

var (
	shLayout64 = shLayout{
		entrySize:   64,
		offsetOff: 24, sizeOff: 32, linkOff: 40,
		clearFields: []int{4, 8, 16, 24, 32, 40, 44, 48, 56},
	}
	shLayout32 = shLayout{
		entrySize:   40,
		offsetOff: 16, sizeOff: 20, linkOff: 24,
		clearFields: []int{4, 8, 12, 16, 20, 24, 28, 32, 36},
	}
)

// stripNames lists sections to remove (equivalent to strip -s)
var stripNames = map[string]bool{
	".symtab": true, ".strtab": true, ".comment": true,
	".note.GNU-stack": true, ".note.gnu.build-id": true,
}

// stripSections 就地清除符号/调试 section
func (p *Packer) stripSections() {
	if p.isARM32 {
		p.stripSectionsImpl(shLayout32)
	} else {
		p.stripSectionsImpl(shLayout64)
	}
}

// stripContext holds parsed section header state for the strip operation
type stripContext struct {
	data     []byte
	verbose  bool
	shNum    uint32
	shOff    uint64
	shEntSz  uint64
	layout   shLayout
	strOff   uint64 // shstrtab section offset in file
	strSz    uint64 // shstrtab section size
	readU64  func(uint64) uint64
	readU32  func(uint64) uint32
	putU32   func(uint64, uint32)
	putU64   func(uint64, uint64)
}

func (p *Packer) stripSectionsImpl(layout shLayout) {
	shNum, shOff, shEntSz, shstrndx, readU64, readU32, putU32, putU64 := p.stripParams()
	if shNum == 0 {
		return
	}

	shTableEnd := shOff + uint64(shNum)*uint64(shEntSz)
	if shOff == 0 || shTableEnd > uint64(len(p.data)) {
		if p.verbose {
			fmt.Printf("    [strip] Section header table missing or truncated, skipping strip\n")
		}
		return
	}
	if uint32(shstrndx) >= uint32(shNum) {
		return
	}

	shstrEntryOff := shOff + uint64(shstrndx)*uint64(shEntSz)
	strOff := readU64(shstrEntryOff + uint64(layout.offsetOff))
	strSz := readU64(shstrEntryOff + uint64(layout.sizeOff))
	if strOff+strSz > uint64(len(p.data)) {
		return
	}

	ctx := &stripContext{
		data: p.data, verbose: p.verbose,
		shNum: shNum, shOff: shOff, shEntSz: shEntSz,
		layout: layout, strOff: strOff, strSz: strSz,
		readU64: readU64, readU32: readU32, putU32: putU32, putU64: putU64,
	}

	stripped := ctx.collectStripped()
	ctx.applyStripping(stripped)
}

// sectionName reads a null-terminated section name from the shstrtab
func (ctx *stripContext) sectionName(nameOff uint32) string {
	start := ctx.strOff + uint64(nameOff)
	if start >= uint64(len(ctx.data)) || start >= ctx.strOff+ctx.strSz {
		return ""
	}
	end := start
	for end < ctx.strOff+ctx.strSz && end < uint64(len(ctx.data)) && ctx.data[end] != 0 {
		end++
	}
	return string(ctx.data[start:end])
}

// collectStripped scans all sections and returns indices of those to strip
func (ctx *stripContext) collectStripped() map[int]bool {
	stripped := make(map[int]bool)
	for i := 0; i < int(ctx.shNum); i++ {
		entryOff := ctx.shOff + uint64(i)*uint64(ctx.shEntSz)
		name := ctx.sectionName(ctx.readU32(entryOff))
		if stripNames[name] {
			stripped[i] = true
		}
	}
	return stripped
}

// applyStripping zeroes stripped section content/headers and fixes sh_link references
func (ctx *stripContext) applyStripping(stripped map[int]bool) {
	for i := 0; i < int(ctx.shNum); i++ {
		entryOff := ctx.shOff + uint64(i)*uint64(ctx.shEntSz)

		if stripped[i] {
			secOff := ctx.readU64(entryOff + uint64(ctx.layout.offsetOff))
			secSz := ctx.readU64(entryOff + uint64(ctx.layout.sizeOff))

			if secOff+secSz <= uint64(len(ctx.data)) {
				for j := uint64(0); j < secSz; j++ {
					ctx.data[secOff+j] = 0
				}
			}

			if ctx.verbose {
				name := ctx.sectionName(ctx.readU32(entryOff))
				fmt.Printf("    [strip] %s cleared (off=0x%X, sz=%d)\n", name, secOff, secSz)
			}

			for _, fieldOff := range ctx.layout.clearFields {
				ctx.putU64(entryOff+uint64(fieldOff), 0)
			}
		} else {
			linkVal := ctx.readU32(entryOff + uint64(ctx.layout.linkOff))
			if linkVal > 0 && stripped[int(linkVal)] {
				ctx.putU32(entryOff+uint64(ctx.layout.linkOff), 0)
				if ctx.verbose {
					name := ctx.sectionName(ctx.readU32(entryOff))
					fmt.Printf("    [strip] %s: sh_link %d → 0 (target stripped)\n", name, linkVal)
				}
			}
		}
	}
}

// stripParams returns arch-specific section header parameters and read/write helpers
func (p *Packer) stripParams() (shNum uint32, shOff uint64, shEntSize uint64, shstrndx uint32,
	readU64 func(uint64) uint64, readU32 func(uint64) uint32,
	putU32 func(uint64, uint32), putU64 func(uint64, uint64)) {

	if p.isARM32 {
		ehdr := readEhdr32(p.data)
		return ehdr.Shnum, uint64(ehdr.Shoff), uint64(ehdr.Shentsize), ehdr.Shstrndx,
			func(off uint64) uint64 { return uint64(binary.LittleEndian.Uint32(p.data[off:])) },
			func(off uint64) uint32 { return binary.LittleEndian.Uint32(p.data[off:]) },
			func(off uint64, v uint32) { binary.LittleEndian.PutUint32(p.data[off:], v) },
			func(off uint64, v uint64) { binary.LittleEndian.PutUint32(p.data[off:], uint32(v)) }
	}

	ehdr := readEhdr64(p.data)
	return ehdr.Shnum, ehdr.Shoff, uint64(ehdr.Shentsize), ehdr.Shstrndx,
		func(off uint64) uint64 { return binary.LittleEndian.Uint64(p.data[off:]) },
		func(off uint64) uint32 { return binary.LittleEndian.Uint32(p.data[off:]) },
		func(off uint64, v uint32) { binary.LittleEndian.PutUint32(p.data[off:], v) },
		func(off uint64, v uint64) { binary.LittleEndian.PutUint64(p.data[off:], v) }
}
