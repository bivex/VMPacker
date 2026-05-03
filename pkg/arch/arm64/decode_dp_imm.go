package arm64

import "github.com/vmpacker/pkg/vm"

// ============================================================
// Data processing (immediate) pattern table
//
// Covers: ADD/SUB/ADDS/SUBS(imm), AND/ORR/EOR(imm),
//       MOVZ/MOVK/MOVN, UBFM/SBFM, EXTR, ADR/ADRP
// ============================================================

// Common bit fields: registers
var (
	fSF   = FieldDef{Name: "sf", Hi: 31, Lo: 31}
	fRd   = FieldDef{Name: "Rd", Hi: 4, Lo: 0}
	fRn   = FieldDef{Name: "Rn", Hi: 9, Lo: 5}
	fRm16 = FieldDef{Name: "Rm", Hi: 20, Lo: 16} // Rm at [20:16]
)

var dpImmPatterns = []InstrPattern{
	// ---- Add/Subtract (immediate) ----
	// Encoding: sf:op:S:10001:sh:imm12:Rn:Rd
	{
		Name: "ADD_IMM", Mask: 0x7F000000, Value: 0x11000000, Op: ADD_IMM,
		Fields: []FieldDef{fSF, {Name: "sh", Hi: 22, Lo: 22}, {Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imm12"]
			if f["sh"] == 1 {
				inst.Imm <<= 12
			}
		},
	},
	{
		Name: "ADDS_IMM", Mask: 0x7F000000, Value: 0x31000000, Op: ADDS_IMM,
		Fields: []FieldDef{fSF, {Name: "sh", Hi: 22, Lo: 22}, {Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imm12"]
			if f["sh"] == 1 {
				inst.Imm <<= 12
			}
			xzrReplace(&inst.Rd) // Rd=31 → XZR (CMN alias)
		},
	},
	{
		Name: "SUB_IMM", Mask: 0x7F000000, Value: 0x51000000, Op: SUB_IMM,
		Fields: []FieldDef{fSF, {Name: "sh", Hi: 22, Lo: 22}, {Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imm12"]
			if f["sh"] == 1 {
				inst.Imm <<= 12
			}
		},
	},
	{
		Name: "SUBS_IMM", Mask: 0x7F000000, Value: 0x71000000, Op: SUBS_IMM,
		Fields: []FieldDef{fSF, {Name: "sh", Hi: 22, Lo: 22}, {Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imm12"]
			if f["sh"] == 1 {
				inst.Imm <<= 12
			}
			xzrReplace(&inst.Rd) // Rd=31 → XZR (CMP alias)
		},
	},

	// ---- Logical (immediate) ----
	// Encoding: sf:opc:100100:N:immr:imms:Rn:Rd
	{
		Name: "AND_IMM", Mask: 0x7F800000, Value: 0x12000000, Op: AND_IMM,
		Fields: []FieldDef{fSF, {Name: "N", Hi: 22, Lo: 22}, {Name: "immr", Hi: 21, Lo: 16}, {Name: "imms", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postBitmaskImm,
	},
	{
		Name: "ORR_IMM", Mask: 0x7F800000, Value: 0x32000000, Op: ORR_IMM,
		Fields: []FieldDef{fSF, {Name: "N", Hi: 22, Lo: 22}, {Name: "immr", Hi: 21, Lo: 16}, {Name: "imms", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postBitmaskImm,
	},
	{
		Name: "EOR_IMM", Mask: 0x7F800000, Value: 0x52000000, Op: EOR_IMM,
		Fields: []FieldDef{fSF, {Name: "N", Hi: 22, Lo: 22}, {Name: "immr", Hi: 21, Lo: 16}, {Name: "imms", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postBitmaskImm,
	},
	{
		// ANDS(imm) opc=11 → needs to set flags (TST = ANDS XZR, Xn, #imm)
		Name: "ANDS_IMM", Mask: 0x7F800000, Value: 0x72000000, Op: ANDS_IMM,
		Fields: []FieldDef{fSF, {Name: "N", Hi: 22, Lo: 22}, {Name: "immr", Hi: 21, Lo: 16}, {Name: "imms", Hi: 15, Lo: 10}, fRn, fRd},
		Post:   postBitmaskImmANDS,
	},

	// ---- Move wide (immediate) ----
	// Encoding: sf:opc:100101:hw:imm16:Rd
	{
		Name: "MOVN", Mask: 0x7F800000, Value: 0x12800000, Op: MOVN,
		Fields: []FieldDef{fSF, {Name: "hw", Hi: 22, Lo: 21}, {Name: "imm16", Hi: 20, Lo: 5}, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imm16"]
			inst.Shift = int(f["hw"] * 16)
		},
	},
	{
		Name: "MOVZ", Mask: 0x7F800000, Value: 0x52800000, Op: MOVZ,
		Fields: []FieldDef{fSF, {Name: "hw", Hi: 22, Lo: 21}, {Name: "imm16", Hi: 20, Lo: 5}, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imm16"]
			inst.Shift = int(f["hw"] * 16)
		},
	},
	{
		Name: "MOVK", Mask: 0x7F800000, Value: 0x72800000, Op: MOVK,
		Fields: []FieldDef{fSF, {Name: "hw", Hi: 22, Lo: 21}, {Name: "imm16", Hi: 20, Lo: 5}, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imm16"]
			inst.Shift = int(f["hw"] * 16)
		},
	},

	// ---- Bitfield (SBFM/UBFM) ----
	// Encoding: sf:opc:100110:N:immr:imms:Rn:Rd
	{
		Name: "SBFM", Mask: 0x7F800000, Value: 0x13000000, Op: SBFM,
		Fields: []FieldDef{fSF, {Name: "immr", Hi: 21, Lo: 16}, {Name: "imms", Hi: 15, Lo: 10}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["immr"]
			inst.Shift = int(f["imms"])
		},
	},
	{
		Name: "UBFM", Mask: 0x7F800000, Value: 0x53000000, Op: UBFM,
		Fields: []FieldDef{fSF, {Name: "immr", Hi: 21, Lo: 16}, {Name: "imms", Hi: 15, Lo: 10}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["immr"]
			inst.Shift = int(f["imms"])
		},
	},
	{
		// BFM: opc=01, Bitfield Move (BFI/BFXIL aliases)
		Name: "BFM", Mask: 0x7F800000, Value: 0x33000000, Op: BFM,
		Fields: []FieldDef{fSF, {Name: "immr", Hi: 21, Lo: 16}, {Name: "imms", Hi: 15, Lo: 10}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["immr"]        // immr
			inst.Shift = int(f["imms"]) // imms
		},
	},

	// ---- Extract (EXTR) ----
	// Encoding: sf:00:100111:N0:Rm:imms:Rn:Rd
	{
		Name: "EXTR", Mask: 0x7F800000, Value: 0x13800000, Op: EXTR,
		Fields: []FieldDef{fSF, fRm16, {Name: "imms", Hi: 15, Lo: 10}, fRn, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["imms"]
		},
	},

	// ---- PC-relative addressing ----
	// ADR:  0:immlo:10000:immhi:Rd
	// ADRP: 1:immlo:10000:immhi:Rd
	{
		Name: "ADR", Mask: 0x9F000000, Value: 0x10000000, Op: ADR,
		Fields: []FieldDef{{Name: "immhi", Hi: 23, Lo: 5}, {Name: "immlo", Hi: 30, Lo: 29}, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			imm := (f["immhi"] << 2) | f["immlo"]
			inst.Imm = SignExtend(uint32(imm), 21)
		},
	},
	{
		Name: "ADRP", Mask: 0x9F000000, Value: 0x90000000, Op: ADRP,
		Fields: []FieldDef{{Name: "immhi", Hi: 23, Lo: 5}, {Name: "immlo", Hi: 30, Lo: 29}, fRd},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			imm := (f["immhi"] << 2) | f["immlo"]
			inst.Imm = SignExtend(uint32(imm), 21) << 12
		},
	},
}

// postBitmaskImm decodes logical immediate bitmask
// For logical imm instructions, Rn=31 means XZR (zero register), not SP
func postBitmaskImm(f map[string]int64, inst *vm.Instruction) {
	n := uint32(f["N"])
	immr := uint32(f["immr"])
	imms := uint32(f["imms"])
	imm, ok := decodeBitmaskImm(n, immr, imms, inst.SF)
	if !ok {
		inst.Op = int(UNSUPPORTED)
		return
	}
	inst.Imm = int64(imm)
	xzrReplace(&inst.Rn) // Rn=31 → XZR (logical imm: Rn is always XZR, not SP)
}

// postBitmaskImmANDS ANDS(imm) specific: Rn=31→XZR, Rd=31→XZR (TST alias)
func postBitmaskImmANDS(f map[string]int64, inst *vm.Instruction) {
	n := uint32(f["N"])
	immr := uint32(f["immr"])
	imms := uint32(f["imms"])
	imm, ok := decodeBitmaskImm(n, immr, imms, inst.SF)
	if !ok {
		inst.Op = int(UNSUPPORTED)
		return
	}
	inst.Imm = int64(imm)
	xzrReplace(&inst.Rn) // Rn=31 → XZR
	xzrReplace(&inst.Rd) // Rd=31 → XZR (TST = ANDS XZR, Xn, #imm)
}
