package main
import (
	"fmt"
	"golang.org/x/arch/arm64/arm64asm"
)
func main() {
	bytes := []byte{0x42, 0x08, 0x02, 0x8b}
	inst, _ := arm64asm.Decode(bytes)
	fmt.Printf("%v\n", inst)
}
