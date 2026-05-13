package main

import (
	"fmt"
	"golang.org/x/arch/x86/x86asm"
)

func main() {
	inst, _ := x86asm.Decode([]byte{0x89, 0x7d, 0xfc}, 64) // mov %edi, -0x4(%rbp)
	fmt.Printf("DataSize=%d MemBytes=%d\n", inst.DataSize, inst.MemBytes)
}
