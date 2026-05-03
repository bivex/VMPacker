package elf

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sort"

	"debug/elf"
)

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
