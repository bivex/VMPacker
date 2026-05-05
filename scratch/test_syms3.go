package main

import (
	"debug/elf"
	"fmt"
)

func main() {
	f, err := elf.Open("test/android/build/libnative_test_arm64.so")
	if err != nil { panic(err) }
	defer f.Close()

	dsyms, _ := f.DynamicSymbols()
	if len(dsyms) > 4 {
		fmt.Printf("DynSym 3: %s\n", dsyms[3].Name)
		fmt.Printf("DynSym 4: %s\n", dsyms[4].Name)
	}
}
