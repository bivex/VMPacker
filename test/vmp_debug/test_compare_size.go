package main

import (
	"fmt"
	"github.com/vmpacker/pkg/vm"
)

func main() {
	vm.GenerateDynamicISA()
	for i := 0; i < int(vm.OpIdCount); i++ {
		// we need to map logical -> physical, then look up in opTable
		phys := vm.GlobalOpMap[i]
		info := vm.OpTable()[phys]
		fmt.Printf("id=%d size=%d name=%s\n", i, info.Size, info.Name)
	}
}
