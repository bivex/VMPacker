package elf

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// MangleSymbols renames protected symbols in .dynsym/.symtab to random strings
func (p *Packer) MangleSymbols() {
	if p.isARM32 {
		p.mangleSymbolsImpl(shLayout32)
	} else {
		p.mangleSymbolsImpl(shLayout64)
	}
}

func (p *Packer) mangleSymbolsImpl(layout shLayout) {
	shNum, shOff, shEntSz, _, readU64, readU32, _, _ := p.stripParams()
	if shNum == 0 { return }

	// 1. Find .symtab, .strtab, .dynsym, .dynstr
	var symtabOff, symtabSz, strtabOff, strtabSz uint64
	var dynsymOff, dynsymSz, dynstrOff, dynstrSz uint64
	var symtabEntSz, dynsymEntSz uint64

	_, _, _, shstrndx, _, _, _, _ := p.stripParams()
	shstrEntryOff := shOff + uint64(shstrndx)*uint64(shEntSz)
	shstrOff := readU64(shstrEntryOff + uint64(layout.offsetOff))
	shstrSz := readU64(shstrEntryOff + uint64(layout.sizeOff))

	getSecName := func(nameOff uint32) string {
		start := shstrOff + uint64(nameOff)
		if start >= uint64(len(p.data)) || start >= shstrOff+shstrSz { return "" }
		end := start
		for end < uint64(len(p.data)) && p.data[end] != 0 { end++ }
		return string(p.data[start:end])
	}

	for i := 0; i < int(shNum); i++ {
		entryOff := shOff + uint64(i)*uint64(shEntSz)
		name := getSecName(readU32(entryOff))
		off := readU64(entryOff + uint64(layout.offsetOff))
		sz := readU64(entryOff + uint64(layout.sizeOff))
		entsz := uint64(0)
		if layout.entrySize == 64 {
			entsz = binary.LittleEndian.Uint64(p.data[entryOff+56:])
		} else {
			entsz = uint64(binary.LittleEndian.Uint32(p.data[entryOff+36:]))
		}

		switch name {
		case ".symtab":
			symtabOff, symtabSz, symtabEntSz = off, sz, entsz
		case ".strtab":
			strtabOff, strtabSz = off, sz
		case ".dynsym":
			dynsymOff, dynsymSz, dynsymEntSz = off, sz, entsz
		case ".dynstr":
			dynstrOff, dynstrSz = off, sz
		}
	}

	// 2. Mangle symbols
	p.mangleTable(dynsymOff, dynsymSz, dynsymEntSz, dynstrOff, dynstrSz, layout.entrySize == 64, true)
	p.mangleTable(symtabOff, symtabSz, symtabEntSz, strtabOff, strtabSz, layout.entrySize == 64, false)
}

func (p *Packer) mangleTable(symOff, symSz, entSz, strOff, strSz uint64, is64 bool, isDyn bool) {
	if symOff == 0 || strOff == 0 || entSz == 0 { return }

	targetNames := make(map[string]bool)
	for _, name := range p.funcNames {
		targetNames[name] = true
	}

	for off := symOff; off < symOff+symSz; off += entSz {
		var nameOff uint32
		if is64 {
			nameOff = binary.LittleEndian.Uint32(p.data[off:])
		} else {
			nameOff = binary.LittleEndian.Uint32(p.data[off:])
		}

		if nameOff == 0 { continue }
		
		sStart := strOff + uint64(nameOff)
		if sStart >= strOff+strSz { continue }
		
		sEnd := sStart
		for sEnd < strOff+strSz && p.data[sEnd] != 0 { sEnd++ }
		name := string(p.data[sStart:sEnd])

		// Only mangle if it's in our protected list
		if targetNames[name] {
			newName := p.generateMangledName(len(name))
			if p.verbose {
				fmt.Printf("    [mangle] %s -> %s (%s)\n", name, newName, func() string {
					if isDyn { return "dyn" }
					return "sym"
				}())
			}
			copy(p.data[sStart:sEnd], newName)
		}
	}
}

func (p *Packer) generateMangledName(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	// Ensure it starts with a letter
	if b[0] >= '0' && b[0] <= '9' {
		b[0] = 'v'
	}
	return string(b)
}
