package arm64

import "github.com/vmpacker/pkg/vm"

// ============================================================
// FP / SIMD 模式表
// ============================================================

var fpuPatterns = []InstrPattern{
	// ---- FP Data Processing (2-source) ----
	// 编码: M:0:S:11110:type:1:Rm:0010:opcode:Rn:Rd
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

	// ---- FP Data Processing (1-source) ----
	{
		Name: "FMOV", Mask: 0x5F20FC00, Value: 0x1E204000, Op: FMOV,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post:   postFP2,
	},
	{
		Name: "FABS", Mask: 0x5F20FC00, Value: 0x1E202000, Op: FABS,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post:   postFP2,
	},
	{
		Name: "FNEG", Mask: 0x5F20FC00, Value: 0x1E201000, Op: FNEG,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post:   postFP2,
	},
	{
		Name: "FSQRT", Mask: 0x5F20FC00, Value: 0x1E206000, Op: FSQRT,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post:   postFP2,
	},

	// ---- FP Compare ----
	{
		Name: "FCMP", Mask: 0x5F20FC1F, Value: 0x1E202000, Op: FCMP,
		Fields: []FieldDef{{Name: "type", Hi: 23, Lo: 22}, fRm16, fRn},
		Post:   postFPComp,
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
