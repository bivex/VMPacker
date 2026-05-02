package arm32

import "github.com/vmpacker/pkg/vm"

// ARM32 branch + SVC + unconditional instruction patterns.

// branchPatterns: op1=101 — B/BL
var branchPatterns = []InstrPattern{
	// B: cond:1010:imm24
	{
		Name: "B", Mask: 0x0F000000, Value: 0x0A000000, Op: B,
		Fields: []FieldDef{{Name: "imm24", Hi: 23, Lo: 0, Signed: true}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imm24"] * 4 // PC-relative, scaled by 4; +8 for pipeline added by translator
		},
	},
	// BL: cond:1011:imm24
	{
		Name: "BL", Mask: 0x0F000000, Value: 0x0B000000, Op: BL,
		Fields: []FieldDef{{Name: "imm24", Hi: 23, Lo: 0, Signed: true}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imm24"] * 4
		},
	},
}

// svcPatterns: op1=111 — SVC
var svcPatterns = []InstrPattern{
	// SVC: cond:1111:imm24
	{
		Name: "SVC", Mask: 0x0F000000, Value: 0x0F000000, Op: SVC,
		Fields: []FieldDef{{Name: "imm24", Hi: 23, Lo: 0}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imm24"]
		},
	},
}

// unconditionalPatterns: cond=0xF — BLX(imm), barriers, etc.
var unconditionalPatterns = []InstrPattern{
	// BLX(imm): 1111:101:H:imm24
	{
		Name: "BLX_IMM", Mask: 0xFE000000, Value: 0xFA000000, Op: BL,
		Fields: []FieldDef{{Name: "imm24", Hi: 23, Lo: 0, Signed: true}, {Name: "H", Hi: 24, Lo: 24}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imm24"]*4 + f["H"]*2
			inst.Cond = COND_AL
		},
	},
	// DMB: 1111:0101:0111:1111:1111:0000:0101:option
	{
		Name: "DMB", Mask: 0xFFFFFFF0, Value: 0xF57FF050, Op: DMB,
		Fields: []FieldDef{{Name: "option", Hi: 3, Lo: 0}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Cond = COND_AL
		},
	},
	// DSB: 1111:0101:0111:1111:1111:0000:0100:option
	{
		Name: "DSB", Mask: 0xFFFFFFF0, Value: 0xF57FF040, Op: DSB,
		Fields: []FieldDef{{Name: "option", Hi: 3, Lo: 0}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Cond = COND_AL
		},
	},
	// ISB: 1111:0101:0111:1111:1111:0000:0110:option
	{
		Name: "ISB", Mask: 0xFFFFFFF0, Value: 0xF57FF060, Op: ISB,
		Fields: []FieldDef{{Name: "option", Hi: 3, Lo: 0}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Cond = COND_AL
		},
	},
}
