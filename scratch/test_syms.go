//go:build ignore

package main

import (
	"debug/elf"
	"fmt"
)

func main() {
	f, err := elf.Open("test/android/build/libnative_test_arm64.so")
	if err != nil { panic(err) }
	defer f.Close()

	syms, _ := f.ImportedSymbols()
	for i, s := range syms {
		if s.Name == "snprintf" || s.Name == "__snprintf_chk" {
			fmt.Printf("Imported %d: %s\n", i, s.Name)
		}
	}
	
	dsyms, _ := f.DynamicSymbols()
	for i, s := range dsyms {
		if s.Name == "snprintf" || s.Name == "__snprintf_chk" {
			fmt.Printf("DynSym %d: %s\n", i, s.Name)
		}
	}
}