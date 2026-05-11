package elf

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sort"

	"debug/elf"
)

// bcRecord tracks bytecode position within the payload
type bcRecord struct {
	payloadOff int
	bcLen      int
}

// injectVMPBatch dispatches to arch-specific injection
func (p *Packer) injectVMPBatch(funcs []FuncBytecode, activeBlob []byte) error {
	if p.isARM32 {
		return p.injectVMPBatch32(funcs, activeBlob)
	}
	return p.injectVMPBatch64(funcs, activeBlob)
}

// ---- Shared payload construction ----

// buildPayload constructs [interpCode][padding][bc0][pad][bc1][pad][...]
func buildPayload(interpCode []byte, funcs []FuncBytecode, extraGap int) ([]byte, []bcRecord) {
	payload := make([]byte, 0, len(interpCode)+1024)
	payload = append(payload, interpCode...)

	if extraGap > 0 {
		payload = append(payload, make([]byte, extraGap)...)
	}
	for len(payload)%4 != 0 {
		payload = append(payload, 0x00)
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
	return payload, records
}

// pageAlignAppend pads data to page boundary and appends payload, returns (payloadFileOff, payloadVA)
func pageAlignAppend(data []byte, payload []byte, maxVA uint64, is64 bool) ([]byte, uint64, uint64) {
	appendOff := uint64(len(data))
	padLen := (0x1000 - (appendOff % 0x1000)) % 0x1000
	for i := uint64(0); i < padLen; i++ {
		data = append(data, 0x00)
	}
	payloadFileOff := uint64(len(data))
	payloadVA := (maxVA + 0xFFFF) &^ 0xFFFF
	data = append(data, payload...)
	return data, payloadFileOff, payloadVA
}

// writeTrampolines writes token trampolines and fills remaining bytes with garbage
func writeTrampolines(data []byte, funcs []FuncBytecode, vmEntryTokenVA uint64, tokenVA uint32, buildFn func(int, FuncBytecode) []byte) error {
	for i, fb := range funcs {
		funcID := uint32(i)
		token := (uint32(fb.XorKey) << 24) | (0 << 12) | (funcID & 0xFFF)

		trampoline := buildFn(i, fb)
		if uint64(len(trampoline)) > fb.FI.Size {
			return fmt.Errorf("token trampoline for %s (%d bytes) exceeds function size (%d bytes)",
				fb.FI.Name, len(trampoline), fb.FI.Size)
		}

		for j := 0; j < len(trampoline); j++ {
			data[fb.FI.Offset+uint64(j)] = trampoline[j]
		}

		garbageLen := int(fb.FI.Size) - len(trampoline)
		if garbageLen > 0 {
			garbage := make([]byte, garbageLen)
			rand.Read(garbage)
			copy(data[fb.FI.Offset+uint64(len(trampoline)):], garbage)
		}

		fmt.Printf("    [TOKEN] %s: func_id=%d, token=0x%08X, trampoline=%d bytes\n",
			fb.FI.Name, funcID, token, len(trampoline))
	}
	return nil
}

// ---- ARM64 (ELF64) injection ----

// injectVMPBatch64 — ARM64 and x86_64 ELF64 injection
func (p *Packer) injectVMPBatch64(funcs []FuncBytecode, activeBlob []byte) error {
	ehdr := readEhdr64(p.data)

	if len(activeBlob) < 24 {
		return fmt.Errorf("token mode requires extended blob header (24 bytes), got %d", len(activeBlob))
	}
	_ = binary.LittleEndian.Uint64(activeBlob[:8])
	tokenEntryOff := binary.LittleEndian.Uint64(activeBlob[8:16])
	tokenTableVAOff := binary.LittleEndian.Uint64(activeBlob[16:24])
	interpCode := activeBlob[24:]

	// 1. Build payload with 8-byte RTLr gap
	payload, records := buildPayload(interpCode, funcs, 8)

	// 2. Page-align and append
	maxVA := findMaxVA64(p.data, ehdr)
	var payloadFileOff, payloadVA uint64
	p.data, payloadFileOff, payloadVA = pageAlignAppend(p.data, payload, maxVA, true)

	fmt.Printf("    Payload at file offset: 0x%X, VA: 0x%X, size: %d\n",
		payloadFileOff, payloadVA, len(payload))
	for i, fb := range funcs {
		bcVA := payloadVA + uint64(records[i].payloadOff)
		fmt.Printf("    [%s] bytecode VA: 0x%X, len: %d\n", fb.FI.Name, bcVA, records[i].bcLen)
	}

	// 3. Hijack PT_NOTE → PT_LOAD
	noteIdx, err := findPTNOTE64(p.data, ehdr)
	if err != nil {
		return err
	}
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

	notePhdrOff = reorderPTLoads64(p.data, ehdr, payloadVA, notePhdrOff)

	// 4. Token descriptor table (16 bytes per entry)
	for len(payload)%8 != 0 {
		payload = append(payload, 0x00)
	}
	tokenTableOff := len(payload)
	tokenTableVA := payloadVA + uint64(tokenTableOff)
	selfVA := payloadVA // Base of interpreter stub

	for i := range funcs {
		bcVA := payloadVA + uint64(records[i].payloadOff)
		bcLen := uint32(records[i].bcLen)
		var desc [16]byte
		binary.LittleEndian.PutUint64(desc[0:], bcVA-selfVA)
		binary.LittleEndian.PutUint32(desc[8:], bcLen)
		binary.LittleEndian.PutUint32(desc[12:], 0)
		payload = append(payload, desc[:]...)
	}

	// 5. RTLR relocation table
	rtlrOff := buildRTLrTable(&payload, funcs)
	totalRelocs := 0
	for _, fb := range funcs {
		totalRelocs += len(fb.Relocations)
	}

	newPhdr.Filesz = uint64(len(payload))
	newPhdr.Memsz = uint64(len(payload))
	writePhdr64(p.data, notePhdrOff, newPhdr)

	p.data = p.data[:payloadFileOff]
	p.data = append(p.data, payload...)

	patchTokenHeader64(p.data, payloadFileOff, tokenTableVAOff, selfVA, tokenTableVA, rtlrOff)

	fmt.Printf("    [TOKEN] descriptor table VA: 0x%X, entries: %d\n", tokenTableVA, len(funcs))
	fmt.Printf("    [TOKEN] RTLR table at offset 0x%X in payload, %d relocs\n", rtlrOff, totalRelocs)

	vmEntryTokenVA := payloadVA + tokenEntryOff
	fmt.Printf("    [TOKEN] vm_entry_token VA: 0x%X\n", vmEntryTokenVA)

	return writeTrampolines(p.data, funcs, vmEntryTokenVA, 0, func(i int, fb FuncBytecode) []byte {
		token := (uint32(fb.XorKey) << 24) | (uint32(i) & 0xFFF)
		if p.isX86_64 {
			return BuildTokenTrampolineX86_64(fb.FI.Addr, vmEntryTokenVA, token)
		}
		return BuildTokenTrampoline(fb.FI.Addr, vmEntryTokenVA, token)
	})
}

// ---- ARM32 (ELF32) injection ----

// injectVMPBatch32 — ARM32 ELF32 injection
func (p *Packer) injectVMPBatch32(funcs []FuncBytecode, activeBlob []byte) error {
	ehdr := readEhdr32(p.data)
	blob := activeBlob

	if len(blob) < 12 {
		return fmt.Errorf("ARM32 interp blob too small: %d bytes", len(blob))
	}
	_ = uint64(binary.LittleEndian.Uint32(blob[:4]))
	tokenEntryOff := uint64(binary.LittleEndian.Uint32(blob[4:8]))
	tokenTableVAOff := uint64(binary.LittleEndian.Uint32(blob[8:12]))
	interpCode := blob[12:]

	// 1. Build payload (no RTLr gap for ARM32)
	payload, records := buildPayload(interpCode, funcs, 0)

	// 2. Page-align and append
	maxVA := findMaxVA32(p.data, ehdr)
	var payloadFileOffU, payloadVAU uint64
	p.data, payloadFileOffU, payloadVAU = pageAlignAppend(p.data, payload, maxVA, false)
	payloadFileOff := uint32(payloadFileOffU)
	payloadVA := uint32(payloadVAU)

	fmt.Printf("    Payload at file offset: 0x%X, VA: 0x%X, size: %d\n",
		payloadFileOff, payloadVA, len(payload))
	for i, fb := range funcs {
		bcVA := payloadVA + uint32(records[i].payloadOff)
		fmt.Printf("    [%s] bytecode VA: 0x%X, len: %d\n", fb.FI.Name, bcVA, records[i].bcLen)
	}

	// 3. Hijack PT_NOTE → PT_LOAD
	noteIdx, err := findPTNOTE32(p.data, ehdr)
	if err != nil {
		return err
	}
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

	notePhdrOff = reorderPTLoads32(p.data, ehdr, payloadVA, notePhdrOff)

	// 4. Token descriptor table (8 bytes per entry for ARM32)
	for len(payload)%4 != 0 {
		payload = append(payload, 0x00)
	}
	tokenTableOff := len(payload)
	tokenTableVA32 := payloadVA + uint32(tokenTableOff)
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

	patchTokenHeader32(p.data, payloadFileOff, uint32(tokenTableVAOff), selfVA32, tokenTableVA32)

	fmt.Printf("    [TOKEN] descriptor table VA: 0x%X, entries: %d\n", tokenTableVA32, len(funcs))
	fmt.Printf("    [TOKEN] _token_table_va patched at blob offset 0x%X → relative offset 0x%X (PIE)\n", tokenTableVAOff, tokenTableVA32-selfVA32)
	fmt.Printf("    [TOKEN] _link_time_self_va patched → 0x%X\n", selfVA32)

	vmEntryTokenVA := payloadVA + uint32(tokenEntryOff)
	fmt.Printf("    [TOKEN] vm_entry_token VA: 0x%X\n", vmEntryTokenVA)

	return writeTrampolines(p.data, funcs, uint64(vmEntryTokenVA), 0, func(i int, fb FuncBytecode) []byte {
		token := (uint32(fb.XorKey) << 24) | (uint32(i) & 0xFFF)
		if p.thumbFuncs[fb.FI.Addr] {
			return BuildTokenTrampolineThumb(uint32(fb.FI.Addr), vmEntryTokenVA, token)
		}
		return BuildTokenTrampolineARM32(uint32(fb.FI.Addr), vmEntryTokenVA, token)
	})
}

// ---- ELF64 helpers ----

func findMaxVA64(data []byte, ehdr elf64Ehdr) uint64 {
	var maxVA uint64
	for i := 0; i < int(ehdr.Phnum); i++ {
		phOff := ehdr.Phoff + uint64(i)*uint64(ehdr.Phentsize)
		ph := readPhdr64(data, phOff)
		if ph.Type == uint32(elf.PT_LOAD) {
			end := ph.Vaddr + ph.Memsz
			if end > maxVA {
				maxVA = end
			}
		}
	}
	return maxVA
}

func findPTNOTE64(data []byte, ehdr elf64Ehdr) (int, error) {
	for i := 0; i < int(ehdr.Phnum); i++ {
		phOff := ehdr.Phoff + uint64(i)*uint64(ehdr.Phentsize)
		ph := readPhdr64(data, phOff)
		if ph.Type == uint32(elf.PT_NOTE) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("PT_NOTE segment not found")
}

func reorderPTLoads64(data []byte, ehdr elf64Ehdr, payloadVA uint64, notePhdrOff uint64) uint64 {
	type phdrSlot struct {
		idx  int
		phdr elf64Phdr
	}
	var loads []phdrSlot
	for i := 0; i < int(ehdr.Phnum); i++ {
		off := ehdr.Phoff + uint64(i)*uint64(ehdr.Phentsize)
		ph := readPhdr64(data, off)
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
	if !needSort {
		return notePhdrOff
	}

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
		writePhdr64(data, off, loads[k].phdr)
	}

	fmt.Printf("    [PHDR] Reordered %d PT_LOAD segments by Vaddr ascending\n", len(loads))

	// Find updated notePhdrOff after reordering
	for i := 0; i < int(ehdr.Phnum); i++ {
		off := ehdr.Phoff + uint64(i)*uint64(ehdr.Phentsize)
		ph := readPhdr64(data, off)
		if ph.Type == uint32(elf.PT_LOAD) && ph.Vaddr == payloadVA {
			return off
		}
	}
	return notePhdrOff
}

func buildRTLrTable(payload *[]byte, funcs []FuncBytecode) int {
	rtlrOff := len(*payload)
	*payload = append(*payload, "RTLR"...)

	totalRelocs := 0
	for _, fb := range funcs {
		totalRelocs += len(fb.Relocations)
	}
	tmp32 := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmp32, uint32(totalRelocs))
	*payload = append(*payload, tmp32...)

	if totalRelocs > 0 {
		fmt.Printf("    [RELOC] Appending %d relocations to RTLR table\n", totalRelocs)
		tmp64 := make([]byte, 8)
		for i, fb := range funcs {
			for _, rel := range fb.Relocations {
				binary.LittleEndian.PutUint64(tmp64, uint64(i))
				*payload = append(*payload, tmp64...)
				binary.LittleEndian.PutUint64(tmp64, uint64(rel.BcOffset))
				*payload = append(*payload, tmp64...)
				binary.LittleEndian.PutUint64(tmp64, rel.TargetAddr)
				*payload = append(*payload, tmp64...)
			}
		}
	}
	return rtlrOff
}

func patchTokenHeader64(data []byte, payloadFileOff uint64, tokenTableVAOff uint64, selfVA uint64, tokenTableVA uint64, rtlrOff int) {
	tblRelOff := tokenTableVA - selfVA
	binary.LittleEndian.PutUint64(data[payloadFileOff+tokenTableVAOff:], tblRelOff)
	binary.LittleEndian.PutUint64(data[payloadFileOff+tokenTableVAOff+8:], selfVA)
	binary.LittleEndian.PutUint64(data[payloadFileOff+tokenTableVAOff+16:], uint64(rtlrOff))
}

// ---- ELF32 helpers ----

func findMaxVA32(data []byte, ehdr elf32Ehdr) uint64 {
	var maxVA uint32
	for i := 0; i < int(ehdr.Phnum); i++ {
		phOff := ehdr.Phoff + uint32(i)*uint32(ehdr.Phentsize)
		ph := readPhdr32(data, phOff)
		if ph.Type == uint32(elf.PT_LOAD) {
			end := ph.Vaddr + ph.Memsz
			if end > maxVA {
				maxVA = end
			}
		}
	}
	return uint64(maxVA)
}

func findPTNOTE32(data []byte, ehdr elf32Ehdr) (int, error) {
	for i := 0; i < int(ehdr.Phnum); i++ {
		phOff := ehdr.Phoff + uint32(i)*uint32(ehdr.Phentsize)
		ph := readPhdr32(data, phOff)
		if ph.Type == uint32(elf.PT_NOTE) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("PT_NOTE segment not found")
}

func reorderPTLoads32(data []byte, ehdr elf32Ehdr, payloadVA uint32, notePhdrOff uint32) uint32 {
	type phdrSlot struct {
		idx  int
		phdr elf32Phdr
	}
	var loads []phdrSlot
	for i := 0; i < int(ehdr.Phnum); i++ {
		off := ehdr.Phoff + uint32(i)*uint32(ehdr.Phentsize)
		ph := readPhdr32(data, off)
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
	if !needSort {
		return notePhdrOff
	}

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
		writePhdr32(data, off, loads[k].phdr)
	}

	fmt.Printf("    [PHDR] Reordered %d PT_LOAD segments by Vaddr ascending\n", len(loads))

	for i := 0; i < int(ehdr.Phnum); i++ {
		off := ehdr.Phoff + uint32(i)*uint32(ehdr.Phentsize)
		ph := readPhdr32(data, off)
		if ph.Type == uint32(elf.PT_LOAD) && ph.Vaddr == payloadVA {
			return off
		}
	}
	return notePhdrOff
}

func patchTokenHeader32(data []byte, payloadFileOff uint32, tokenTableVAOff uint32, selfVA32 uint32, tokenTableVA32 uint32) {
	tblRelOff := tokenTableVA32 - selfVA32
	binary.LittleEndian.PutUint32(data[payloadFileOff+tokenTableVAOff:], tblRelOff)
	binary.LittleEndian.PutUint32(data[payloadFileOff+tokenTableVAOff+4:], selfVA32)
}
