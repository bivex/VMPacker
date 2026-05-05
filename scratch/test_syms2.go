package main

import (
	"debug/elf"
	"fmt"
)

func main() {
	f, err := elf.Open("test/android/build/libnative_test_arm64.so")
	if err != nil { panic(err) }
	defer f.Close()

	pltRelocs := f.Section(".rela.plt")
	if pltRelocs != nil {
		data, _ := pltRelocs.Data()
		for i := 0; i < len(data); i += 24 {
			r_offset := f.ByteOrder.Uint64(data[i:i+8])
			r_info := f.ByteOrder.Uint64(data[i+8:i+16])
			symIdx := r_info >> 32
			
			if r_offset == 0x9fa8 {
				fmt.Printf("0x9fa8 -> symIdx %d\n", symIdx)
			}
			if r_offset == 0x9fc8 {
				fmt.Printf("0x9fc8 -> symIdx %d\n", symIdx)
			}
		}
	}
}