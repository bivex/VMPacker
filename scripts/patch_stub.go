//go:build ignore

// patch_stub.go — Binary-patches debug svc #0 write() sequences out of vm_interp.bin
// These debug prints were added temporarily during debugging and must be removed.
//
// ARM64 encoding of the debug sequence:
//   MOV X8, #64       -> D2800808
//   MOV X0, #1        -> D2800020
//   (ADR or MOV X1 ...)
//   MOV X2, #N        -> D2800(N<<5)02  (varies)
//   SVC #0            -> D4000001
//
// We search for the pattern: D2800808 (mov x8, #64 = __NR_write)
// followed within 5 instructions by D4000001 (svc #0)
// and replace the entire 5-instruction block with NOPs (D503201F)
//
// Usage: go run scripts/patch_stub.go
package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	binPath  = "cmd/vmpacker/vm_interp.bin"
	// ARM64 NOP
	nopInsn  = uint32(0xD503201F)
	// MOV X8, #64 (__NR_write)
	movX8_64 = uint32(0xD2800808)
	// SVC #0
	svc0     = uint32(0xD4000001)
	// Header size: 3 x uint64 (vm_entry, vm_entry_token, _token_table_va offsets)
	headerSize = 24
)

func main() {
	data, err := os.ReadFile(binPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", binPath, err)
		os.Exit(1)
	}

	// Work on the raw blob portion (skip 24-byte header)
	blob := data[headerSize:]
	patched := 0

	// Scan 4-byte aligned words
	for i := 0; i+4 <= len(blob); i += 4 {
		insn := binary.LittleEndian.Uint32(blob[i:])
		if insn != movX8_64 {
			continue
		}
		// Found MOV X8, #64 — look for SVC #0 within the next 8 instructions
		found := -1
		for j := i + 4; j < i+36 && j+4 <= len(blob); j += 4 {
			if binary.LittleEndian.Uint32(blob[j:]) == svc0 {
				found = j
				break
			}
		}
		if found < 0 {
			continue
		}
		// NOP out from i to found (inclusive), i.e. the whole block
		blockEnd := found + 4
		fmt.Printf("[patch] Found debug write() block at blob offset 0x%X..0x%X, NOPping %d instructions\n",
			i, blockEnd, (blockEnd-i)/4)
		for k := i; k < blockEnd; k += 4 {
			binary.LittleEndian.PutUint32(blob[k:], nopInsn)
		}
		patched++
		i = blockEnd - 4 // advance past the block
	}

	if patched == 0 {
		fmt.Println("[info] No debug write() sequences found — binary may already be clean.")
		return
	}

	fmt.Printf("[+] Patched %d debug sequence(s)\n", patched)
	if err := os.WriteFile(binPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", binPath, err)
		os.Exit(1)
	}
	fmt.Printf("[+] Written: %s (%d bytes)\n", binPath, len(data))
}
