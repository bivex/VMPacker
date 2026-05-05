package main
import "fmt"
import "github.com/vmpacker/pkg/vm"
func main() {
    vm.RebuildOpTable()
    for id := 0; id < vm.OpIdCount; id++ {
        // Find the ptr for this ID. Wait, opPtrs is not exported.
        // We can just check the default size of all opcodes.
    }
}
