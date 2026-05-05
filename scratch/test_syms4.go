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
	for i := 0; i < 6; i++ {
		fmt.Printf("Index %d: %q\n", i, dsyms[i].Name)
	}
}