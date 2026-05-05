package arm64

import (
	"encoding/binary"
	"math/rand"
	"slices"
	"testing"

	"github.com/vmpacker/pkg/vm"
)

func TestInsertJunkCode(t *testing.T) {
	// Initialize dummy opcodes if needed, but vm.OpJmp is just a byte
	vm.GenerateDynamicISA()

	rand.Seed(42) // for determinism in testing
	tr := NewTranslator(0x1000, 0x100)

	// Run until junk code triggers
	initialPos := tr.pos()
	for i := 0; i < 100; i++ {
		tr.insertJunkCode()
		if tr.pos() > initialPos {
			break
		}
	}

	if tr.pos() == initialPos {
		t.Fatalf("insertJunkCode never triggered")
	}

	code := tr.code
	// Byte 0: vm.OpJmp
	if code[0] != vm.OpJmp {
		t.Errorf("Expected OpJmp (0x%X), got 0x%X", vm.OpJmp, code[0])
	}

	// Byte 1-4: Target Pos (Little Endian uint32)
	target := binary.LittleEndian.Uint32(code[1:5])
	if int(target) != len(code) {
		t.Errorf("Expected jump target %d, got %d", len(code), target)
	}

	junkLen := int(target) - 5
	if junkLen < 1 || junkLen > 12 {
		t.Errorf("Expected junk length between 1 and 12, got %d", junkLen)
	}
}

func TestEmitStackMBA(t *testing.T) {
	vm.GenerateDynamicISA()
	rand.Seed(42)

	tests := []struct {
		name     string
		sOp      byte
		contains []byte // opcodes that should be present in the MBA sequence
	}{
		{"SAdd_MBA", vm.OpSAdd, []byte{vm.OpSXor, vm.OpSAnd}},
		{"SSub_MBA", vm.OpSSub, []byte{vm.OpSXor, vm.OpSAnd}},
		{"SXor_MBA", vm.OpSXor, []byte{vm.OpSOr, vm.OpSAnd, vm.OpSSub}},
		{"SAnd_MBA", vm.OpSAnd, []byte{vm.OpSOr, vm.OpSXor, vm.OpSSub}},
		{"SOr_MBA", vm.OpSOr, []byte{vm.OpSAnd, vm.OpSXor, vm.OpSAdd}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := NewTranslator(0x1000, 0x100)
			tr.SetMBA(true)
			pushX := func() { tr.emit(0xAA) } // dummy instruction for X
			pushY := func() { tr.emit(0xBB) } // dummy instruction for Y

			triggered := false
			for i := 0; i < 100; i++ {
				tr.code = nil // reset
				if tr.emitStackMBA(tc.sOp, pushX, pushY) {
					// Check if all required opcodes are present somewhere in the code
					// (Recursive MBA might have replaced some, so we try multiple times)
					allFound := true
					for _, op := range tc.contains {
						found := slices.Contains(tr.code, op)
						if !found {
							allFound = false
							break
						}
					}
					if allFound {
						triggered = true
						break
					}
				}
			}

			if !triggered {
				t.Fatalf("emitStackMBA never produced expected sequence for %s (tried 100 times)", tc.name)
			}
		})
	}
}

func TestEmitStringDecryption(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x100)

	refs := []StringRef{
		{Addr: 0x401000, Len: 10, Key: 0xA5},
	}

	tr.EmitStringDecryption(refs)

	// Verify the emitted bytecode
	// Expected:
	// SPushImm64 (1 byte) + 8 bytes addr = 9 bytes
	// SPushImm32 (1 byte) + 4 bytes len = 5 bytes
	// SPushImm32 (1 byte) + 4 bytes key = 5 bytes
	// SDecryptStr (1 byte)
	// Total = 20 bytes

	code := tr.code
	if len(code) != 20 {
		t.Fatalf("Expected 20 bytes, got %d", len(code))
	}

	if code[0] != vm.OpSPushImm64 {
		t.Errorf("Expected OpSPushImm64, got 0x%X", code[0])
	}
	addr := binary.LittleEndian.Uint64(code[1:9])
	if addr != 0x401000 {
		t.Errorf("Expected addr 0x401000, got 0x%X", addr)
	}

	if code[9] != vm.OpSPushImm32 {
		t.Errorf("Expected OpSPushImm32, got 0x%X", code[9])
	}
	length := binary.LittleEndian.Uint32(code[10:14])
	if length != 10 {
		t.Errorf("Expected len 10, got %d", length)
	}

	if code[14] != vm.OpSPushImm32 {
		t.Errorf("Expected OpSPushImm32, got 0x%X", code[14])
	}
	key := binary.LittleEndian.Uint32(code[15:19])
	if key != 0xA5 {
		t.Errorf("Expected key 0xA5, got 0x%X", key)
	}

	if code[19] != vm.OpSDecryptStr {
		t.Errorf("Expected OpSDecryptStr, got 0x%X", code[19])
	}
}
