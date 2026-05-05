package main
import "fmt"
import "github.com/vmpacker/pkg/vm"
func main() {
    vm.GenerateDynamicISA()
    vm.RebuildOpTable()
    for i := 0; i < 256; i++ {
        sz := vm.InstructionSize(byte(i))
        if sz > 10 {
            fmt.Printf("WARNING: op %d has size %d\n", i, sz)
        }
    }
    fmt.Println("Done")
}
