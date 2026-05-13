package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

func main() {
	data, err := os.ReadFile("/tmp/test_vmp2.vmp")
	if err != nil {
		panic(err)
	}
	idx := bytes.Index(data, []byte{0x32, 0x43, 0x52, 0x43})
	bcStart := idx - 20
	ocKeyOffset := bcStart + 24 + 64 + 256 + 101*8 + 1
	// Wait, the trailer length depends on map count. Let's just find ocKey right before mapCount
	// The trailer ends with: ocKey(4), mapCount(4), funcAddr(8), funcSize(4)
	// So ocKey is at idx_of_next_trailer - 20
	
	// Just let's use the Python output which said "bc[0] = 88".
}
