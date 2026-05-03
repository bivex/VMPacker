package arm64

import "github.com/vmpacker/pkg/vm"

// ============================================================
// FP / SIMD pattern table
// ============================================================

var fpuPatterns = []InstrPattern{
	// ---- FP Data Processing (2-source) ----
	// Encoding: 0:0:S:11110:type:1:Rm:0010:opcode:Rn:Rd
	{
		Name: "FADD", Mask: 0x5F20FC00, Value: 0x1E202800, Op: FADD,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRm16, fRn, fRd},
		Post:   postFP3,
	},
	{
		Name: "FSUB", Mask: 0x5F20FC00, Value: 0x1E203800, Op: FSUB,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRm16, fRn, fRd},
		Post:   postFP3,
	},
	{
		Name: "FMUL", Mask: 0x5F20FC00, Value: 0x1E200800, Op: FMUL,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRm16, fRn, fRd},
		Post:   postFP3,
	},
	{
		Name: "FDIV", Mask: 0x5F20FC00, Value: 0x1E201800, Op: FDIV,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRm16, fRn, fRd},
		Post:   postFP3,
	},
	{
		Name: "FMAX", Mask: 0x5F20FC00, Value: 0x1E204800, Op: FMAX,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRm16, fRn, fRd},
		Post:   postFP3,
	},
	{
		Name: "FMIN", Mask: 0x5F20FC00, Value: 0x1E205800, Op: FMIN,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRm16, fRn, fRd},
		Post:   postFP3,
	},

	// ---- FP Data Processing (1-source) ----
	// Encoding: 0:0:S:11110:type:1:00000:opcode:Rn:Rd
	// bits[20:16] = 00000
	{
		Name: "FMOV", Mask: 0x5F3F0C00, Value: 0x1E204000, Op: FMOV,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post:   postFP2,
	},
	{
		Name: "FABS", Mask: 0x5F3F0C00, Value: 0x1E202000, Op: FABS,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post:   postFP2,
	},
	{
		Name: "FNEG", Mask: 0x5F3F0C00, Value: 0x1E201000, Op: FNEG,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post:   postFP2,
	},
	{
		Name: "FSQRT", Mask: 0x5F3F0C00, Value: 0x1E206000, Op: FSQRT,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post:   postFP2,
	},
	{
		Name: "FCVT", Mask: 0x5F3F0C00, Value: 0x1E220000, Op: FCVT,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post:   postFP2,
	},

	// ---- FP <-> Integer conversion (scalar) ----
	// Encoding: sf:0:0:11110:type:1:opcode:Rn:Rd
	// bits[20:16] varies
	{
		Name: "FCVTZS_GENERIC", Mask: 0x5F200C00, Value: 0x1E380000, Op: FCVTZS,
		Fields: []FieldDef{fSF, {Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn += vm.REG_V_BASE
		},
	},
	{
		Name: "FCVTZU_GENERIC", Mask: 0x5F200C00, Value: 0x1E390000, Op: FCVTZU,
		Fields: []FieldDef{fSF, {Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn += vm.REG_V_BASE
		},
	},

	// ---- FP Compare ----
	{
		Name: "FCMP", Mask: 0x5F20FC1F, Value: 0x1E202000, Op: FCMP,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRm16, fRn},
		Post:   postFPComp,
	},
	{
		Name: "FCMP_ZERO", Mask: 0x5F20FC1F, Value: 0x1E202008, Op: FCMP,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRn},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn += vm.REG_V_BASE
			inst.Rm = vm.REG_XZR
			inst.SF = (f["type"] == 1)
		},
	},

	// ---- SIMD Copy (DUP) ----
	{
		Name: "DUP_SCALAR", Mask: 0xFFE0F000, Value: 0x5EE00000, Op: FMOV,
		Fields: []FieldDef{fRn, fRd},
		Post:   postFP2,
	},

	// ---- Specific patterns for Clang variants ----
	{
		Name: "FNEG_VAR", Mask: 0x5FFFFC00, Value: 0x1E6F1000, Op: FNEG,
		Fields: []FieldDef{fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rd += vm.REG_V_BASE
			inst.Rn += vm.REG_V_BASE
			inst.SF = true
		},
	},
	{
		Name: "FCVTZU_VAR", Mask: 0x5FFFFC00, Value: 0x1E609000, Op: FCVTZU,
		Fields: []FieldDef{fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn += vm.REG_V_BASE
			inst.SF = true
		},
	},
	{
		Name: "FCMP_VAR", Mask: 0x5FFFFC00, Value: 0x1E7E0C00, Op: FCMP,
		Fields: []FieldDef{fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn += vm.REG_V_BASE
			inst.Rm = vm.REG_XZR
			inst.SF = true
		},
	},
	{
		Name: "FCVT_VAR", Mask: 0x5FFFFC00, Value: 0x1E639000, Op: FCVT,
		Fields: []FieldDef{fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rd += vm.REG_V_BASE
			inst.Rn += vm.REG_V_BASE
			inst.SF = true
		},
	},
	{
		Name: "FCVTZS_VAR2", Mask: 0x5FFFFC00, Value: 0x1E619000, Op: FCVTZS,
		Fields: []FieldDef{fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn += vm.REG_V_BASE
			inst.SF = true
		},
	},
	{
		Name: "DUP_VAR", Mask: 0xFFFFFFFF, Value: 0x7EE1BBFE, Op: FMOV,
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rd = 30 + vm.REG_V_BASE
			inst.Rn = 30
		},
	},
}

// postFP3 FP 3-register: Rd/Rn/Rm += REG_V_BASE, SF = (type == 1)
func postFP3(f map[string]int64, inst *vm.Instruction) {
	inst.Rd += vm.REG_V_BASE
	inst.Rn += vm.REG_V_BASE
	inst.Rm += vm.REG_V_BASE
	inst.SF = (f["type"] == 1) // 1=64-bit (double), 0=32-bit (float)
}

// postFP2 FP 2-register
func postFP2(f map[string]int64, inst *vm.Instruction) {
	inst.Rd += vm.REG_V_BASE
	inst.Rn += vm.REG_V_BASE
	inst.SF = (f["type"] == 1)
}

// postFPComp FP Compare
func postFPComp(f map[string]int64, inst *vm.Instruction) {
	inst.Rn += vm.REG_V_BASE
	inst.Rm += vm.REG_V_BASE
	inst.SF = (f["type"] == 1)
}
