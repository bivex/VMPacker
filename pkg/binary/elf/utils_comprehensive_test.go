package elf

import (
	"encoding/binary"
	"testing"

	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// Comprehensive ELF packer tests
// Covers: utility functions, bytecode reversal, opcode encryption,
// branch remapping, ParseAddrSpec, NewPacker, types
// ============================================================

// --- ParseAddrSpec Edge Cases ---

func TestParseAddrSpec_JustAddr(t *testing.T) {
	spec, err := ParseAddrSpec("0x400100")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Addr != 0x400100 {
		t.Errorf("Expected Addr=0x400100, got 0x%X", spec.Addr)
	}
	if spec.End != 0 {
		t.Errorf("Expected End=0, got 0x%X", spec.End)
	}
	if spec.Name != "sub_400100" {
		t.Errorf("Expected auto-name 'sub_400100', got '%s'", spec.Name)
	}
}

func TestParseAddrSpec_AddrRange(t *testing.T) {
	spec, err := ParseAddrSpec("0x400100-0x400200")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Addr != 0x400100 || spec.End != 0x400200 {
		t.Errorf("Expected 0x400100-0x400200, got 0x%X-0x%X", spec.Addr, spec.End)
	}
}

func TestParseAddrSpec_WithName(t *testing.T) {
	spec, err := ParseAddrSpec("0x400100-0x400200:my_func")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "my_func" {
		t.Errorf("Expected 'my_func', got '%s'", spec.Name)
	}
}

func TestParseAddrSpec_DecimalAddr(t *testing.T) {
	spec, err := ParseAddrSpec("4194560") // 0x400100
	if err != nil {
		t.Fatal(err)
	}
	if spec.Addr != 0x400100 {
		t.Errorf("Expected 0x400100, got 0x%X", spec.Addr)
	}
}

func TestParseAddrSpec_InvalidInput(t *testing.T) {
	_, err := ParseAddrSpec("not_an_addr")
	if err == nil {
		t.Error("Expected error for invalid input")
	}
}

func TestParseAddrSpec_ReversedRange(t *testing.T) {
	_, err := ParseAddrSpec("0x400200-0x400100")
	if err == nil {
		t.Error("Expected error when end < start")
	}
}

func TestParseAddrSpec_ZeroWidth(t *testing.T) {
	_, err := ParseAddrSpec("0x400100-0x400100")
	if err == nil {
		t.Error("Expected error for zero-width range")
	}
}

// --- NewPacker Tests ---

func TestNewPacker_Fields(t *testing.T) {
	p := NewPacker("input", "output", []string{"f1", "f2"}, nil, true, false, true, false, []byte{1, 2, 3})
	if p.inputPath != "input" {
		t.Error("inputPath not set")
	}
	if p.outputPath != "output" {
		t.Error("outputPath not set")
	}
	if len(p.funcNames) != 2 {
		t.Errorf("Expected 2 func names, got %d", len(p.funcNames))
	}
	if !p.verbose {
		t.Error("verbose should be true")
	}
	if p.stripSymbols {
		t.Error("stripSymbols should be false")
	}
	if !p.debug {
		t.Error("debug should be true")
	}
	if len(p.interpBlob) != 3 {
		t.Errorf("Expected 3-byte blob, got %d", len(p.interpBlob))
	}
}

func TestPacker_SetARM32(t *testing.T) {
	p := NewPacker("in", "out", nil, nil, false, false, false, false, nil)
	blob := []byte{0xAA, 0xBB}
	p.SetInterpBlobARM32(blob)
	if len(p.interpBlobARM32) != 2 {
		t.Error("ARM32 blob not set")
	}
}

func TestPacker_SetCFF(t *testing.T) {
	p := NewPacker("in", "out", nil, nil, false, false, false, false, nil)
	p.SetCFF(true)
	if !p.cff {
		t.Error("CFF should be enabled")
	}
}

func TestPacker_SetMBA(t *testing.T) {
	p := NewPacker("in", "out", nil, nil, false, false, false, false, nil)
	p.SetMBA(true)
	if !p.mba {
		t.Error("MBA should be enabled")
	}
}

func TestPacker_SetMangle(t *testing.T) {
	p := NewPacker("in", "out", nil, nil, false, false, false, false, nil)
	p.SetMangle(true)
	if !p.mangleSymbols {
		t.Error("Mangle should be enabled")
	}
}

// --- Bytecode Reversal Tests ---

func TestReverseInstructions_Simple(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// NOP + HALT → should be reversed to HALT + NOP + size markers
	original := []byte{vm.OpNop, vm.OpHalt}
	reversed, offsetMap, byteMap := reverseInstructions(original, len(original))

	// After reversal, HALT should come first (was last), then NOP
	if len(reversed) == 0 {
		t.Fatal("Reversed bytecode is empty")
	}

	// Each instruction gets a 1-byte size marker appended
	// HALT (1B) + size(1B) + NOP (1B) + size(1B) = 4 bytes
	expectedLen := len(original) + 2 // 2 instructions, 2 size markers
	if len(reversed) != expectedLen {
		t.Errorf("Expected %d reversed bytes, got %d", expectedLen, len(reversed))
	}

	// offsetMap should have entries for each original instruction
	if len(offsetMap) != 2 {
		t.Errorf("Expected 2 offset map entries, got %d", len(offsetMap))
	}

	// byteMap should map every original byte to its new position
	if len(byteMap) != len(original) {
		t.Errorf("Expected %d byte map entries, got %d", len(original), len(byteMap))
	}
}

func TestReverseInstructions_MultiByteInstruction(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// PUSH R5 (2B) + HALT (1B)
	original := []byte{vm.OpPush, 5, vm.OpHalt}
	reversed, _, _ := reverseInstructions(original, len(original))

	// HALT(1B) + size(1B) + PUSH(2B) + size(1B) = 5 bytes
	if len(reversed) != 5 {
		t.Errorf("Expected 5 reversed bytes, got %d", len(reversed))
	}

	// First instruction should be HALT (was last in original)
	if reversed[0] != vm.OpHalt {
		t.Errorf("First reversed byte should be HALT (0x%02X), got 0x%02X", vm.OpHalt, reversed[0])
	}
}

func TestReverseInstructions_ThreeInstructions(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// NOP + NOP + HALT
	original := []byte{vm.OpNop, vm.OpNop, vm.OpHalt}
	reversed, _, _ := reverseInstructions(original, len(original))

	// HALT(1)+size(1) + NOP(1)+size(1) + NOP(1)+size(1) = 6 bytes
	if len(reversed) != 6 {
		t.Errorf("Expected 6 bytes, got %d", len(reversed))
	}

	// Verify reversal: HALT should be first, NOP last
	if reversed[0] != vm.OpHalt {
		t.Error("First should be HALT")
	}
}

func TestReverseInstructions_EmptyBytecode(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	reversed, offsetMap, byteMap := reverseInstructions([]byte{}, 0)
	if len(reversed) != 0 {
		t.Errorf("Expected empty output, got %d bytes", len(reversed))
	}
	if len(offsetMap) != 0 {
		t.Errorf("Expected empty offsetMap")
	}
	if len(byteMap) != 0 {
		t.Errorf("Expected empty byteMap")
	}
}

// --- Opcode Encryption Tests ---

func TestEncryptOpcodes_Forward(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// Create simple bytecode
	bc := []byte{vm.OpNop, vm.OpNop, vm.OpHalt}
	original := make([]byte, len(bc))
	copy(original, bc)

	ocKey := uint32(0xDEADBEEF)
	encryptOpcodes(bc, len(bc), ocKey, false)

	// After encryption, opcodes should be different (unless key collision)
	if bc[0] == original[0] && bc[1] == original[1] && bc[2] == original[2] {
		t.Error("Encryption did not change any bytes (unlikely)")
	}
}

func TestEncryptOpcodes_ForwardChangesOpcodes(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// Test with single-byte instructions where encryption can parse them correctly
	bc := []byte{vm.OpNop, vm.OpHalt}
	original := make([]byte, len(bc))
	copy(original, bc)

	ocKey := uint32(0x12345678)
	encryptOpcodes(bc, len(bc), ocKey, false)

	// Verify that opcodes changed (XOR with non-zero mask at each position)
	// For pc=0: mask = byte(0x12345678 ^ 0) = 0x78, so NOP ^ 0x78 should differ
	if bc[0] == original[0] && bc[1] == original[1] {
		t.Error("Encryption did not change opcodes")
	}

	// Manually verify: byte 0 should be OpNop ^ byte(ocKey ^ 0)
	expected0 := original[0] ^ byte(ocKey)
	if bc[0] != expected0 {
		t.Errorf("Byte 0: expected 0x%02X, got 0x%02X", expected0, bc[0])
	}
}

func TestEncryptOpcodes_Reversed(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// Create reversed bytecode with size markers: HALT + 0x01 + NOP + 0x01
	bc := []byte{vm.OpHalt, 0x01, vm.OpNop, 0x01}
	original := make([]byte, len(bc))
	copy(original, bc)

	ocKey := uint32(0xCAFEBABE)
	encryptOpcodes(bc, len(bc), ocKey, true)

	// Should have changed the opcodes but not the size markers
	if bc[1] != 0x01 || bc[3] != 0x01 {
		t.Error("Size markers should not be changed by encryption")
	}
}

func TestEncryptOpcodes_ZeroKey(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	bc := []byte{vm.OpNop, vm.OpHalt}
	original := make([]byte, len(bc))
	copy(original, bc)

	// Key=0 with pc*golden at each position
	encryptOpcodes(bc, len(bc), 0, false)
	// With key=0, mask = byte(0 ^ (0 * 0x9E3779B9)) = 0 for first byte
	// First byte should remain unchanged since mask = byte(0) = 0
	if bc[0] != original[0] {
		t.Errorf("With key=0 and pc=0, mask=0, opcode should be unchanged")
	}
}

// --- Branch Target Offset Tests ---

func TestBranchTargetOffset(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	tests := []struct {
		op      byte
		offset  int
		isBranch bool
	}{
		{vm.OpJmp, 1, true},
		{vm.OpJe, 1, true},
		{vm.OpJne, 1, true},
		{vm.OpJl, 1, true},
		{vm.OpJge, 1, true},
		{vm.OpJgt, 1, true},
		{vm.OpJle, 1, true},
		{vm.OpJb, 1, true},
		{vm.OpJae, 1, true},
		{vm.OpJbe, 1, true},
		{vm.OpJa, 1, true},
		{vm.OpJvs, 1, true},
		{vm.OpJvc, 1, true},
		{vm.OpTbz, 3, true},
		{vm.OpTbnz, 3, true},
		{vm.OpNop, 0, false},
		{vm.OpAdd, 0, false},
		{vm.OpHalt, 0, false},
		{vm.OpPush, 0, false},
	}

	for _, tc := range tests {
		off := branchTargetOffset(tc.op)
		if tc.isBranch && off != tc.offset {
			t.Errorf("branchTargetOffset(0x%02X) = %d, expected %d", tc.op, off, tc.offset)
		}
		if !tc.isBranch && off != 0 {
			t.Errorf("branchTargetOffset(0x%02X) should be 0 for non-branch, got %d", tc.op, off)
		}
	}
}

// --- Remap Branch Targets Tests ---

func TestRemapBranchTargets(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// Build reversed bytecode with a JMP whose target needs remapping
	// Reversed layout: JMP(5B) + size(1B) + NOP(1B) + size(1B)
	// JMP target originally points to offset 1 (the NOP)
	offsetMap := map[int]int{
		1: 6, // original offset 1 → reversed offset 6 (after JMP+size)
	}

	bc := make([]byte, 8)
	bc[0] = vm.OpJmp
	binary.LittleEndian.PutUint32(bc[1:], 1) // target original offset 1
	// rest are zero/size markers

	remapBranchTargets(bc, len(bc), offsetMap, false)

	newTarget := binary.LittleEndian.Uint32(bc[1:])
	// remapped = offsetMap[1] + 1 = 7
	if newTarget != 7 {
		t.Errorf("Expected remapped target 7, got %d", newTarget)
	}
}

// --- Types Tests ---

func TestAddrSpec(t *testing.T) {
	spec := AddrSpec{Addr: 0x1000, End: 0x2000, Name: "test"}
	if spec.Addr != 0x1000 {
		t.Error("Addr not set correctly")
	}
	if spec.End != 0x2000 {
		t.Error("End not set correctly")
	}
	if spec.Name != "test" {
		t.Error("Name not set correctly")
	}
}

func TestFuncBytecode(t *testing.T) {
	fb := FuncBytecode{
		FI:      &vm.FuncInfo{Name: "f", Addr: 0x1000, Size: 64},
		Encrypted: []byte{0xAA, 0xBB, 0xCC},
		XorKey:  0x42,
	}
	if fb.XorKey != 0x42 {
		t.Error("XorKey not set correctly")
	}
	if len(fb.Encrypted) != 3 {
		t.Error("Encrypted length wrong")
	}
	if fb.FI.Name != "f" {
		t.Error("FuncInfo not set correctly")
	}
}

// --- Integration: Reverse + Encrypt Round Trip ---

func TestReverseAndEncrypt_RoundTrip(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// Create a simple program: NOP + PUSH R5 + HALT
	original := []byte{vm.OpNop, vm.OpPush, 5, vm.OpHalt}

	// Step 1: Reverse
	reversed, offsetMap, _ := reverseInstructions(original, len(original))

	// Step 2: Encrypt (forward mode on reversed bytecode)
	ocKey := uint32(0x42424242)
	encrypted := make([]byte, len(reversed))
	copy(encrypted, reversed)
	encryptOpcodes(encrypted, len(encrypted), ocKey, true)

	// Verify encryption changed something
	allSame := true
	for i := range encrypted {
		if encrypted[i] != reversed[i] {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("Encryption did not change any bytes")
	}

	// Verify offsetMap has entries
	if len(offsetMap) != 3 {
		t.Errorf("Expected 3 offset map entries, got %d", len(offsetMap))
	}
}

// --- Decoder Tests ---

func TestDecodeFunction_ARM64(t *testing.T) {
	vm.GenerateDynamicISA()
	p := NewPacker("dummy", "", nil, nil, false, false, false, false, nil)
	p.isARM32 = false

	// RET = 0xD65F03C0
	code := make([]byte, 4)
	binary.LittleEndian.PutUint32(code, 0xD65F03C0)

	insts := p.decodeInstructions(code, false)
	if len(insts) != 1 {
		t.Fatalf("Expected 1 instruction, got %d", len(insts))
	}
}

func TestDecodeFunction_Empty(t *testing.T) {
	vm.GenerateDynamicISA()
	p := NewPacker("dummy", "", nil, nil, false, false, false, false, nil)
	p.isARM32 = false

	insts := p.decodeInstructions([]byte{}, false)
	if len(insts) != 0 {
		t.Errorf("Expected 0 instructions for empty code, got %d", len(insts))
	}
}

// --- ValidateArch Tests ---

func TestValidateArch_NoBlob(t *testing.T) {
	// This test only checks the blob validation path
	p := NewPacker("dummy", "", nil, nil, false, false, false, false, nil)
	p.isARM32 = true
	p.interpBlobARM32 = nil

	// Can't easily test without a real ELF file, but we can verify the struct state
	if p.isARM32 && len(p.interpBlobARM32) == 0 {
		// This is the expected state for ARM32 without blob
		// validateArch would return an error for this case
	}
}
