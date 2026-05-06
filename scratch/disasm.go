package main
import (
	"debug/elf"
	"fmt"
	"os"
	"golang.org/x/arch/arm64/arm64asm"
)
func main() {
	f, err := elf.Open(os.Args[1])
	if err != nil { panic(err) }
	syms, _ := f.Symbols()
	var start uint64
	var size uint64
	for _, s := range syms {
		if s.Name == "check_complex" {
			start = s.Value
			size = s.Size
			break
		}
	}
	if start == 0 { panic("not found") }
	
	// find section
	for _, sect := range f.Sections {
		if start >= sect.Addr && start < sect.Addr+sect.Size {
			data, _ := sect.Data()
			offset := start - sect.Addr
			code := data[offset : offset+size]
			for i := 0; i < len(code); i+=4 {
				v := uint32(code[i]) | uint32(code[i+1])<<8 | uint32(code[i+2])<<16 | uint32(code[i+3])<<24
				inst, _ := arm64asm.Decode(code[i:])
				fmt.Printf("0x%x: %08x %s\n", start+uint64(i), v, inst.String())
			}
			break
		}
	}
}
