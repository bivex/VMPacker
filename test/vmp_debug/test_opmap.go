//go:build ignore

package main

import (
	"fmt"
	vm "vmpacker/pkg/vm"
)

func main() {
	vm.GenerateDynamicISA()
	fmt.Printf("OpPush = 0x%02X\n", vm.OpPush)
	fmt.Printf("GlobalOpMap[48] = 0x%02X\n", vm.GlobalOpMap[48])
	fmt.Printf("Match: %v\n", vm.OpPush == vm.GlobalOpMap[48])
}
