package main

import (
	"bytes"
	"fmt"
	"os"
)

func main() {
	data, err := os.ReadFile("/tmp/test_vmp2.vmp")
	if err != nil {
		panic(err)
	}
	
	// Find VMP1 magic
	vmp1Idx := bytes.Index(data, []byte("VMP1"))
	fmt.Printf("VMP1 at %x\n", vmp1Idx)
	
	// payloadStart is vmp1Idx - 24 (since header is 24 bytes)
	// But the interpreter is the whole thing?
	// Let's just search for the CRC_MAGIC
	idx := bytes.Index(data, []byte{0x32, 0x43, 0x52, 0x43})
	bcStart := idx - 20
	
	// The bytecode ends at bcStart. Let's find the start of the bytecode.
	// Since we know the bytecode is not that long, let's just print the bytes before the trailer.
	// Actually, the trailer starts with stub_va(8).
	// Let's just find the start of the bytecode by looking at the padding.
	
	// The payload format is [interpCode][padding][bc0]
	// The first bytecode is likely right after the padding.
	// Let's look at offset vmp1Idx + 20000.
	
	opMapStart := idx + 4 + 64
	opMap := data[opMapStart : opMapStart+256]
	
	fmt.Printf("op_map[48] (Push) = %02x\n", opMap[48])
	fmt.Printf("op_map[0] (Nop) = %02x\n", opMap[0])
	
	// In the python script, I found `bc_len = 81` for check1.
	fmt.Printf("bc[0] = %02x\n", data[bcStart-81])
	fmt.Printf("bc = %x\n", data[bcStart-81:bcStart])
}
