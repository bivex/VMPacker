package main

import (
	"fmt"
	"github.com/vmpacker/pkg/vm"
)

func main() {
	vm.GenerateDynamicISA()
	fmt.Printf("OpPush: %x\n", vm.OpPush)
	fmt.Printf("GlobalOpMap[48]: %x\n", vm.GlobalOpMap[48])
}