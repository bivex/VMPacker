//go:build ignore

package main
import (
	"fmt"
	"github.com/vmpacker/pkg/vm"
)
func main() {
    // We need to know the mapping. But OpIds are constants.
    fmt.Printf("OpIdNop: %d\n", vm.OpIdNop)
    fmt.Printf("OpIdHalt: %d\n", vm.OpIdHalt)
    fmt.Printf("OpIdRet: %d\n", vm.OpIdRet)
    fmt.Printf("OpIdAdd: %d\n", vm.OpIdAdd)
    fmt.Printf("OpIdSAdd: %d\n", vm.OpIdSAdd)
    fmt.Printf("OpIdJe: %d\n", vm.OpIdJe)
    fmt.Printf("OpIdJne: %d\n", vm.OpIdJne)
    fmt.Printf("OpIdSvc: %d\n", vm.OpIdSvc)
}
