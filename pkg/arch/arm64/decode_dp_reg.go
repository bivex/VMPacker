package arm64

import "github.com/vmpacker/pkg/vm"

// ============================================================
// Data processing (register) pattern table
//
// Covers: ADD/SUB/ADDS/SUBS(reg), AND/ORR/EOR/ANDS(reg), MVN,
//       LSL/LSR/ASR/ROR(reg), MUL/MADD/MSUB, SDIV/UDIV,
//       CSEL/CSINC/CSINV/CSNEG
// ============================================================

var dpRegPatterns = []InstrPattern{
	// ---- Logical (shifted register) ----
	// Encoding: sf:opc:01010:shift:N:Rm:imm6:Rn:Rd
	// bits[28:24] = 01010 → use opc+N to distinguish within group
	{
		Name: "AND_REG", Mask: 0x7F200000, Value: 0x0A000000, Op: AND_REG,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},
	{
		// BIC = AND with NOT: Rd = Rn AND NOT(shift(Rm)), opc=00 N=1
		Name: "BIC", Mask: 0x7F200000, Value: 0x0A200000, Op: BIC,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},
	{
		Name: "ORR_REG", Mask: 0x7F200000, Value: 0x2A000000, Op: ORR_REG,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},
	{
		// MVN = ORN with Rn=11111 (XZR) — must come before ORN (more specific mask)
		Name: "MVN", Mask: 0x7F2003E0, Value: 0x2A2003E0, Op: MVN,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},
	{
		// ORN = ORR with NOT: Rd = Rn OR NOT(shift(Rm)), opc=01 N=1
		Name: "ORN", Mask: 0x7F200000, Value: 0x2A200000, Op: ORN,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},
	{
		Name: "EOR_REG", Mask: 0x7F200000, Value: 0x4A000000, Op: EOR_REG,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},
	{
		// EON = EOR(reg) with N=1 → Rd = Rn XOR NOT(shift(Rm))
		Name: "EON", Mask: 0x7F200000, Value: 0x4A200000, Op: EON,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},
	{
		Name: "ANDS_REG", Mask: 0x7F200000, Value: 0x6A000000, Op: ANDS_REG,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},
	{
		// BICS = AND with NOT + flags: Rd = Rn AND NOT(shift(Rm)), set flags, opc=11 N=1
		Name: "BICS", Mask: 0x7F200000, Value: 0x6A200000, Op: BICS,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},

	// ---- Add/Subtract (shifted register) ----
	// Encoding: sf:op:S:01011:shift:0:Rm:imm6:Rn:Rd
	// bits[28:24] = 01011
	{
		Name: "ADD_REG", Mask: 0x7F200000, Value: 0x0B000000, Op: ADD_REG,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},
	{
		Name: "ADDS_REG", Mask: 0x7F200000, Value: 0x2B000000, Op: ADDS_REG,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},
	{
		Name: "SUB_REG", Mask: 0x7F200000, Value: 0x4B000000, Op: SUB_REG,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},
	{
		Name: "SUBS_REG", Mask: 0x7F200000, Value: 0x6B000000, Op: SUBS_REG,
		Fields: []FieldDef{fSF, {Name: "shtype", Hi: 23, Lo: 22}, fRm16, {Name: "shift", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postShiftedXZR3,
	},

	// ---- Add/Subtract with carry ----
	// Encoding: sf:op:S:11010000:Rm:000000:Rn:Rd
	{
		Name: "ADC", Mask: 0x7FE0FC00, Value: 0x1A000000, Op: ADC,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
		Post:   postXZR3,
	},
	{
		Name: "ADCS", Mask: 0x7FE0FC00, Value: 0x3A000000, Op: ADCS,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
		Post:   postXZR3,
	},
	{
		Name: "SBC", Mask: 0x7FE0FC00, Value: 0x5A000000, Op: SBC,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
		Post:   postXZR3,
	},
	{
		Name: "SBCS", Mask: 0x7FE0FC00, Value: 0x7A000000, Op: SBCS,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
		Post:   postXZR3,
	},

	// ---- Conditional select ----
	// Encoding: sf:op:S:11010:00:Rm:cond:o2:Rn:Rd
	// bits[28:21] = 11010_00_0 (bit21=0 for condsel)
	{
		Name: "CSEL", Mask: 0x7FE00C00, Value: 0x1A800000, Op: CSEL,
		Fields: []FieldDef{fSF, fRm16, {Name: "cond", Hi: 15, Lo: 12}, fRn, fRd},
		Post:   postXZR3,
	},
	{
		Name: "CSINC", Mask: 0x7FE00C00, Value: 0x1A800400, Op: CSINC,
		Fields: []FieldDef{fSF, fRm16, {Name: "cond", Hi: 15, Lo: 12}, fRn, fRd},
		Post:   postXZR3,
	},
	{
		Name: "CSINV", Mask: 0x7FE00C00, Value: 0x5A800000, Op: CSINV,
		Fields: []FieldDef{fSF, fRm16, {Name: "cond", Hi: 15, Lo: 12}, fRn, fRd},
		Post:   postXZR3,
	},
	{
		Name: "CSNEG", Mask: 0x7FE00C00, Value: 0x5A800400, Op: CSNEG,
		Fields: []FieldDef{fSF, fRm16, {Name: "cond", Hi: 15, Lo: 12}, fRn, fRd},
		Post:   postXZR3,
	},

	// ---- Data processing (2-source): DIV/SHIFT ----
	// Encoding: sf:0:S:11010110:Rm:opcode:Rn:Rd
	// bits[28:21] = 11010_11_0 (bit21=1 for 2-source)
	{
		Name: "UDIV", Mask: 0x7FE0FC00, Value: 0x1AC00800, Op: UDIV,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
	},
	{
		Name: "SDIV", Mask: 0x7FE0FC00, Value: 0x1AC00C00, Op: SDIV,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
	},
	{
		Name: "LSL_REG", Mask: 0x7FE0FC00, Value: 0x1AC02000, Op: LSL_REG,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
	},
	{
		Name: "LSR_REG", Mask: 0x7FE0FC00, Value: 0x1AC02400, Op: LSR_REG,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
	},
	{
		Name: "ASR_REG", Mask: 0x7FE0FC00, Value: 0x1AC02800, Op: ASR_REG,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
	},
	{
		Name: "ROR_REG", Mask: 0x7FE0FC00, Value: 0x1AC02C00, Op: ROR_REG,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
	},

	// ---- Data processing (1-source) ----
	// Encoding: sf:1:S:11010110:00000:opcode2:Rn:Rd
	// bits[30:21] = 1_0_11010110, bit20:16 = 00000
	{
		Name: "CLZ", Mask: 0x7FFFFC00, Value: 0x5AC01000, Op: CLZ,
		Fields: []FieldDef{fSF, fRn, fRd},
	},
	{
		Name: "CLS", Mask: 0x7FFFFC00, Value: 0x5AC01400, Op: CLS,
		Fields: []FieldDef{fSF, fRn, fRd},
	},
	{
		Name: "RBIT", Mask: 0x7FFFFC00, Value: 0x5AC00000, Op: RBIT,
		Fields: []FieldDef{fSF, fRn, fRd},
	},
	{
		// REV32 (64-bit): opcode2=000010, sf=1 → 0xDAC00800
		// Must come before REV to avoid 32-bit REV matching 64-bit REV32
		Name: "REV32", Mask: 0xFFFFFC00, Value: 0xDAC00800, Op: REV32,
		Fields: []FieldDef{fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.SF = true // REV32 is 64-bit only
		},
	},
	{
		// REV: 32-bit opcode2=000010(0x5AC00800), 64-bit opcode2=000011(0xDAC00C00)
		// Use sf to distinguish: sf=0→32-bit REV, sf=1→64-bit REV
		Name: "REV_32", Mask: 0xFFFFFC00, Value: 0x5AC00800, Op: REV,
		Fields: []FieldDef{fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.SF = false
		},
	},
	{
		Name: "REV_64", Mask: 0xFFFFFC00, Value: 0xDAC00C00, Op: REV,
		Fields: []FieldDef{fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.SF = true
		},
	},
	{
		Name: "REV16", Mask: 0x7FFFFC00, Value: 0x5AC00400, Op: REV16,
		Fields: []FieldDef{fSF, fRn, fRd},
	},

	// ---- Data processing (3-source): MUL/MADD/MSUB ----
	// Encoding: sf:00:11011:000:Rm:o0:Ra:Rn:Rd
	// MUL = MADD with Ra=11111
	{
		Name: "MUL", Mask: 0x7FE0FC00, Value: 0x1B007C00, Op: MUL,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			xzrReplace(&inst.Rd)
			xzrReplace(&inst.Rn)
			xzrReplace(&inst.Rm)
		},
	},
	{
		// MADD: o0=0, Ra≠11111 → need to match o0=0 with any Ra
		// Use looser mask to match MADD first (MUL has stricter mask, matched earlier)
		Name: "MADD", Mask: 0x7FE08000, Value: 0x1B000000, Op: MADD,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			xzrReplace(&inst.Rd)
			xzrReplace(&inst.Rn)
			xzrReplace(&inst.Rm)
		},
	},
	{
		Name: "MSUB", Mask: 0x7FE08000, Value: 0x1B008000, Op: MSUB,
		Fields: []FieldDef{fSF, fRm16, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			xzrReplace(&inst.Rd)
			xzrReplace(&inst.Rn)
			xzrReplace(&inst.Rm)
		},
	},

	// ---- Data processing (3-source): SMADDL/SMSUBL ----
	// Encoding: 1:00:11011:010:Rm:o0:Ra:Rn:Rd  (sf=1 only, 32×32→64)
	// SMADDL: o0=0, Xd = Xa + SEXT(Wn)*SEXT(Wm)
	// SMULL:  o0=0, Ra=11111 (SMADDL alias)
	{
		Name: "SMADDL", Mask: 0xFFE08000, Value: 0x9B200000, Op: SMADDL,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.SF = true // always 64-bit result
			xzrReplace(&inst.Rd)
			xzrReplace(&inst.Rn)
			xzrReplace(&inst.Rm)
		},
	},
	// SMSUBL: o0=1, Xd = Xa - SEXT(Wn)*SEXT(Wm)
	// SMNEGL: Ra=11111 (SMSUBL alias)
	{
		Name: "SMSUBL", Mask: 0xFFE08000, Value: 0x9B208000, Op: SMSUBL,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.SF = true
			xzrReplace(&inst.Rd)
			xzrReplace(&inst.Rn)
			xzrReplace(&inst.Rm)
		},
	},

	// ---- Data processing (3-source): UMADDL/UMSUBL ----
	// Encoding: 1:00:11011:101:Rm:o0:Ra:Rn:Rd  (sf=1 only, 32×32→64 unsigned)
	// UMADDL: o0=0, Xd = Xa + ZEXT(Wn)*ZEXT(Wm)
	// UMULL:  o0=0, Ra=11111 (UMADDL alias)
	{
		Name: "UMADDL", Mask: 0xFFE08000, Value: 0x9BA00000, Op: UMADDL,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.SF = true // always 64-bit result
			xzrReplace(&inst.Rd)
			xzrReplace(&inst.Rn)
			xzrReplace(&inst.Rm)
		},
	},
	// UMSUBL: o0=1, Xd = Xa - ZEXT(Wn)*ZEXT(Wm)
	// UMNEGL: Ra=11111 (UMSUBL alias)
	{
		Name: "UMSUBL", Mask: 0xFFE08000, Value: 0x9BA08000, Op: UMSUBL,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.SF = true
			xzrReplace(&inst.Rd)
			xzrReplace(&inst.Rn)
			xzrReplace(&inst.Rm)
		},
	},

	// ---- Data processing (3-source): UMULH ----
	// Encoding: 1:00:11011:110:Rm:0:11111:Rn:Rd
	// sf=1 (64-bit only), op54=00, op31=110, o0=0, Ra=11111
	{
		Name: "UMULH", Mask: 0xFFE0FC00, Value: 0x9BC07C00, Op: UMULH,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.SF = true // UMULH is always 64-bit
			xzrReplace(&inst.Rd)
			xzrReplace(&inst.Rn)
			xzrReplace(&inst.Rm)
		},
	},
	// ---- Data processing (3-source): SMULH ----
	// Encoding: 1:00:11011:010:Rm:0:11111:Rn:Rd
	// sf=1 (64-bit only), op54=00, op31=010, o0=0, Ra=11111
	{
		Name: "SMULH", Mask: 0xFFE0FC00, Value: 0x9B407C00, Op: SMULH,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.SF = true // SMULH is always 64-bit
			xzrReplace(&inst.Rd)
			xzrReplace(&inst.Rn)
			xzrReplace(&inst.Rm)
		},
	},

	// ---- Add/Subtract (extended register) ----
	// Encoding: sf:op:S:01011:00:1:Rm:option:imm3:Rn:Rd
	// bits[28:24]=01011, bits[23:22]=00, bit21=1
	{
		Name: "ADD_EXT", Mask: 0x7FE00000, Value: 0x0B200000, Op: ADD_EXT,
		Fields: []FieldDef{fSF, fRm16, {Name: "option", Hi: 15, Lo: 13}, {Name: "imm3", Hi: 12, Lo: 10}, fRn, fRd},
		Post:   postExtReg,
	},
	{
		Name: "ADDS_EXT", Mask: 0x7FE00000, Value: 0x2B200000, Op: ADDS_EXT,
		Fields: []FieldDef{fSF, fRm16, {Name: "option", Hi: 15, Lo: 13}, {Name: "imm3", Hi: 12, Lo: 10}, fRn, fRd},
		Post:   postExtReg,
	},
	{
		Name: "SUB_EXT", Mask: 0x7FE00000, Value: 0x4B200000, Op: SUB_EXT,
		Fields: []FieldDef{fSF, fRm16, {Name: "option", Hi: 15, Lo: 13}, {Name: "imm3", Hi: 12, Lo: 10}, fRn, fRd},
		Post:   postExtReg,
	},
	{
		Name: "SUBS_EXT", Mask: 0x7FE00000, Value: 0x6B200000, Op: SUBS_EXT,
		Fields: []FieldDef{fSF, fRm16, {Name: "option", Hi: 15, Lo: 13}, {Name: "imm3", Hi: 12, Lo: 10}, fRn, fRd},
		Post:   postExtReg,
	},

	// ---- Conditional compare (CCMP/CCMN) ----
	// CCMP register: sf:1:1:11010010:Rm:cond:0:0:Rn:0:nzcv
	{
		Name: "CCMP_REG", Mask: 0x7FE00C10, Value: 0x7A400000, Op: CCMP_REG,
		Fields: []FieldDef{fSF, fRm16, {Name: "cond", Hi: 15, Lo: 12}, fRn, {Name: "nzcv", Hi: 3, Lo: 0}},
		Post:   postCCMP,
	},
	// CCMP immediate: sf:1:1:11010010:imm5:cond:1:0:Rn:0:nzcv
	{
		Name: "CCMP_IMM", Mask: 0x7FE00C10, Value: 0x7A400800, Op: CCMP_IMM,
		Fields: []FieldDef{fSF, {Name: "imm5", Hi: 20, Lo: 16}, {Name: "cond", Hi: 15, Lo: 12}, fRn, {Name: "nzcv", Hi: 3, Lo: 0}},
		Post:   postCCMPImm,
	},
	// CCMN register: sf:0:1:11010010:Rm:cond:0:0:Rn:0:nzcv
	{
		Name: "CCMN_REG", Mask: 0x7FE00C10, Value: 0x3A400000, Op: CCMN_REG,
		Fields: []FieldDef{fSF, fRm16, {Name: "cond", Hi: 15, Lo: 12}, fRn, {Name: "nzcv", Hi: 3, Lo: 0}},
		Post:   postCCMP,
	},
	// CCMN immediate: sf:0:1:11010010:imm5:cond:1:0:Rn:0:nzcv
	{
		Name: "CCMN_IMM", Mask: 0x7FE00C10, Value: 0x3A400800, Op: CCMN_IMM,
		Fields: []FieldDef{fSF, {Name: "imm5", Hi: 20, Lo: 16}, {Name: "cond", Hi: 15, Lo: 12}, fRn, {Name: "nzcv", Hi: 3, Lo: 0}},
		Post:   postCCMPImm,
	},

	// ---- Floating-point <-> Fixed-point/Integer conversion ----
	// FP <-> Integer conversion (general register)
	{
		Name: "SCVTF_INT", Mask: 0x7F3F0C00, Value: 0x1E220000, Op: SCVTF,
		Fields: []FieldDef{fSF, {Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rd += vm.REG_V_BASE
		},
	},
	{
		Name: "UCVTF_INT", Mask: 0x7F3F0C00, Value: 0x1E230000, Op: UCVTF,
		Fields: []FieldDef{fSF, {Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rd += vm.REG_V_BASE
		},
	},
	{
		Name: "FMOV_VR", Mask: 0x7F3F0C00, Value: 0x1E260000, Op: FMOV,
		Fields: []FieldDef{fSF, {Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn += vm.REG_V_BASE
		},
	},
	{
		Name: "FMOV_RV", Mask: 0x7F3F0C00, Value: 0x1E270000, Op: FMOV,
		Fields: []FieldDef{fSF, {Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rd += vm.REG_V_BASE
		},
	},
	// FCVTZS (scalar, integer): sf:0:0:11110:type:1:11000:000000:Rn:Rd
	{
		Name: "FCVTZS_INT", Mask: 0x7F3F0C00, Value: 0x1E380000, Op: FCVTZS,
		Fields: []FieldDef{fSF, {Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn += vm.REG_V_BASE
		},
	},
	{
		Name: "FCVTZU_INT", Mask: 0x7F3F0C00, Value: 0x1E390000, Op: FCVTZU,
		Fields: []FieldDef{fSF, {Name: "type", Hi: 23, Lo: 22}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Rn += vm.REG_V_BASE
		},
	},
}

// postXZR3 logic/arithmetic/conditional select(reg): Rd/Rn/Rm=31 → XZR
func postXZR3(f map[string]int64, inst *vm.Instruction) {
	xzrReplace(&inst.Rd)
	xzrReplace(&inst.Rn)
	xzrReplace(&inst.Rm)
}

// postShiftedXZR3 shifted register: XZR replacement + shift type save
func postShiftedXZR3(f map[string]int64, inst *vm.Instruction) {
	xzrReplace(&inst.Rd)
	xzrReplace(&inst.Rn)
	xzrReplace(&inst.Rm)
	if shtype, ok := f["shtype"]; ok {
		inst.ShiftType = int(shtype) // 0=LSL, 1=LSR, 2=ASR, 3=ROR
	}
	if shift, ok := f["shift"]; ok {
		inst.Shift = int(shift) // imm6
	}
}

// postExtReg extended register: option→ShiftType, imm3→Shift, Rn=31→SP(preserved), Rd=31→SP(preserved), Rm→XZR
func postExtReg(f map[string]int64, inst *vm.Instruction) {
	// Rd=31 in extended register is also SP (e.g. SUB SP, SP, Xm), don't replace with XZR
	xzrReplace(&inst.Rm)
	// Rn=31 in extended register is SP, don't replace with XZR
	if option, ok := f["option"]; ok {
		inst.ShiftType = int(option) // 0=UXTB..7=SXTX
	}
	if imm3, ok := f["imm3"]; ok {
		inst.Shift = int(imm3) // extra left shift amount 0-4
	}
}

// postCCMP conditional compare (register): nzcv→WB, cond→Cond, Rn/Rm→XZR
func postCCMP(f map[string]int64, inst *vm.Instruction) {
	xzrReplace(&inst.Rn)
	xzrReplace(&inst.Rm)
	if nzcv, ok := f["nzcv"]; ok {
		inst.WB = int(nzcv)
	}
}

// postCCMPImm conditional compare (immediate): nzcv→WB, cond→Cond, imm5→Rm, Rn→XZR
func postCCMPImm(f map[string]int64, inst *vm.Instruction) {
	xzrReplace(&inst.Rn)
	if nzcv, ok := f["nzcv"]; ok {
		inst.WB = int(nzcv)
	}
	if imm5, ok := f["imm5"]; ok {
		inst.Rm = int(imm5) // reuse Rm field to store imm5
	}
}
