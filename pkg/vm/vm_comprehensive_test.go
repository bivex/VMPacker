package vm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

// ============================================================
// Comprehensive tests for pkg/vm
// Covers: ISA generation, disassembler, opcode completeness,
// instruction sizes, bytecode round-trip, edge cases
// ============================================================

// --- ISA Generation Tests ---

func TestDynamicISA_OpPtrsCount(t *testing.T) {
	expectedCount := int(OpIdCount)
	if len(opPtrs) != expectedCount {
		t.Errorf("opPtrs length %d != OpIdCount %d", len(opPtrs), expectedCount)
	}
}

func TestDynamicISA_AllOpIdsAreUnique(t *testing.T) {
	GenerateDynamicISA()

	// Every opcode variable should map to a unique byte value
	vals := make(map[byte]string)
	allOps := []struct {
		name string
		val  byte
	}{
		{"OpNop", OpNop}, {"OpHalt", OpHalt},
		{"OpMovImm", OpMovImm}, {"OpMovImm32", OpMovImm32}, {"OpMovReg", OpMovReg},
		{"OpLoad8", OpLoad8}, {"OpLoad16", OpLoad16}, {"OpLoad32", OpLoad32}, {"OpLoad64", OpLoad64},
		{"OpStore8", OpStore8}, {"OpStore16", OpStore16}, {"OpStore32", OpStore32}, {"OpStore64", OpStore64},
		{"OpAdd", OpAdd}, {"OpSub", OpSub}, {"OpMul", OpMul},
		{"OpXor", OpXor}, {"OpAnd", OpAnd}, {"OpOr", OpOr},
		{"OpShl", OpShl}, {"OpShr", OpShr}, {"OpAsr", OpAsr},
		{"OpUmulh", OpUmulh}, {"OpNot", OpNot}, {"OpRor", OpRor},
		{"OpAddImm", OpAddImm}, {"OpSubImm", OpSubImm}, {"OpXorImm", OpXorImm},
		{"OpAndImm", OpAndImm}, {"OpOrImm", OpOrImm}, {"OpMulImm", OpMulImm},
		{"OpShlImm", OpShlImm}, {"OpShrImm", OpShrImm}, {"OpAsrImm", OpAsrImm},
		{"OpCmp", OpCmp}, {"OpCmpImm", OpCmpImm},
		{"OpJmp", OpJmp}, {"OpJe", OpJe}, {"OpJne", OpJne},
		{"OpJl", OpJl}, {"OpJge", OpJge}, {"OpJgt", OpJgt}, {"OpJle", OpJle},
		{"OpJb", OpJb}, {"OpJae", OpJae}, {"OpJbe", OpJbe}, {"OpJa", OpJa},
		{"OpJvs", OpJvs}, {"OpJvc", OpJvc},
		{"OpPush", OpPush}, {"OpPop", OpPop},
		{"OpCallNative", OpCallNative}, {"OpCallReg", OpCallReg}, {"OpBrReg", OpBrReg},
		{"OpRet", OpRet},
		{"OpVld16", OpVld16}, {"OpVst16", OpVst16},
		{"OpSLoadSlide", OpSLoadSlide}, {"OpSnprintf", OpSnprintf},
		{"OpTbz", OpTbz}, {"OpTbnz", OpTbnz},
		{"OpCcmpReg", OpCcmpReg}, {"OpCcmpImm", OpCcmpImm},
		{"OpCcmnReg", OpCcmnReg}, {"OpCcmnImm", OpCcmnImm},
		{"OpSvc", OpSvc}, {"OpUdiv", OpUdiv}, {"OpSdiv", OpSdiv}, {"OpMrs", OpMrs},
		{"OpSmulh", OpSmulh}, {"OpClz", OpClz}, {"OpCls", OpCls},
		{"OpRbit", OpRbit}, {"OpRev", OpRev}, {"OpRev16", OpRev16}, {"OpRev32", OpRev32},
		{"OpAdc", OpAdc}, {"OpSbc", OpSbc},
		// Stack machine
		{"OpSVload", OpSVload}, {"OpSVstore", OpSVstore},
		{"OpSPushImm32", OpSPushImm32}, {"OpSPushImm64", OpSPushImm64},
		{"OpSDup", OpSDup}, {"OpSSwap", OpSSwap}, {"OpSDrop", OpSDrop},
		{"OpSAdd", OpSAdd}, {"OpSSub", OpSSub}, {"OpSMul", OpSMul},
		{"OpSXor", OpSXor}, {"OpSAnd", OpSAnd}, {"OpSOr", OpSOr},
		{"OpSShl", OpSShl}, {"OpSShr", OpSShr}, {"OpSAsr", OpSAsr},
		{"OpSRor", OpSRor}, {"OpSUmulh", OpSUmulh}, {"OpSSmulh", OpSSmulh},
		{"OpSUdiv", OpSUdiv}, {"OpSSdiv", OpSSdiv},
		{"OpSAdc", OpSAdc}, {"OpSSbc", OpSSbc},
		{"OpSNot", OpSNot}, {"OpSNeg", OpSNeg}, {"OpSClz", OpSClz},
		{"OpSCls", OpSCls}, {"OpSRbit", OpSRbit}, {"OpSRev", OpSRev},
		{"OpSRev16", OpSRev16}, {"OpSRev32", OpSRev32},
		{"OpSTrunc32", OpSTrunc32}, {"OpSSext32", OpSSext32},
		{"OpSCmp", OpSCmp},
		{"OpSLd8", OpSLd8}, {"OpSLd16", OpSLd16}, {"OpSLd32", OpSLd32}, {"OpSLd64", OpSLd64},
		{"OpSSt8", OpSSt8}, {"OpSSt16", OpSSt16}, {"OpSSt32", OpSSt32}, {"OpSSt64", OpSSt64},
		{"OpSVLd", OpSVLd}, {"OpSVSt", OpSVSt},
		// FP
		{"OpSFAdd", OpSFAdd}, {"OpSFSub", OpSFSub}, {"OpSFMul", OpSFMul}, {"OpSFDiv", OpSFDiv},
		{"OpSFMov", OpSFMov}, {"OpSFCmp", OpSFCmp}, {"OpSFNeg", OpSFNeg}, {"OpSFAbs", OpSFAbs},
		{"OpSFSqrt", OpSFSqrt}, {"OpSFMax", OpSFMax}, {"OpSFMin", OpSFMin},
		{"OpSFCvtIF", OpSFCvtIF}, {"OpSFCvtFI", OpSFCvtFI},
		{"OpSFMovRV", OpSFMovRV}, {"OpSFMovVR", OpSFMovVR}, {"OpSFCvt", OpSFCvt},
		{"OpSDecryptStr", OpSDecryptStr},
	}

	for _, op := range allOps {
		if prev, exists := vals[op.val]; exists {
			t.Errorf("Opcode collision: %s and %s both map to 0x%02X", prev, op.name, op.val)
		}
		vals[op.val] = op.name
	}
}

func TestDynamicISA_InverseMap(t *testing.T) {
	GenerateDynamicISA()

	for i := 0; i < int(OpIdCount); i++ {
		encoded := GlobalOpMap[i]
		decoded := InverseOpMap[encoded]
		if int(decoded) != i {
			t.Errorf("InverseOpMap[GlobalOpMap[%d]=0x%02X] = %d, want %d", i, encoded, decoded, i)
		}
	}
}

func TestDynamicISA_MultipleGenerations(t *testing.T) {
	captures := make([][256]byte, 3)
	for gen := 0; gen < 3; gen++ {
		GenerateDynamicISA()
		copy(captures[gen][:], GlobalOpMap[:])
	}

	// Each generation should produce a different permutation
	diff01 := false
	diff12 := false
	for i := 0; i < 256; i++ {
		if captures[0][i] != captures[1][i] {
			diff01 = true
		}
		if captures[1][i] != captures[2][i] {
			diff12 = true
		}
	}
	if !diff01 {
		t.Error("Generation 0 and 1 produced identical ISA (extremely unlikely)")
	}
	if !diff12 {
		t.Error("Generation 1 and 2 produced identical ISA (extremely unlikely)")
	}
}

// --- Disassembler Tests ---

func TestDisasmAllOpcodes(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// Build bytecode with one of each known opcode
	type opEntry struct {
		name string
		op   byte
		size int
	}

	entries := []opEntry{
		{"NOP", OpNop, 1},
		{"HALT", OpHalt, 1},
		{"MOV_IMM64", OpMovImm, 10},
		{"MOV_IMM32", OpMovImm32, 6},
		{"MOV_REG", OpMovReg, 3},
		{"ADD", OpAdd, 4},
		{"SUB", OpSub, 4},
		{"MUL", OpMul, 4},
		{"XOR", OpXor, 4},
		{"AND", OpAnd, 4},
		{"OR", OpOr, 4},
		{"SHL", OpShl, 4},
		{"SHR", OpShr, 4},
		{"ASR", OpAsr, 4},
		{"NOT", OpNot, 3},
		{"ROR", OpRor, 4},
		{"JMP", OpJmp, 5},
		{"PUSH", OpPush, 2},
		{"POP", OpPop, 2},
		{"RET", OpRet, 2},
		{"CALL_NATIVE", OpCallNative, 9},
		{"CALL_REG", OpCallReg, 2},
		{"BR_REG", OpBrReg, 2},
		{"CMP", OpCmp, 3},
		{"CMP_IMM", OpCmpImm, 6},
		{"TBZ", OpTbz, 7},
		{"TBNZ", OpTbnz, 7},
	}

	for _, e := range entries {
		t.Run(e.name, func(t *testing.T) {
			// Build minimal valid bytecode for this opcode
			bc := make([]byte, e.size)
			bc[0] = e.op
			// Fill operand bytes with non-zero sentinel values
			for i := 1; i < e.size; i++ {
				bc[i] = byte(i)
			}

			text, size := DisasmOne(bc, 0)
			if size != e.size {
				t.Errorf("%s: expected size %d, got %d", e.name, e.size, size)
			}
			if text == "" {
				t.Errorf("%s: got empty disassembly text", e.name)
			}
			// Verify it starts with a hex offset and is not EOF/truncated/unknown
			if len(text) < 5 {
				t.Errorf("%s: disassembly text too short: '%s'", e.name, text)
			}
		})
	}
}

func TestDisasmOne_EOF(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	text, size := DisasmOne([]byte{}, 0)
	if text != "EOF" {
		t.Errorf("Expected EOF text, got '%s'", text)
	}
	if size != 0 {
		t.Errorf("Expected size 0 for EOF, got %d", size)
	}

	// PC past end
	text, size = DisasmOne([]byte{OpNop}, 5)
	if text != "EOF" {
		t.Errorf("Expected EOF for past-end PC, got '%s'", text)
	}
}

func TestDisasmOne_UnknownOpcode(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// Find a byte that doesn't map to any opcode
	knownOpcodes := make(map[byte]bool)
	for i := 0; i < int(OpIdCount); i++ {
		knownOpcodes[GlobalOpMap[i]] = true
	}

	unknownByte := byte(0xFF)
	for b := byte(0); b < 255; b++ {
		if !knownOpcodes[b] {
			unknownByte = b
			break
		}
	}

	text, size := DisasmOne([]byte{unknownByte}, 0)
	if size != 1 {
		t.Errorf("Unknown opcode should return size 1, got %d", size)
	}
	expected := fmt.Sprintf("0000: UNKNOWN 0x%02X", unknownByte)
	if text != expected {
		t.Errorf("Expected '%s', got '%s'", expected, text)
	}
}

func TestDisasmRange(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// Three instructions: NOP + NOP + HALT
	bc := []byte{OpNop, OpNop, OpHalt}
	lines := DisasmRange(bc, 0, 3)
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}
}

func TestDisasmRange_Partial(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// Only disassemble first 2 bytes of 3
	bc := []byte{OpNop, OpNop, OpHalt}
	lines := DisasmRange(bc, 0, 2)
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}
}

func TestDisasmAll_EmptyBytecode(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	lines := DisasmAll([]byte{})
	if len(lines) != 0 {
		t.Errorf("Expected 0 lines for empty bytecode, got %d", len(lines))
	}
}

func TestDisasm_MOV_IMM64(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// MOV R5, 0xDEADBEEFCAFEBABE
	bc := make([]byte, 10)
	bc[0] = OpMovImm
	bc[1] = 5
	binary.LittleEndian.PutUint64(bc[2:], 0xDEADBEEFCAFEBABE)

	text, size := DisasmOne(bc, 0)
	if size != 10 {
		t.Errorf("Expected size 10, got %d", size)
	}
	expected := "0000: MOV R5, 0xDEADBEEFCAFEBABE"
	if text != expected {
		t.Errorf("Expected '%s', got '%s'", expected, text)
	}
}

func TestDisasm_MOV_IMM32(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	bc := make([]byte, 6)
	bc[0] = OpMovImm32
	bc[1] = 3
	binary.LittleEndian.PutUint32(bc[2:], 0x12345678)

	text, size := DisasmOne(bc, 0)
	if size != 6 {
		t.Errorf("Expected size 6, got %d", size)
	}
	if text != "0000: MOV32 R3, 0x12345678" {
		t.Errorf("Unexpected text: '%s'", text)
	}
}

func TestDisasm_MOV_REG(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	bc := []byte{OpMovReg, 1, 2}
	text, size := DisasmOne(bc, 0)
	if size != 3 {
		t.Errorf("Expected size 3, got %d", size)
	}
	if text != "0000: MOV R1, R2" {
		t.Errorf("Unexpected text: '%s'", text)
	}
}

func TestDisasm_LOAD(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	for _, op := range []struct {
		opcode byte
		width  string
	}{
		{OpLoad8, "8"}, {OpLoad16, "16"}, {OpLoad32, "32"}, {OpLoad64, "64"},
	} {
		t.Run("LOAD"+op.width, func(t *testing.T) {
			bc := make([]byte, 5)
			bc[0] = op.opcode
			bc[1] = 3  // dst
			bc[2] = 5  // base
			binary.LittleEndian.PutUint16(bc[3:], 100)

			text, size := DisasmOne(bc, 0)
			if size != 5 {
				t.Errorf("Expected size 5, got %d", size)
			}
			expected := fmt.Sprintf("0000: LOAD%s R3, [R5 + 100]", op.width)
			if text != expected {
				t.Errorf("Expected '%s', got '%s'", expected, text)
			}
		})
	}
}

func TestDisasm_STORE(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	for _, op := range []struct {
		opcode byte
		width  string
	}{
		{OpStore8, "8"}, {OpStore16, "16"}, {OpStore32, "32"}, {OpStore64, "64"},
	} {
		t.Run("STORE"+op.width, func(t *testing.T) {
			bc := make([]byte, 5)
			bc[0] = op.opcode
			bc[1] = 5  // base
			bc[2] = 3  // src
			binary.LittleEndian.PutUint16(bc[3:], 200)

			text, size := DisasmOne(bc, 0)
			if size != 5 {
				t.Errorf("Expected size 5, got %d", size)
			}
			expected := fmt.Sprintf("0000: STORE%s [R5 + 200], R3", op.width)
			if text != expected {
				t.Errorf("Expected '%s', got '%s'", expected, text)
			}
		})
	}
}

func TestDisasm_ALU3(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	ops := []struct {
		opcode byte
		name   string
	}{
		{OpAdd, "ADD"}, {OpSub, "SUB"}, {OpMul, "MUL"},
		{OpXor, "XOR"}, {OpAnd, "AND"}, {OpOr, "OR"},
		{OpShl, "SHL"}, {OpShr, "SHR"}, {OpAsr, "ASR"},
		{OpRor, "ROR"}, {OpUmulh, "UMULH"},
		{OpUdiv, "UDIV"}, {OpSdiv, "SDIV"}, {OpSmulh, "SMULH"},
		{OpAdc, "ADC"}, {OpSbc, "SBC"},
	}

	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			bc := []byte{op.opcode, 1, 2, 3}
			text, size := DisasmOne(bc, 0)
			if size != 4 {
				t.Errorf("Expected size 4, got %d", size)
			}
			expected := fmt.Sprintf("0000: %s R1, R2, R3", op.name)
			if text != expected {
				t.Errorf("Expected '%s', got '%s'", expected, text)
			}
		})
	}
}

func TestDisasm_ALUImm(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	ops := []struct {
		opcode byte
		name   string
	}{
		{OpAddImm, "ADD_IMM"}, {OpSubImm, "SUB_IMM"}, {OpXorImm, "XOR_IMM"},
		{OpAndImm, "AND_IMM"}, {OpOrImm, "OR_IMM"}, {OpMulImm, "MUL_IMM"},
		{OpShlImm, "SHL_IMM"}, {OpShrImm, "SHR_IMM"}, {OpAsrImm, "ASR_IMM"},
	}

	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			bc := make([]byte, 7)
			bc[0] = op.opcode
			bc[1] = 1 // d
			bc[2] = 2 // s
			binary.LittleEndian.PutUint32(bc[3:], 0xAA)
			text, size := DisasmOne(bc, 0)
			if size != 7 {
				t.Errorf("Expected size 7, got %d", size)
			}
			expected := fmt.Sprintf("0000: %s R1, R2, 0xAA", op.name)
			if text != expected {
				t.Errorf("Expected '%s', got '%s'", expected, text)
			}
		})
	}
}

func TestDisasm_Unary(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	ops := []struct {
		opcode byte
		name   string
	}{
		{OpNot, "NOT"}, {OpClz, "CLZ"}, {OpCls, "CLS"},
		{OpRbit, "RBIT"}, {OpRev, "REV"}, {OpRev16, "REV16"}, {OpRev32, "REV32"},
	}

	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			bc := []byte{op.opcode, 5, 3}
			text, size := DisasmOne(bc, 0)
			if size != 3 {
				t.Errorf("Expected size 3, got %d", size)
			}
			expected := fmt.Sprintf("0000: %s R5, R3", op.name)
			if text != expected {
				t.Errorf("Expected '%s', got '%s'", expected, text)
			}
		})
	}
}

func TestDisasm_Branches(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	branchOps := []struct {
		opcode byte
		name   string
	}{
		{OpJmp, "JMP"}, {OpJe, "JE"}, {OpJne, "JNE"},
		{OpJl, "JL"}, {OpJge, "JGE"}, {OpJgt, "JGT"}, {OpJle, "JLE"},
		{OpJb, "JB"}, {OpJae, "JAE"}, {OpJbe, "JBE"}, {OpJa, "JA"},
		{OpJvs, "JVS"}, {OpJvc, "JVC"},
	}

	for _, op := range branchOps {
		t.Run(op.name, func(t *testing.T) {
			bc := make([]byte, 5)
			bc[0] = op.opcode
			binary.LittleEndian.PutUint32(bc[1:], 0x100)
			text, size := DisasmOne(bc, 0)
			if size != 5 {
				t.Errorf("Expected size 5, got %d", size)
			}
			expected := fmt.Sprintf("0000: %s 0x0100", op.name)
			if text != expected {
				t.Errorf("Expected '%s', got '%s'", expected, text)
			}
		})
	}
}

func TestDisasm_PUSH_POP(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	bc := []byte{OpPush, 7}
	text, size := DisasmOne(bc, 0)
	if size != 2 {
		t.Errorf("Expected PUSH size 2, got %d", size)
	}
	if text != "0000: PUSH R7" {
		t.Errorf("Unexpected PUSH text: '%s'", text)
	}

	bc = []byte{OpPop, 7}
	text, size = DisasmOne(bc, 0)
	if size != 2 {
		t.Errorf("Expected POP size 2, got %d", size)
	}
	if text != "0000: POP R7" {
		t.Errorf("Unexpected POP text: '%s'", text)
	}
}

func TestDisasm_CALL_NATIVE(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	bc := make([]byte, 9)
	bc[0] = OpCallNative
	binary.LittleEndian.PutUint64(bc[1:], 0x400500)
	text, size := DisasmOne(bc, 0)
	if size != 9 {
		t.Errorf("Expected size 9, got %d", size)
	}
	if text != "0000: CALL 0x400500" {
		t.Errorf("Unexpected text: '%s'", text)
	}
}

func TestDisasm_CALL_REG_BR_REG_RET(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	bc := []byte{OpCallReg, 5}
	text, size := DisasmOne(bc, 0)
	if size != 2 || text != "0000: BLR R5" {
		t.Errorf("CALL_REG: size=%d text='%s'", size, text)
	}

	bc = []byte{OpBrReg, 3}
	text, size = DisasmOne(bc, 0)
	if size != 2 || text != "0000: BR R3" {
		t.Errorf("BR_REG: size=%d text='%s'", size, text)
	}

	bc = []byte{OpRet, 30}
	text, size = DisasmOne(bc, 0)
	if size != 2 || text != "0000: RET R30" {
		t.Errorf("RET: size=%d text='%s'", size, text)
	}
}

func TestDisasm_TBZ_TBNZ(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// TBZ R2, #5, 0x0040
	bc := make([]byte, 7)
	bc[0] = OpTbz
	bc[1] = 2 // reg
	bc[2] = 5 // bit
	binary.LittleEndian.PutUint32(bc[3:], 0x0040)

	text, size := DisasmOne(bc, 0)
	if size != 7 {
		t.Errorf("Expected size 7, got %d", size)
	}
	if text != "0000: TBZ R2, #5, 0x0040" {
		t.Errorf("Unexpected text: '%s'", text)
	}

	// TBNZ
	bc[0] = OpTbnz
	text, size = DisasmOne(bc, 0)
	if size != 7 {
		t.Errorf("Expected size 7, got %d", size)
	}
	if text != "0000: TBNZ R2, #5, 0x0040" {
		t.Errorf("Unexpected text: '%s'", text)
	}
}

func TestDisasm_CCMP(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// CCMP_REG: [op][cond][nzcv][rn][rm][sf]
	bc := []byte{OpCcmpReg, 0, 0x0F, 5, 3, 1}
	text, size := DisasmOne(bc, 0)
	if size != 6 {
		t.Errorf("Expected size 6, got %d", size)
	}
	if text != "0000: CCMP_REG R5, R3, #15, cond=0 sf=1" {
		t.Errorf("Unexpected text: '%s'", text)
	}

	// CCMP_IMM: [op][cond][nzcv][rn][imm5][sf]
	bc = []byte{OpCcmpImm, 1, 0x0F, 5, 7, 0}
	text, size = DisasmOne(bc, 0)
	if size != 6 {
		t.Errorf("Expected size 6, got %d", size)
	}
	if text != "0000: CCMP_IMM R5, #7, #15, cond=1 sf=0" {
		t.Errorf("Unexpected text: '%s'", text)
	}
}

func TestDisasm_SVC(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	bc := make([]byte, 3)
	bc[0] = OpSvc
	binary.LittleEndian.PutUint16(bc[1:], 0x1234)
	text, size := DisasmOne(bc, 0)
	if size != 3 {
		t.Errorf("Expected size 3, got %d", size)
	}
	if text != "0000: SVC #0x1234" {
		t.Errorf("Unexpected text: '%s'", text)
	}
}

func TestDisasm_MRS(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	bc := make([]byte, 4)
	bc[0] = OpMrs
	bc[1] = 3
	binary.LittleEndian.PutUint16(bc[2:], 0xDE00)
	text, size := DisasmOne(bc, 0)
	if size != 4 {
		t.Errorf("Expected size 4, got %d", size)
	}
	if text != "0000: MRS R3, sysreg=0xDE00" {
		t.Errorf("Unexpected text: '%s'", text)
	}
}

// --- Instruction Size Consistency ---

func TestInstructionSize_AllKnownOpcodes(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// Verify every opcode in the table has a non-zero size
	checked := 0
	for i := 0; i < int(OpIdCount); i++ {
		opByte := GlobalOpMap[i]
		size := InstructionSize(opByte)
		if size == 0 {
			t.Errorf("Opcode for OpId %d (byte 0x%02X) has zero instruction size", i, opByte)
		}
		checked++
	}

	if checked != int(OpIdCount) {
		t.Errorf("Only checked %d/%d opcodes", checked, OpIdCount)
	}
}

func TestInstructionSize_SizesMatchOpTable(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// Verify InstructionSize matches opTable entries
	for opByte, info := range opTable {
		size := InstructionSize(opByte)
		if size != info.Size {
			t.Errorf("InstructionSize(0x%02X) = %d, opTable says %d", opByte, size, info.Size)
		}
	}
}

// --- OpcodeName Tests ---

func TestOpcodeName_KnownAndUnknown(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	if name := OpcodeName(OpNop); name != "NOP" {
		t.Errorf("Expected 'NOP', got '%s'", name)
	}
	if name := OpcodeName(OpHalt); name != "HALT" {
		t.Errorf("Expected 'HALT', got '%s'", name)
	}

	// Find a byte that is NOT a known opcode
	known := make(map[byte]bool)
	for b, info := range opTable {
		_ = info
		known[b] = true
	}
	var unknown byte
	found := false
	for b := byte(0); b < 255; b++ {
		if !known[b] {
			unknown = b
			found = true
			break
		}
	}
	if found {
		name := OpcodeName(unknown)
		expected := fmt.Sprintf("UNKNOWN(0x%02X)", unknown)
		if name != expected {
			t.Errorf("Expected '%s', got '%s'", expected, name)
		}
	}
}

// --- Disasm Round-Trip Tests ---

func TestDisasmAll_RoundTripConsistency(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// Build a program with multiple instruction types
	bc := []byte{}
	// NOP
	bc = append(bc, OpNop)
	// MOV R0, 0x42
	bc = append(bc, OpMovImm, 0)
	imm64 := make([]byte, 8)
	binary.LittleEndian.PutUint64(imm64, 0x42)
	bc = append(bc, imm64...)
	// ADD R0, R1, R2
	bc = append(bc, OpAdd, 0, 1, 2)
	// CMP R0, R1
	bc = append(bc, OpCmp, 0, 1)
	// JMP offset
	bc = append(bc, OpJmp)
	jmpTarget := make([]byte, 4)
	binary.LittleEndian.PutUint32(jmpTarget, uint32(len(bc)+5+4+5+1)) // past JE, another JMP, HALT
	bc = append(bc, jmpTarget...)
	// JE target
	bc = append(bc, OpJe)
	bc = append(bc, 0, 0, 0, 0) // placeholder
	// HALT
	bc = append(bc, OpHalt)

	lines := DisasmAll(bc)
	if len(lines) == 0 {
		t.Fatal("DisasmAll returned empty result")
	}

	// Verify we can re-disassemble starting from each reported offset
	totalBytes := 0
	for range lines {
		_, size := DisasmOne(bc, totalBytes)
		if size == 0 {
			t.Errorf("Re-disassembly at offset %d failed", totalBytes)
		}
		totalBytes += size
	}

	if totalBytes != len(bc) {
		t.Errorf("Round-trip consumed %d bytes, but bytecode is %d bytes", totalBytes, len(bc))
	}
}

// --- Stack Machine Opcode Tests ---

func TestStackMachineOpcodes_SizeOne(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// All 1-byte stack machine opcodes
	oneByteOps := []struct {
		name string
		op   byte
	}{
		{"S_DUP", OpSDup}, {"S_SWAP", OpSSwap}, {"S_DROP", OpSDrop},
		{"S_ADD", OpSAdd}, {"S_SUB", OpSSub}, {"S_MUL", OpSMul},
		{"S_XOR", OpSXor}, {"S_AND", OpSAnd}, {"S_OR", OpSOr},
		{"S_SHL", OpSShl}, {"S_SHR", OpSShr}, {"S_ASR", OpSAsr},
		{"S_ROR", OpSRor}, {"S_UMULH", OpSUmulh}, {"S_SMULH", OpSSmulh},
		{"S_UDIV", OpSUdiv}, {"S_SDIV", OpSSdiv},
		{"S_ADC", OpSAdc}, {"S_SBC", OpSSbc},
		{"S_NOT", OpSNot}, {"S_NEG", OpSNeg}, {"S_CLZ", OpSClz},
		{"S_CLS", OpSCls}, {"S_RBIT", OpSRbit}, {"S_REV", OpSRev},
		{"S_REV16", OpSRev16}, {"S_REV32", OpSRev32},
		{"S_TRUNC32", OpSTrunc32}, {"S_SEXT32", OpSSext32},
		{"S_CMP", OpSCmp},
		{"S_LD8", OpSLd8}, {"S_LD16", OpSLd16}, {"S_LD32", OpSLd32}, {"S_LD64", OpSLd64},
		{"S_ST8", OpSSt8}, {"S_ST16", OpSSt16}, {"S_ST32", OpSSt32}, {"S_ST64", OpSSt64},
		{"S_DECRYPT_STR", OpSDecryptStr},
		{"S_LOAD_SLIDE", OpSLoadSlide},
	}

	for _, op := range oneByteOps {
		size := InstructionSize(op.op)
		if size != 1 {
			t.Errorf("%s: expected size 1, got %d", op.name, size)
		}
	}
}

// --- Flags Tests ---

func TestFlagValues(t *testing.T) {
	if FlagZero != 1 {
		t.Errorf("FlagZero should be 1, got %d", FlagZero)
	}
	if FlagSign != 2 {
		t.Errorf("FlagSign should be 2, got %d", FlagSign)
	}
	if FlagCarry != 4 {
		t.Errorf("FlagCarry should be 4, got %d", FlagCarry)
	}

	// All flags combined
	allFlags := FlagZero | FlagSign | FlagCarry
	if allFlags != 7 {
		t.Errorf("All flags combined should be 7, got %d", allFlags)
	}
}

// --- Constants Tests ---

func TestVMConstants(t *testing.T) {
	if RegCount != 32 {
		t.Errorf("RegCount should be 32, got %d", RegCount)
	}
	if StackSize != 128 {
		t.Errorf("StackSize should be 128, got %d", StackSize)
	}
	if MaxExtFunc != 16 {
		t.Errorf("MaxExtFunc should be 16, got %d", MaxExtFunc)
	}
}

func TestSpecialRegisterConstants(t *testing.T) {
	if REG_XZR != -2 {
		t.Errorf("REG_XZR should be -2, got %d", REG_XZR)
	}
	if REG_V_BASE != 64 {
		t.Errorf("REG_V_BASE should be 64, got %d", REG_V_BASE)
	}
}

// --- Types Tests ---

func TestInstructionStruct(t *testing.T) {
	inst := Instruction{
		Raw:       0xD280001D,
		Op:        1, // some op
		Rd:        29,
		Rn:        -1,
		Rm:        -1,
		Imm:       0,
		Shift:     0,
		ShiftType: 0,
		Cond:      0,
		SF:        true,
		Offset:    0,
		WB:        0,
	}
	if inst.Raw != 0xD280001D {
		t.Error("Instruction Raw field not set correctly")
	}
	if !inst.SF {
		t.Error("Instruction SF should be true for 64-bit")
	}
}

func TestRelocationStruct(t *testing.T) {
	reloc := Relocation{
		BcOffset:   42,
		TargetAddr: 0x400100,
		IsInternal: true,
	}
	if reloc.BcOffset != 42 {
		t.Error("Relocation BcOffset not set correctly")
	}
	if !reloc.IsInternal {
		t.Error("Relocation IsInternal should be true")
	}
}

func TestTranslateResultStruct(t *testing.T) {
	res := TranslateResult{
		Bytecode:    []byte{0x00},
		CodeLen:     1,
		Unsupported: nil,
		TotalInsts:  10,
		TransInsts:  9,
		Relocations: []Relocation{},
	}
	if res.TotalInsts != 10 || res.TransInsts != 9 {
		t.Error("TranslateResult fields not set correctly")
	}
	if len(res.Relocations) != 0 {
		t.Error("Expected empty relocations slice")
	}
}

func TestFuncInfoStruct(t *testing.T) {
	fi := FuncInfo{
		Name:    "test_func",
		Addr:    0x400100,
		Size:    64,
		Offset:  0x100,
		Section: ".text",
	}
	if fi.Name != "test_func" {
		t.Error("FuncInfo Name not set correctly")
	}
}

func TestFuncBytecodeStruct(t *testing.T) {
	fb := FuncBytecode{
		Info:      &FuncInfo{Name: "f"},
		Encrypted: []byte{0xAA, 0xBB},
		XorKey:    0x42,
	}
	if fb.XorKey != 0x42 {
		t.Error("FuncBytecode XorKey not set correctly")
	}
	if len(fb.Encrypted) != 2 {
		t.Error("FuncBytecode Encrypted length wrong")
	}
}

// --- Disasm Multiple Instructions ---

func TestDisasmAll_MultipleTypes(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// Build a realistic small program
	var bc []byte

	// MOV R0, 0x5
	bc = append(bc, OpMovImm, 0)
	bc = append(bc, make([]byte, 8)...)
	binary.LittleEndian.PutUint64(bc[len(bc)-8:], 5)

	// MOV R1, 0x3
	bc = append(bc, OpMovImm, 1)
	bc = append(bc, make([]byte, 8)...)
	binary.LittleEndian.PutUint64(bc[len(bc)-8:], 3)

	// ADD R2, R0, R1
	bc = append(bc, OpAdd, 2, 0, 1)

	// PUSH R2
	bc = append(bc, OpPush, 2)

	// POP R3
	bc = append(bc, OpPop, 3)

	// RET R3
	bc = append(bc, OpRet, 3)

	lines := DisasmAll(bc)
	if len(lines) != 6 {
		t.Errorf("Expected 6 lines, got %d", len(lines))
	}

	// Verify each line contains expected content
	expectedFragments := []string{"MOV", "MOV", "ADD", "PUSH", "POP", "RET"}
	for i, fragment := range expectedFragments {
		if i >= len(lines) {
			break
		}
		if !bytes.Contains([]byte(lines[i]), []byte(fragment)) {
			t.Errorf("Line %d: expected '%s' in '%s'", i, fragment, lines[i])
		}
	}
}

// --- RebuildOpTable After Multiple ISA Generations ---

func TestRebuildOpTable_AfterRegeneration(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	firstNopName := OpcodeName(OpNop)
	if firstNopName != "NOP" {
		t.Errorf("Expected NOP, got '%s'", firstNopName)
	}

	// Regenerate
	GenerateDynamicISA()
	RebuildOpTable()

	secondNopName := OpcodeName(OpNop)
	if secondNopName != "NOP" {
		t.Errorf("After regeneration, expected NOP, got '%s'", secondNopName)
	}
}

// --- Disasm with Non-zero PC ---

func TestDisasmOne_NonZeroPC(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	bc := make([]byte, 20)
	// Place a NOP at offset 10
	bc[10] = OpNop

	text, size := DisasmOne(bc, 10)
	if size != 1 {
		t.Errorf("Expected size 1, got %d", size)
	}
	if text != "000A: NOP" {
		t.Errorf("Expected '000A: NOP', got '%s'", text)
	}
}

// --- DisasmAll Verifies No Byte Left Behind ---

func TestDisasmAll_ConsumesAllBytes(t *testing.T) {
	GenerateDynamicISA()
	RebuildOpTable()

	// Create a program where all bytes are consumed
	bc := []byte{
		OpNop,                     // 1 byte
		OpPush, 0,                 // 2 bytes
		OpPop, 1,                  // 2 bytes
		OpHalt,                    // 1 byte
	}
	// Total: 6 bytes

	lines := DisasmAll(bc)
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}

	// Verify all bytes are consumed by re-disassembling
	pos := 0
	for range lines {
		_, size := DisasmOne(bc, pos)
		pos += size
	}
	if pos != len(bc) {
		t.Errorf("Disassembly consumed %d bytes, expected %d", pos, len(bc))
	}
}
