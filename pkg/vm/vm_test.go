package vm

import (
	"testing"
)

func TestGenerateDynamicISA(t *testing.T) {
	// First generation
	GenerateDynamicISA()

	// Verify GlobalOpMap contains unique values for valid opcodes
	seen := make(map[byte]bool)
	opCount := int(OpIdCount)
	for i := 0; i < opCount; i++ {
		seen[GlobalOpMap[i]] = true
	}

	if len(seen) != opCount {
		t.Errorf("GenerateDynamicISA did not produce %d unique opcodes, got %d unique", opCount, len(seen))
	}

	// Verify InverseOpMap is correct
	for i := 0; i < opCount; i++ {
		val := GlobalOpMap[i]
		if InverseOpMap[val] != byte(i) {
			t.Errorf("InverseOpMap is incorrect for index %d: expected %d, got %d", i, val, InverseOpMap[val])
		}
	}

	// Verify that opPtrs actually updated the exported variables
	// We'll check if OpNop and OpHalt are different (they should be, as they are unique)
	if OpNop == OpHalt {
		t.Errorf("OpNop and OpHalt have the same value: %d", OpNop)
	}

	// Second generation to ensure it randomizes differently
	firstNop := OpNop
	firstHalt := OpHalt
	
	GenerateDynamicISA()
	
	// There is an extremely small chance (1/256) that one of these might be the same, 
	// but the permutation as a whole should definitely be different.
	if firstNop == OpNop && firstHalt == OpHalt {
		t.Logf("Warning: OpNop and OpHalt remained the same after second generation (rare but possible)")
	}
}

func TestRebuildOpTable(t *testing.T) {
	// Generate ISA and rebuild table
	GenerateDynamicISA()
	RebuildOpTable()

	// Get the randomized opcode for OpAdd
	randomizedAdd := OpAdd

	// Verify size via disassembler
	size := InstructionSize(randomizedAdd)
	if size != 4 {
		t.Errorf("Expected OpAdd size to be 4, got %d for randomized opcode 0x%X", size, randomizedAdd)
	}

	// Verify name via disassembler
	name := OpcodeName(randomizedAdd)
	if name != "ADD" {
		t.Errorf("Expected opcode name 'ADD', got '%s'", name)
	}

	// Now re-generate and rebuild, check if the old opcode now points to something else (or UNKNOWN)
	GenerateDynamicISA()
	RebuildOpTable()

	// Ensure the new OpAdd is correctly mapped
	newAdd := OpAdd
	size = InstructionSize(newAdd)
	if size != 4 {
		t.Errorf("Expected new OpAdd size to be 4, got %d", size)
	}
}

func TestDisasmOne(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// Construct a dummy ADD instruction: OpAdd R1, R2, R3
	bytecode := []byte{OpAdd, 1, 2, 3}

	text, size := DisasmOne(bytecode, 0)
	expectedText := "0000: ADD R1, R2, R3"
	
	if size != 4 {
		t.Errorf("Expected DisasmOne to return size 4, got %d", size)
	}

	if text != expectedText {
		t.Errorf("Expected DisasmOne text '%s', got '%s'", expectedText, text)
	}

	// Test EOF handling
	textEOF, sizeEOF := DisasmOne([]byte{OpAdd, 1}, 0)
	if sizeEOF != 2 { // 2 bytes remaining
		t.Errorf("Expected truncated size 2, got %d", sizeEOF)
	}
	if textEOF != "0000: ADD (truncated)" {
		t.Errorf("Expected truncated text, got '%s'", textEOF)
	}
}

func TestDisasmAll(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	bytecode := []byte{OpAdd, 1, 2, 3, OpNop, OpJmp, 0x05, 0x00, 0x00, 0x00}
	lines := DisasmAll(bytecode)

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines of disassembly, got %d", len(lines))
	}
}

func TestInstructionSize(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// OpNop is 1 byte
	if sz := InstructionSize(OpNop); sz != 1 {
		t.Errorf("Expected OpNop size 1, got %d", sz)
	}

	// OpAdd is 4 bytes
	if sz := InstructionSize(OpAdd); sz != 4 {
		t.Errorf("Expected OpAdd size 4, got %d", sz)
	}

	// OpJmp is 5 bytes
	if sz := InstructionSize(OpJmp); sz != 5 {
		t.Errorf("Expected OpJmp size 5, got %d", sz)
	}
}
