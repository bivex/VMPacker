package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/vmpacker/pkg/vm"
)

func main() {
	// Re-run encryption logic manually to see if it hits unknown opcodes
	data, err := os.ReadFile("/tmp/test_vmp2.vmp")
	if err != nil {
		panic(err)
	}

	idx := bytes.Index(data, []byte{0x32, 0x43, 0x52, 0x43})
	bcStart := idx - 20
	opMapStart := idx + 4 + 64
	opMap := data[opMapStart : opMapStart+256]

	// Reconstruct InverseOpMap
	inverseOpMap := make(map[byte]byte)
	for logical := 0; logical < 256; logical++ {
		phys := opMap[logical]
		// skip the padding zeroes at the end
		if logical > 0 && phys == opMap[0] {
			continue // mapped to NOP
		}
		inverseOpMap[phys] = byte(logical)
	}

	// Wait, we don't have the original OpTable...
	// We can use vm.OpLogicalTable() but we don't have that function.
	// Let's just print the bytecode and manually decode the first few instructions.
	bcLen := 81
	bc := data[bcStart-bcLen : bcStart]
	fmt.Printf("bc = %x\n", bc)
}
