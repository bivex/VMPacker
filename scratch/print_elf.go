//go:build ignore

package main
import (
	"debug/elf"
	"fmt"
	"os"
)
func main() {
	f, err := elf.Open(os.Args[1])
	if err != nil { panic(err) }
	for _, p := range f.Progs {
		if p.Type == elf.PT_LOAD {
			fmt.Printf("LOAD Off: 0x%x Vaddr: 0x%x Filesz: 0x%x Memsz: 0x%x Flags: %s\n", p.Off, p.Vaddr, p.Filesz, p.Memsz, p.Flags)
		}
	}
}
