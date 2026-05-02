package arm32

import "github.com/vmpacker/pkg/vm"

// ARM32 data processing instruction patterns.
//
// Encoding (immediate): cond:001:opcode:S:Rn:Rd:rot:imm8
// Encoding (register):  cond:000:opcode:S:Rn:Rd:shamt:shtype:0:Rm
// Encoding (reg-shift): cond:000:opcode:S:Rn:Rd:Rs:0:shtype:1:Rm
//
// Mask layout for immediate: bits[27:26]=00, bit[25]=1
// Mask layout for register:  bits[27:26]=00, bit[25]=0, bit[4]=0 (imm shift)
//                            bits[27:26]=00, bit[25]=0, bit[4]=1, bit[7]=0 (reg shift)

// Field definitions for DP immediate
var dpImmFields = []FieldDef{
	fRd, fRn,
	{Name: "rot", Hi: 11, Lo: 8},
	{Name: "imm8", Hi: 7, Lo: 0},
}

// Field definitions for DP register (immediate shift)
var dpRegImmShiftFields = []FieldDef{
	fRd, fRn, fRm,
	{Name: "shamt", Hi: 11, Lo: 7},
	{Name: "shtype", Hi: 6, Lo: 5},
	{Name: "bit4", Hi: 4, Lo: 4},
}

// Field definitions for DP register (register shift)
var dpRegRegShiftFields = []FieldDef{
	fRd, fRn, fRm, fRs,
	{Name: "shtype", Hi: 6, Lo: 5},
	{Name: "bit4", Hi: 4, Lo: 4},
}

// dpImmPatterns: data processing (immediate) — op1=001
var dpImmPatterns = []InstrPattern{
	// MOVW Rd, #imm16 (ARMv7): cond:0011:0000:imm4:Rd:imm12
	{
		Name: "MOVW", Mask: 0x0FF00000, Value: 0x03000000, Op: MOVW,
		Fields: []FieldDef{fRd, {Name: "imm4", Hi: 19, Lo: 16}, {Name: "imm12", Hi: 11, Lo: 0}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = (f["imm4"] << 12) | f["imm12"]
		},
	},
	// MOVT Rd, #imm16 (ARMv7): cond:0011:0100:imm4:Rd:imm12
	{
		Name: "MOVT", Mask: 0x0FF00000, Value: 0x03400000, Op: MOVT,
		Fields: []FieldDef{fRd, {Name: "imm4", Hi: 19, Lo: 16}, {Name: "imm12", Hi: 11, Lo: 0}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = (f["imm4"] << 12) | f["imm12"]
		},
	},

	// AND(imm): cond:001:0000:S:Rn:Rd:rot:imm8
	{Name: "AND_IMM", Mask: 0x0FE00000, Value: 0x02000000, Op: AND_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "ANDS_IMM", Mask: 0x0FE00000, Value: 0x02100000, Op: ANDS_IMM, Fields: dpImmFields, Post: postDPImm},

	// EOR(imm): cond:001:0001:S:Rn:Rd:rot:imm8
	{Name: "EOR_IMM", Mask: 0x0FE00000, Value: 0x02200000, Op: EOR_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "EORS_IMM", Mask: 0x0FE00000, Value: 0x02300000, Op: EORS_IMM, Fields: dpImmFields, Post: postDPImm},

	// SUB(imm): cond:001:0010:S:Rn:Rd:rot:imm8
	{Name: "SUB_IMM", Mask: 0x0FE00000, Value: 0x02400000, Op: SUB_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "SUBS_IMM", Mask: 0x0FE00000, Value: 0x02500000, Op: SUBS_IMM, Fields: dpImmFields, Post: postDPImm},

	// RSB(imm): cond:001:0011:S:Rn:Rd:rot:imm8
	{Name: "RSB_IMM", Mask: 0x0FE00000, Value: 0x02600000, Op: RSB_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "RSBS_IMM", Mask: 0x0FE00000, Value: 0x02700000, Op: RSBS_IMM, Fields: dpImmFields, Post: postDPImm},

	// ADD(imm): cond:001:0100:S:Rn:Rd:rot:imm8
	{Name: "ADD_IMM", Mask: 0x0FE00000, Value: 0x02800000, Op: ADD_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "ADDS_IMM", Mask: 0x0FE00000, Value: 0x02900000, Op: ADDS_IMM, Fields: dpImmFields, Post: postDPImm},

	// ADC(imm): cond:001:0101:S:Rn:Rd:rot:imm8
	{Name: "ADC_IMM", Mask: 0x0FE00000, Value: 0x02A00000, Op: ADC_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "ADCS_IMM", Mask: 0x0FE00000, Value: 0x02B00000, Op: ADCS_IMM, Fields: dpImmFields, Post: postDPImm},

	// SBC(imm): cond:001:0110:S:Rn:Rd:rot:imm8
	{Name: "SBC_IMM", Mask: 0x0FE00000, Value: 0x02C00000, Op: SBC_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "SBCS_IMM", Mask: 0x0FE00000, Value: 0x02D00000, Op: SBCS_IMM, Fields: dpImmFields, Post: postDPImm},

	// RSC(imm): cond:001:0111:S:Rn:Rd:rot:imm8
	{Name: "RSC_IMM", Mask: 0x0FE00000, Value: 0x02E00000, Op: RSC_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "RSCS_IMM", Mask: 0x0FE00000, Value: 0x02F00000, Op: RSCS_IMM, Fields: dpImmFields, Post: postDPImm},

	// TST(imm): cond:001:10001:Rn:0000:rot:imm8 (S=1, Rd=0)
	{Name: "TST_IMM", Mask: 0x0FF0F000, Value: 0x03100000, Op: TST_IMM, Fields: dpImmFields, Post: postDPImm},
	// TEQ(imm): cond:001:10011:Rn:0000:rot:imm8
	{Name: "TEQ_IMM", Mask: 0x0FF0F000, Value: 0x03300000, Op: TEQ_IMM, Fields: dpImmFields, Post: postDPImm},
	// CMP(imm): cond:001:10101:Rn:0000:rot:imm8
	{Name: "CMP_IMM", Mask: 0x0FF0F000, Value: 0x03500000, Op: CMP_IMM, Fields: dpImmFields, Post: postDPImm},
	// CMN(imm): cond:001:10111:Rn:0000:rot:imm8
	{Name: "CMN_IMM", Mask: 0x0FF0F000, Value: 0x03700000, Op: CMN_IMM, Fields: dpImmFields, Post: postDPImm},

	// ORR(imm): cond:001:1100:S:Rn:Rd:rot:imm8
	{Name: "ORR_IMM", Mask: 0x0FE00000, Value: 0x03800000, Op: ORR_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "ORRS_IMM", Mask: 0x0FE00000, Value: 0x03900000, Op: ORRS_IMM, Fields: dpImmFields, Post: postDPImm},

	// MOV(imm): cond:001:1101:S:0000:Rd:rot:imm8
	{Name: "MOV_IMM", Mask: 0x0FEF0000, Value: 0x03A00000, Op: MOV_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "MOVS_IMM", Mask: 0x0FEF0000, Value: 0x03B00000, Op: MOVS_IMM, Fields: dpImmFields, Post: postDPImm},

	// BIC(imm): cond:001:1110:S:Rn:Rd:rot:imm8
	{Name: "BIC_IMM", Mask: 0x0FE00000, Value: 0x03C00000, Op: BIC_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "BICS_IMM", Mask: 0x0FE00000, Value: 0x03D00000, Op: BICS_IMM, Fields: dpImmFields, Post: postDPImm},

	// MVN(imm): cond:001:1111:S:0000:Rd:rot:imm8
	{Name: "MVN_IMM", Mask: 0x0FEF0000, Value: 0x03E00000, Op: MVN_IMM, Fields: dpImmFields, Post: postDPImm},
	{Name: "MVNS_IMM", Mask: 0x0FEF0000, Value: 0x03F00000, Op: MVNS_IMM, Fields: dpImmFields, Post: postDPImm},
}

// dpRegPatterns: data processing (register) + multiply + misc — op1=000
var dpRegPatterns = []InstrPattern{
	// ---- Multiply / Divide (must be before general DP reg to avoid mis-match) ----
	// MUL: cond:0000:000S:Rd:0000:Rm:1001:Rn (note: Rd at [19:16], Rn at [3:0], Rm at [11:8])
	{
		Name: "MUL", Mask: 0x0FE000F0, Value: 0x00000090, Op: MUL,
		Fields: []FieldDef{
			{Name: "Rd", Hi: 19, Lo: 16},
			{Name: "Rs", Hi: 11, Lo: 8},
			fRm,
		},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn = int(f["Rs"]) // MUL Rd, Rm, Rs
		},
	},
	// MLA: cond:0000:001S:Rd:Ra:Rm:1001:Rn
	{
		Name: "MLA", Mask: 0x0FE000F0, Value: 0x00200090, Op: MLA,
		Fields: []FieldDef{
			{Name: "Rd", Hi: 19, Lo: 16},
			{Name: "Ra", Hi: 15, Lo: 12},
			{Name: "Rs", Hi: 11, Lo: 8},
			fRm,
		},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn = int(f["Rs"])
			inst.Imm = f["Ra"] // accumulator register
		},
	},
	// UMULL: cond:0000:100S:RdHi:RdLo:Rm:1001:Rn
	{
		Name: "UMULL", Mask: 0x0FE000F0, Value: 0x00800090, Op: UMULL,
		Fields: []FieldDef{fRdHi, fRdLo, fRs, fRm},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rd = int(f["RdLo"])
			inst.Rn = int(f["Rs"])
			inst.Imm = f["RdHi"]
		},
	},
	// SMULL: cond:0000:110S:RdHi:RdLo:Rm:1001:Rn
	{
		Name: "SMULL", Mask: 0x0FE000F0, Value: 0x00C00090, Op: SMULL,
		Fields: []FieldDef{fRdHi, fRdLo, fRs, fRm},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rd = int(f["RdLo"])
			inst.Rn = int(f["Rs"])
			inst.Imm = f["RdHi"]
		},
	},
	// UMLAL: cond:0000:101S:RdHi:RdLo:Rm:1001:Rn
	{
		Name: "UMLAL", Mask: 0x0FE000F0, Value: 0x00A00090, Op: UMLAL,
		Fields: []FieldDef{fRdHi, fRdLo, fRs, fRm},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rd = int(f["RdLo"])
			inst.Rn = int(f["Rs"])
			inst.Imm = f["RdHi"]
		},
	},
	// SMLAL: cond:0000:111S:RdHi:RdLo:Rm:1001:Rn
	{
		Name: "SMLAL", Mask: 0x0FE000F0, Value: 0x00E00090, Op: SMLAL,
		Fields: []FieldDef{fRdHi, fRdLo, fRs, fRm},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rd = int(f["RdLo"])
			inst.Rn = int(f["Rs"])
			inst.Imm = f["RdHi"]
		},
	},
	// SDIV: cond:0111:0001:Rd:1111:Rm:0001:Rn
	{
		Name: "SDIV", Mask: 0x0FF000F0, Value: 0x07100010, Op: SDIV,
		Fields: []FieldDef{{Name: "Rd", Hi: 19, Lo: 16}, fRm, {Name: "Rn_lo", Hi: 3, Lo: 0}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn = int(f["Rn_lo"])
		},
	},
	// UDIV: cond:0111:0011:Rd:1111:Rm:0001:Rn
	{
		Name: "UDIV", Mask: 0x0FF000F0, Value: 0x07300010, Op: UDIV,
		Fields: []FieldDef{{Name: "Rd", Hi: 19, Lo: 16}, fRm, {Name: "Rn_lo", Hi: 3, Lo: 0}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn = int(f["Rn_lo"])
		},
	},

	// ---- Misc instructions ----
	// BX: cond:0001:0010:1111:1111:1111:0001:Rm
	{
		Name: "BX", Mask: 0x0FFFFFF0, Value: 0x012FFF10, Op: BX,
		Fields: []FieldDef{fRm},
	},
	// BLX(reg): cond:0001:0010:1111:1111:1111:0011:Rm
	{
		Name: "BLX_REG", Mask: 0x0FFFFFF0, Value: 0x012FFF30, Op: BLX_REG,
		Fields: []FieldDef{fRm},
	},
	// CLZ: cond:0001:0110:1111:Rd:1111:0001:Rm
	{
		Name: "CLZ", Mask: 0x0FFF0FF0, Value: 0x016F0F10, Op: CLZ,
		Fields: []FieldDef{fRd, fRm},
	},
	// RBIT: cond:0110:1111:1111:Rd:1111:0011:Rm
	{
		Name: "RBIT", Mask: 0x0FFF0FF0, Value: 0x06FF0F30, Op: RBIT,
		Fields: []FieldDef{fRd, fRm},
	},
	// REV: cond:0110:1011:1111:Rd:1111:0011:Rm
	{
		Name: "REV", Mask: 0x0FFF0FF0, Value: 0x06BF0F30, Op: REV,
		Fields: []FieldDef{fRd, fRm},
	},
	// REV16: cond:0110:1011:1111:Rd:1111:1011:Rm
	{
		Name: "REV16", Mask: 0x0FFF0FF0, Value: 0x06BF0FB0, Op: REV16,
		Fields: []FieldDef{fRd, fRm},
	},
	// MRS: cond:0001:0:R:00:1111:Rd:0000:0000:0000
	{
		Name: "MRS", Mask: 0x0FBF0FFF, Value: 0x010F0000, Op: MRS,
		Fields: []FieldDef{fRd, {Name: "R", Hi: 22, Lo: 22}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["R"] // 0=CPSR, 1=SPSR
		},
	},
	// MSR (register): cond:0001:0:R:10:mask:1111:0000:0000:Rm
	{
		Name: "MSR", Mask: 0x0FB0FFF0, Value: 0x0120F000, Op: MSR,
		Fields: []FieldDef{fRm, {Name: "R", Hi: 22, Lo: 22}, {Name: "mask", Hi: 19, Lo: 16}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = (f["R"] << 4) | f["mask"]
		},
	},
	// BKPT: cond:0001:0010:imm12:0111:imm4
	{
		Name: "BKPT", Mask: 0x0FF000F0, Value: 0x01200070, Op: BKPT,
		Fields: []FieldDef{{Name: "imm12", Hi: 19, Lo: 8}, {Name: "imm4", Hi: 3, Lo: 0}},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = (f["imm12"] << 4) | f["imm4"]
		},
	},

	// ---- Data processing (register, immediate shift) ----
	// AND(reg): cond:000:0000:S:Rn:Rd:shamt:shtype:0:Rm
	{Name: "AND_REG", Mask: 0x0FE00010, Value: 0x00000000, Op: AND_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "ANDS_REG", Mask: 0x0FE00010, Value: 0x00100000, Op: ANDS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "EOR_REG", Mask: 0x0FE00010, Value: 0x00200000, Op: EOR_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "EORS_REG", Mask: 0x0FE00010, Value: 0x00300000, Op: EORS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "SUB_REG", Mask: 0x0FE00010, Value: 0x00400000, Op: SUB_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "SUBS_REG", Mask: 0x0FE00010, Value: 0x00500000, Op: SUBS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "RSB_REG", Mask: 0x0FE00010, Value: 0x00600000, Op: RSB_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "RSBS_REG", Mask: 0x0FE00010, Value: 0x00700000, Op: RSBS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "ADD_REG", Mask: 0x0FE00010, Value: 0x00800000, Op: ADD_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "ADDS_REG", Mask: 0x0FE00010, Value: 0x00900000, Op: ADDS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "ADC_REG", Mask: 0x0FE00010, Value: 0x00A00000, Op: ADC_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "ADCS_REG", Mask: 0x0FE00010, Value: 0x00B00000, Op: ADCS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "SBC_REG", Mask: 0x0FE00010, Value: 0x00C00000, Op: SBC_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "SBCS_REG", Mask: 0x0FE00010, Value: 0x00D00000, Op: SBCS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "RSC_REG", Mask: 0x0FE00010, Value: 0x00E00000, Op: RSC_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "RSCS_REG", Mask: 0x0FE00010, Value: 0x00F00000, Op: RSCS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},

	// TST/TEQ/CMP/CMN (register)
	{Name: "TST_REG", Mask: 0x0FF0F010, Value: 0x01100000, Op: TST_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "TEQ_REG", Mask: 0x0FF0F010, Value: 0x01300000, Op: TEQ_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "CMP_REG", Mask: 0x0FF0F010, Value: 0x01500000, Op: CMP_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "CMN_REG", Mask: 0x0FF0F010, Value: 0x01700000, Op: CMN_REG, Fields: dpRegImmShiftFields, Post: postDPReg},

	{Name: "ORR_REG", Mask: 0x0FE00010, Value: 0x01800000, Op: ORR_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "ORRS_REG", Mask: 0x0FE00010, Value: 0x01900000, Op: ORRS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},

	// MOV(reg): ORR with Rn=0 — cond:000:1101:S:0000:Rd:shamt:shtype:0:Rm
	{Name: "MOV_REG", Mask: 0x0FEF0010, Value: 0x01A00000, Op: MOV_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "MOVS_REG", Mask: 0x0FEF0010, Value: 0x01B00000, Op: MOVS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},

	{Name: "BIC_REG", Mask: 0x0FE00010, Value: 0x01C00000, Op: BIC_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "BICS_REG", Mask: 0x0FE00010, Value: 0x01D00000, Op: BICS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "MVN_REG", Mask: 0x0FEF0010, Value: 0x01E00000, Op: MVN_REG, Fields: dpRegImmShiftFields, Post: postDPReg},
	{Name: "MVNS_REG", Mask: 0x0FEF0010, Value: 0x01F00000, Op: MVNS_REG, Fields: dpRegImmShiftFields, Post: postDPReg},

	// ---- Data processing (register, register shift) ----
	// Same opcodes but bit[4]=1, bit[7]=0: cond:000:opcode:S:Rn:Rd:Rs:0:shtype:1:Rm
	// Mask 0x0FE00090 checks opcode+S, bit[7]=0, bit[4]=1
	{Name: "AND_REG_RS", Mask: 0x0FE00090, Value: 0x00000010, Op: AND_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "ANDS_REG_RS", Mask: 0x0FE00090, Value: 0x00100010, Op: ANDS_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "EOR_REG_RS", Mask: 0x0FE00090, Value: 0x00200010, Op: EOR_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "EORS_REG_RS", Mask: 0x0FE00090, Value: 0x00300010, Op: EORS_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "SUB_REG_RS", Mask: 0x0FE00090, Value: 0x00400010, Op: SUB_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "SUBS_REG_RS", Mask: 0x0FE00090, Value: 0x00500010, Op: SUBS_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "ADD_REG_RS", Mask: 0x0FE00090, Value: 0x00800010, Op: ADD_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "ADDS_REG_RS", Mask: 0x0FE00090, Value: 0x00900010, Op: ADDS_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "ADC_REG_RS", Mask: 0x0FE00090, Value: 0x00A00010, Op: ADC_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "ORR_REG_RS", Mask: 0x0FE00090, Value: 0x01800010, Op: ORR_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "MOV_REG_RS", Mask: 0x0FEF0090, Value: 0x01A00010, Op: MOV_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "MOVS_REG_RS", Mask: 0x0FEF0090, Value: 0x01B00010, Op: MOVS_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "BIC_REG_RS", Mask: 0x0FE00090, Value: 0x01C00010, Op: BIC_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
	{Name: "MVN_REG_RS", Mask: 0x0FEF0090, Value: 0x01E00010, Op: MVN_REG, Fields: dpRegRegShiftFields, Post: postDPRegShift},
}

// mediaPatterns: ARM media instructions (op1=011, bit[4]=1)
// UXTB, UXTH, SXTB, SXTH, BFI, UBFX, etc.
var mediaPatterns = []InstrPattern{
	// SXTH: cond:0110:1011:1111:Rd:rot:0000:0111:Rm
	{
		Name: "SXTH", Mask: 0x0FFF03F0, Value: 0x06BF0070, Op: MOV_REG,
		Fields: []FieldDef{fRd, fRm},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = 16 // SXTH flag
		},
	},
	// SXTB: cond:0110:1010:1111:Rd:rot:0000:0111:Rm
	{
		Name: "SXTB", Mask: 0x0FFF03F0, Value: 0x06AF0070, Op: MOV_REG,
		Fields: []FieldDef{fRd, fRm},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = 8 // SXTB flag
		},
	},
	// UXTH: cond:0110:1111:1111:Rd:rot:0000:0111:Rm
	{
		Name: "UXTH", Mask: 0x0FFF03F0, Value: 0x06FF0070, Op: MOV_REG,
		Fields: []FieldDef{fRd, fRm},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = -16 // UXTH flag
		},
	},
	// UXTB: cond:0110:1110:1111:Rd:rot:0000:0111:Rm
	{
		Name: "UXTB", Mask: 0x0FFF03F0, Value: 0x06EF0070, Op: MOV_REG,
		Fields: []FieldDef{fRd, fRm},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = -8 // UXTB flag
		},
	},
}
