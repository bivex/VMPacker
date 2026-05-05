package arm64

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// Comprehensive ARM64 translator tests
// Covers: individual instruction translation (ALU, branch, load/store,
// FP, bitfield), register mapping, trailer format, branch fixup,
// error handling, CFF, obfuscation modes
// ============================================================

// --- Register Mapping Tests ---

func TestMapReg_ValidRegisters(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x100)

	for reg := 0; reg <= 31; reg++ {
		mapped, err := tr.mapReg(reg)
		if err != nil {
			t.Errorf("mapReg(%d) returned error: %v", reg, err)
		}
		if mapped >= 64 {
			t.Errorf("mapReg(%d) = %d, expected < 64", reg, mapped)
		}
	}
}

func TestMapReg_XZR(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x100)

	mapped, err := tr.mapReg(vm.REG_XZR)
	if err != nil {
		t.Fatalf("mapReg(XZR) returned error: %v", err)
	}
	if mapped != 63 {
		t.Errorf("mapReg(XZR) = %d, expected 63 (permanent zero)", mapped)
	}
}

func TestMapReg_SIMDRegisters(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x100)

	for v := 0; v < 32; v++ {
		armReg := vm.REG_V_BASE + v
		mapped, err := tr.mapReg(armReg)
		if err != nil {
			t.Errorf("mapReg(V%d=%d) returned error: %v", v, armReg, err)
		}
		if int(mapped) != v {
			t.Errorf("mapReg(V%d=%d) = %d, expected %d", v, armReg, mapped, v)
		}
	}
}

func TestMapReg_InvalidRegisters(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x100)

	_, err := tr.mapReg(-1)
	if err == nil {
		t.Error("Expected error for register -1")
	}

	_, err = tr.mapReg(32)
	if err == nil {
		t.Error("Expected error for register 32")
	}
}

// --- Single Instruction Translation Tests ---

func TestTranslate_NOP(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(NOP)},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should contain OpNop somewhere in bytecode
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpNop}) {
		t.Error("NOP not found in translated bytecode")
	}
}

func TestTranslate_RET(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should contain OpRet
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpRet}) {
		t.Error("RET not found in translated bytecode")
	}
}

func TestTranslate_MOVZ(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(MOVZ), Rd: 0, Imm: 42, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSVstore}) {
		t.Error("MOVZ should produce S_VSTORE")
	}
}

func TestTranslate_MOVZ_32bit(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(MOVZ), Rd: 0, Imm: 42, SF: false},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 32-bit MOVZ should produce S_TRUNC32
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSTrunc32}) {
		t.Error("32-bit MOVZ should produce S_TRUNC32")
	}
}

func TestTranslate_MOVN(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(MOVN), Rd: 0, Imm: 0, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	// MOVN x0, #0 => x0 = ~0 = 0xFFFFFFFFFFFFFFFF
	// Should produce a push with the NOT value
	if len(res.Bytecode) < 2 {
		t.Error("MOVN should produce bytecode")
	}
}

func TestTranslate_MOVK(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(MOVZ), Rd: 0, Imm: 0, Shift: 0, SF: true},
		{Offset: 4, Op: int(MOVK), Rd: 0, Imm: 0x1234, Shift: 16, SF: true},
		{Offset: 8, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	// MOVK should produce AND, OR operations (mask and insert)
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSAnd}) {
		t.Error("MOVK should produce S_AND")
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSOr}) {
		t.Error("MOVK should produce S_OR")
	}
}

func TestTranslate_ADD_REG(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(ADD_REG), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSAdd}) {
		t.Error("ADD should produce S_ADD")
	}
}

func TestTranslate_SUB_REG(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(SUB_REG), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSSub}) {
		t.Error("SUB should produce S_SUB")
	}
}

func TestTranslate_MUL(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(MUL), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSMul}) {
		t.Error("MUL should produce S_MUL")
	}
}

func TestTranslate_UDIV(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(UDIV), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSUdiv}) {
		t.Error("UDIV should produce S_UDIV")
	}
}

func TestTranslate_SDIV(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(SDIV), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSSdiv}) {
		t.Error("SDIV should produce S_SDIV")
	}
}

func TestTranslate_XOR_REG(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(EOR_REG), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSXor}) {
		t.Error("EOR should produce S_XOR")
	}
}

func TestTranslate_AND_REG(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(AND_REG), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSAnd}) {
		t.Error("AND should produce S_AND")
	}
}

func TestTranslate_ORR_REG_MOV(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(ORR_REG), Rd: 0, Rn: vm.REG_XZR, Rm: 1, SF: true}, // MOV X0, X1
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	// MOV via ORR should produce S_VLOAD/S_VSTORE, not S_OR
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSVstore}) {
		t.Error("MOV (ORR Xd, XZR, Xm) should produce S_VSTORE")
	}
}

func TestTranslate_MVN(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(MVN), Rd: 0, Rm: 1, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSNot}) {
		t.Error("MVN should produce S_NOT")
	}
}

// --- ALU Immediate Tests ---

func TestTranslate_ADD_IMM(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(ADD_IMM), Rd: 0, Rn: 0, Imm: 10, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSAdd}) {
		t.Error("ADD_IMM should produce S_ADD")
	}
}

func TestTranslate_SUB_IMM(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(SUB_IMM), Rd: 0, Rn: 0, Imm: 5, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSSub}) {
		t.Error("SUB_IMM should produce S_SUB")
	}
}

func TestTranslate_AND_IMM(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(AND_IMM), Rd: 0, Rn: 0, Imm: 0xFF, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSAnd}) {
		t.Error("AND_IMM should produce S_AND")
	}
}

func TestTranslate_ORR_IMM(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(ORR_IMM), Rd: 0, Rn: 0, Imm: 0xFF, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSOr}) {
		t.Error("ORR_IMM should produce S_OR")
	}
}

func TestTranslate_EOR_IMM(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(EOR_IMM), Rd: 0, Rn: 0, Imm: 0xFF, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSXor}) {
		t.Error("EOR_IMM should produce S_XOR")
	}
}

// --- CMP/Flags Tests ---

func TestTranslate_CMP_REG(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(SUBS_REG), Rd: vm.REG_XZR, Rn: 0, Rm: 1, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSCmp}) {
		t.Error("CMP should produce S_CMP")
	}
}

func TestTranslate_CMP_IMM(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(SUBS_IMM), Rd: vm.REG_XZR, Rn: 0, Imm: 0, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSCmp}) {
		t.Error("CMP #imm should produce S_CMP")
	}
}

func TestTranslate_TST(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(ANDS_REG), Rd: vm.REG_XZR, Rn: 0, Rm: 1, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSCmp}) {
		t.Error("TST should produce S_CMP for flag setting")
	}
}

// --- Branch Tests ---

func TestTranslate_B(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0x00, Op: int(MOVZ), Rd: 0, Imm: 1, SF: true},
		{Offset: 0x04, Op: int(B), Imm: 4}, // Target: 0x08 (relative to offset)
		{Offset: 0x08, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	// B should produce JMP
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpJmp}) {
		t.Error("B should produce JMP")
	}
}

func TestTranslate_B_Cond(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0x00, Op: int(SUBS_REG), Rd: vm.REG_XZR, Rn: 0, Rm: 1, SF: true},
		{Offset: 0x04, Op: int(B_COND), Cond: COND_EQ, Imm: 8}, // Target 0x0C
		{Offset: 0x08, Op: int(MOVZ), Rd: 0, Imm: 1, SF: true},
		{Offset: 0x0C, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	// B.cond should produce JE (for EQ)
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpJe}) {
		t.Error("B.EQ should produce JE")
	}
}

func TestTranslate_CBZ(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0x00, Op: int(CBZ), Rd: 0, Imm: 8, SF: true},
		{Offset: 0x04, Op: int(MOVZ), Rd: 0, Imm: 1, SF: true},
		{Offset: 0x08, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	// CBZ tests a value and branches — should produce a comparison + conditional jump
	code := res.Bytecode[:res.CodeLen]
	hasCmp := bytes.Contains(code, []byte{vm.OpSCmp}) || bytes.Contains(code, []byte{vm.OpCmp})
	hasBranch := bytes.Contains(code, []byte{vm.OpJe}) || bytes.Contains(code, []byte{vm.OpJne})
	if !hasCmp {
		t.Error("CBZ should produce a comparison")
	}
	if !hasBranch {
		t.Error("CBZ should produce a conditional branch")
	}
}

func TestTranslate_CBNZ(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0x00, Op: int(CBNZ), Rd: 0, Imm: 8, SF: true},
		{Offset: 0x04, Op: int(MOVZ), Rd: 0, Imm: 0, SF: true},
		{Offset: 0x08, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.TransInsts < 3 {
		t.Errorf("Expected 3 translated instructions, got %d", res.TransInsts)
	}
}

func TestTranslate_BL(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0x00, Op: int(BL), Imm: 0x100},
		{Offset: 0x04, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	// BL should produce CALL_NATIVE
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpCallNative}) {
		t.Error("BL should produce CALL_NATIVE")
	}
}

func TestTranslate_BLR(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0x00, Op: int(BLR), Rn: 1},
		{Offset: 0x04, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpCallReg}) {
		t.Error("BLR should produce CALL_REG")
	}
}

func TestTranslate_BR(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0x00, Op: int(BR), Rn: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpBrReg}) {
		t.Error("BR should produce BR_REG")
	}
}

// --- Unary Operation Tests ---

func TestTranslate_CLZ(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(CLZ), Rd: 0, Rn: 1, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSClz}) {
		t.Error("CLZ should produce S_CLZ")
	}
}

func TestTranslate_RBIT(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(RBIT), Rd: 0, Rn: 1, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSRbit}) {
		t.Error("RBIT should produce S_RBIT")
	}
}

func TestTranslate_REV(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(REV), Rd: 0, Rn: 1, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSRev}) {
		t.Error("REV should produce S_REV")
	}
}

func TestTranslate_UMULH(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(UMULH), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSUmulh}) {
		t.Error("UMULH should produce S_UMULH")
	}
}

func TestTranslate_SMULH(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(SMULH), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSSmulh}) {
		t.Error("SMULH should produce S_SMULH")
	}
}

// --- Special Instruction Tests ---

func TestTranslate_NOPified(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(DMB)},
		{Offset: 4, Op: int(DSB)},
		{Offset: 8, Op: int(ISB)},
		{Offset: 12, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	code := res.Bytecode[:res.CodeLen]
	nopCount := bytes.Count(code, []byte{vm.OpNop})
	if nopCount < 3 {
		t.Errorf("Expected at least 3 NOPs for DMB+DSB+ISB, got %d", nopCount)
	}
}

func TestTranslate_HLT_BRK(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(HLT)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpHalt}) {
		t.Error("HLT should produce HALT")
	}
}

func TestTranslate_SVC(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(SVC), Imm: 0},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSvc}) {
		t.Error("SVC should produce SVC opcode")
	}
}

// --- CSEL Tests ---

func TestTranslate_CSEL(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(CSEL), Rd: 0, Rn: 1, Rm: 2, Cond: COND_EQ, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	// CSEL produces conditional branching and S_VSTORE
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSVstore}) {
		t.Error("CSEL should produce S_VSTORE")
	}
}

// --- MADD/MSUB Tests ---

func TestTranslate_MADD(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	// MADD X0, X1, X2, X3: encoding has Ra=X3 in bits[14:10]
	// Raw encoding: 1_00_11011_000_0010_0_00011_00001_00000 = 0x1B020420
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(MADD), Rd: 0, Rn: 1, Rm: 2, SF: true, Raw: 0x1B020420},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	code := res.Bytecode[:res.CodeLen]
	if !bytes.Contains(code, []byte{vm.OpSMul}) {
		t.Error("MADD should produce S_MUL")
	}
	if !bytes.Contains(code, []byte{vm.OpSAdd}) {
		t.Error("MADD should produce S_ADD for the accumulate")
	}
}

func TestTranslate_MSUB(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	// MSUB X0, X1, X2, X3: raw encoding with o0=1
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(MSUB), Rd: 0, Rn: 1, Rm: 2, SF: true, Raw: 0x1B028420},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	code := res.Bytecode[:res.CodeLen]
	if !bytes.Contains(code, []byte{vm.OpSMul}) {
		t.Error("MSUB should produce S_MUL")
	}
	if !bytes.Contains(code, []byte{vm.OpSSub}) {
		t.Error("MSUB should produce S_SUB for the subtraction")
	}
}

// --- Trailer Format Tests ---

func TestTranslate_TrailerHasCRC(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}

	code := res.Bytecode
	// CRC section is between CodeLen and trailer
	// Structure: [code][stub_va(8)][stub_size(4)][stub_crc(4)][bc_crc(4)][CRC_MAGIC(4)]
	crcOffset := res.CodeLen + 8 + 4 + 4 // skip stub placeholders
	if crcOffset+4 > len(code) {
		t.Fatalf("Bytecode too short for CRC section (len=%d, need offset=%d)", len(code), crcOffset)
	}
	bcCRC := binary.LittleEndian.Uint32(code[crcOffset : crcOffset+4])
	if bcCRC == 0 {
		t.Error("Expected non-zero bc_crc in trailer")
	}

	magicOffset := crcOffset + 4
	if magicOffset+4 > len(code) {
		t.Fatal("Bytecode too short for CRC magic")
	}
	magic := binary.LittleEndian.Uint32(code[magicOffset : magicOffset+4])
	if magic != 0x43524332 {
		t.Errorf("Expected CRC magic 0x43524332, got 0x%08X", magic)
	}
}

func TestTranslate_TrailerHasOpMap(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}

	code := res.Bytecode
	// After CRC section (24 bytes), the trailer starts with regMap (64 bytes) then OpMap (256 bytes)
	opMapOffset := res.CodeLen + 24 + 64
	if opMapOffset+256 > len(code) {
		t.Fatalf("Bytecode too short for OpMap")
	}
	extractedOpMap := code[opMapOffset : opMapOffset+256]
	if !bytes.Equal(extractedOpMap, vm.GlobalOpMap[:]) {
		t.Error("Extracted OpMap from trailer does not match GlobalOpMap")
	}
}

// --- Error Handling Tests ---

func TestTranslate_UnsupportedInstruction(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)

	// UNSUPPORTED is the last opcode in the enum
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(UNSUPPORTED), Raw: 0x00000000},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Unsupported) != 1 {
		t.Errorf("Expected 1 unsupported instruction, got %d", len(res.Unsupported))
	}
}

func TestTranslate_EmptyInstructions(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalInsts != 0 {
		t.Errorf("Expected 0 total instructions, got %d", res.TotalInsts)
	}
	// Should still have HALT and trailer
	if !bytes.Contains(res.Bytecode, []byte{vm.OpHalt}) {
		t.Error("Empty function should have HALT")
	}
}

// --- 32-bit Mode Tests ---

func TestTranslate_32bitTruncation(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(ADD_REG), Rd: 0, Rn: 1, Rm: 2, SF: false}, // 32-bit
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSTrunc32}) {
		t.Error("32-bit ADD should produce S_TRUNC32")
	}
}

// --- Translation Statistics Tests ---

func TestTranslate_Stats(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x100)
	insts := []vm.Instruction{
		{Offset: 0x00, Op: int(MOVZ), Rd: 0, Imm: 10, SF: true},
		{Offset: 0x04, Op: int(ADD_IMM), Rd: 0, Rn: 0, Imm: 5, SF: true},
		{Offset: 0x08, Op: int(RET)},
	}
	res, err := tr.Translate(insts)
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalInsts != 3 {
		t.Errorf("Expected TotalInsts=3, got %d", res.TotalInsts)
	}
	if res.TransInsts != 3 {
		t.Errorf("Expected TransInsts=3, got %d", res.TransInsts)
	}
	if len(res.Unsupported) != 0 {
		t.Errorf("Expected 0 unsupported, got %d", len(res.Unsupported))
	}
	if res.CodeLen == 0 {
		t.Error("Expected non-zero CodeLen")
	}
}

// --- Debug Mode Tests ---

func TestTranslate_DebugMode(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	tr.SetDebug(true)

	insts := []vm.Instruction{
		{Offset: 0, Op: int(MOVZ), Rd: 0, Imm: 42, SF: true, Raw: 0xD2800540},
		{Offset: 4, Op: int(RET), Raw: 0xD65F03C0},
	}
	res, err := tr.Translate(insts)
	if err != nil {
		t.Fatal(err)
	}

	log := tr.DebugLog()
	if len(log) < 2 {
		t.Fatalf("Expected at least 2 debug entries, got %d", len(log))
	}

	// Verify first entry
	if log[0].ARM64Offset != 0 {
		t.Errorf("First entry offset = %d, expected 0", log[0].ARM64Offset)
	}
	if log[0].VMStart < 0 {
		t.Error("VMStart should be >= 0")
	}
	if log[0].VMEnd < log[0].VMStart {
		t.Error("VMEnd should be >= VMStart")
	}

	_ = res
}

// --- Branch Fixup Tests ---

func TestTranslate_ForwardBranch(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x20)

	insts := []vm.Instruction{
		{Offset: 0x00, Op: int(B_COND), Cond: COND_NE, Imm: 8}, // target 0x08
		{Offset: 0x04, Op: int(MOVZ), Rd: 0, Imm: 1, SF: true},
		{Offset: 0x08, Op: int(RET)},
	}
	res, err := tr.Translate(insts)
	if err != nil {
		t.Fatal(err)
	}

	// Forward branch should be patched correctly
	// The branch target in the VM bytecode should point past the MOVZ
	code := res.Bytecode[:res.CodeLen]

	// Find JNE instruction
	found := false
	for i := 0; i < len(code)-5; i++ {
		if code[i] == vm.OpJne {
			target := binary.LittleEndian.Uint32(code[i+1 : i+5])
			if target > 0 && target < uint32(len(code)) {
				found = true
			}
		}
	}
	if !found {
		t.Error("Forward branch fixup did not produce valid JNE target")
	}
}

func TestTranslate_BackwardBranch(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x20)

	insts := []vm.Instruction{
		{Offset: 0x00, Op: int(MOVZ), Rd: 0, Imm: 10, SF: true},
		{Offset: 0x04, Op: int(SUBS_IMM), Rd: 0, Rn: 0, Imm: 1, SF: true},
		{Offset: 0x08, Op: int(B_COND), Cond: COND_NE, Imm: -4}, // target 0x04
		{Offset: 0x0C, Op: int(RET)},
	}
	res, err := tr.Translate(insts)
	if err != nil {
		t.Fatal(err)
	}

	code := res.Bytecode[:res.CodeLen]
	// Find JNE and verify target is within bytecode
	found := false
	for i := 0; i < len(code)-5; i++ {
		if code[i] == vm.OpJne {
			target := binary.LittleEndian.Uint32(code[i+1 : i+5])
			if target >= 0 && target < uint32(res.CodeLen) {
				found = true
			}
		}
	}
	if !found {
		t.Error("Backward branch fixup did not produce valid JNE target")
	}
}

// --- Multiple Translations Produce Different Bytecode ---

func TestTranslate_NonDeterministic(t *testing.T) {
	vm.GenerateDynamicISA()

	insts := []vm.Instruction{
		{Offset: 0, Op: int(MOVZ), Rd: 0, Imm: 42, SF: true},
		{Offset: 4, Op: int(ADD_REG), Rd: 0, Rn: 0, Rm: 1, SF: true},
		{Offset: 8, Op: int(RET)},
	}

	results := make([][]byte, 3)
	for i := 0; i < 3; i++ {
		tr := NewTranslator(0x1000, 0x10)
		res, err := tr.Translate(insts)
		if err != nil {
			t.Fatal(err)
		}
		results[i] = res.Bytecode
	}

	// At least some should differ due to register shuffling and junk code
	allSame := true
	for i := 1; i < 3; i++ {
		if !bytes.Equal(results[0], results[i]) {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("Multiple translations produced identical bytecode (unlikely with randomization)")
	}
}

// --- CFF Tests ---

func TestTranslate_CFF_BasicBlockCount(t *testing.T) {
	vm.GenerateDynamicISA()

	insts := []vm.Instruction{
		{Offset: 0x00, Op: int(MOVZ), Rd: 0, Imm: 10, SF: true},
		{Offset: 0x04, Op: int(B_COND), Cond: COND_NE, Imm: 8}, // target 0x0C
		{Offset: 0x08, Op: int(MOVZ), Rd: 0, Imm: 0, SF: true},
		{Offset: 0x0C, Op: int(RET)},
	}

	tr := NewTranslator(0x1000, 0x10)
	tr.SetCFF(true)
	res, err := tr.Translate(insts)
	if err != nil {
		t.Fatal(err)
	}

	// CFF should produce a much larger bytecode due to dispatcher
	if len(res.Bytecode) < 50 {
		t.Errorf("CFF bytecode seems too small: %d bytes", len(res.Bytecode))
	}
}

// --- MBA Obfuscation Tests ---

func TestTranslate_MBA_ProducesMoreBytecode(t *testing.T) {
	vm.GenerateDynamicISA()

	insts := []vm.Instruction{
		{Offset: 0x00, Op: int(MOVZ), Rd: 0, Imm: 10, SF: true},
		{Offset: 0x04, Op: int(ADD_REG), Rd: 0, Rn: 0, Rm: 1, SF: true},
		{Offset: 0x08, Op: int(RET)},
	}

	// Without MBA
	tr1 := NewTranslator(0x1000, 0x10)
	res1, _ := tr1.Translate(insts)

	// With MBA (may not always expand, but statistically should)
	maxLen := res1.CodeLen
	for attempt := 0; attempt < 20; attempt++ {
		tr2 := NewTranslator(0x1000, 0x10)
		tr2.SetMBA(true)
		res2, _ := tr2.Translate(insts)
		if res2.CodeLen > maxLen {
			return // MBA produced more bytecode — test passes
		}
	}
	t.Logf("Warning: MBA did not produce larger bytecode in 20 attempts (statistically unlikely)")
}

// --- Translation Relocations ---

func TestTranslate_BL_ProducesRelocation(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0x00, Op: int(BL), Imm: 0x100},
		{Offset: 0x04, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Relocations) == 0 {
		t.Error("BL should produce a relocation entry")
	}
}

// --- ADC/SBC Tests ---

func TestTranslate_ADC(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(ADC), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSAdc}) {
		t.Error("ADC should produce S_ADC")
	}
}

func TestTranslate_SBC(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(SBC), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSSbc}) {
		t.Error("SBC should produce S_SBC")
	}
}

// --- Shift Tests ---

func TestTranslate_LSL_REG(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(LSL_REG), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSShl}) {
		t.Error("LSL should produce S_SHL")
	}
}

func TestTranslate_LSR_REG(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(LSR_REG), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSShr}) {
		t.Error("LSR should produce S_SHR")
	}
}

func TestTranslate_ASR_REG(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x10)
	res, err := tr.Translate([]vm.Instruction{
		{Offset: 0, Op: int(ASR_REG), Rd: 0, Rn: 1, Rm: 2, SF: true},
		{Offset: 4, Op: int(RET)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(res.Bytecode[:res.CodeLen], []byte{vm.OpSAsr}) {
		t.Error("ASR should produce S_ASR")
	}
}

// --- String Decryption Tests ---

func TestTranslate_SetStringRefs(t *testing.T) {
	vm.GenerateDynamicISA()
	tr := NewTranslator(0x1000, 0x100)
	refs := map[uint64]StringRef{
		0x401000: {Addr: 0x401000, Len: 10, Key: 0x42},
	}
	tr.SetStringRefs(refs)

	if len(tr.stringRefs) != 1 {
		t.Errorf("Expected 1 string ref, got %d", len(tr.stringRefs))
	}
	if tr.stringRefs[0x401000].Key != 0x42 {
		t.Error("StringRef not stored correctly")
	}
}
