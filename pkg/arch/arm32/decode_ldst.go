package arm32

import "github.com/vmpacker/pkg/vm"

// ARM32 load/store instruction patterns.
//
// Word/Byte immediate (op1=010): cond:010:P:U:B:W:L:Rn:Rd:imm12
// Word/Byte register  (op1=011): cond:011:P:U:B:W:L:Rn:Rd:shamt:shtype:0:Rm
// Halfword/Signed byte (op1=000, special encoding):
//   cond:000:P:U:1:W:L:Rn:Rd:imm4H:1:op2:1:imm4L  (immediate)
//   cond:000:P:U:0:W:L:Rn:Rd:0000:1:op2:1:Rm       (register)
// Load/store multiple (op1=100): cond:100:P:U:S:W:L:Rn:reglist

// Field definitions
var ldstImmFieldsWB = []FieldDef{
	fRd, fRn,
	{Name: "imm12", Hi: 11, Lo: 0},
	{Name: "P", Hi: 24, Lo: 24},
	{Name: "U", Hi: 23, Lo: 23},
	{Name: "W", Hi: 21, Lo: 21},
}

var ldstRegFieldsWB = []FieldDef{
	fRd, fRn, fRm,
	{Name: "shamt", Hi: 11, Lo: 7},
	{Name: "shtype", Hi: 6, Lo: 5},
	{Name: "P", Hi: 24, Lo: 24},
	{Name: "U", Hi: 23, Lo: 23},
	{Name: "W", Hi: 21, Lo: 21},
}

// postLdStImm processes the P/U/W and imm12 fields
func postLdStImm(f map[string]int64, inst *vm.Instruction) {
	imm := f["imm12"]
	if f["U"] == 0 {
		imm = -imm
	}
	inst.Imm = imm

	p := int(f["P"])
	w := int(f["W"])
	if p == 1 && w == 0 {
		inst.WB = 0 // offset (no writeback)
	} else if p == 1 && w == 1 {
		inst.WB = 3 // pre-index
	} else if p == 0 {
		inst.WB = 1 // post-index
	}
}

// postLdStReg processes register-offset load/store
func postLdStReg(f map[string]int64, inst *vm.Instruction) {
	inst.ShiftType = int(f["shtype"])
	inst.Shift = int(f["shamt"])

	if f["U"] == 0 {
		inst.Imm = -1 // flag: subtract register offset
	} else {
		inst.Imm = 1 // flag: add register offset
	}

	p := int(f["P"])
	w := int(f["W"])
	if p == 1 && w == 0 {
		inst.WB = 0
	} else if p == 1 && w == 1 {
		inst.WB = 3
	} else if p == 0 {
		inst.WB = 1
	}
}

// ldstImmPatterns: op1=010 — LDR/STR/LDRB/STRB (immediate)
var ldstImmPatterns = []InstrPattern{
	// LDR(imm): cond:010:P:U:0:W:1:Rn:Rd:imm12 (B=0, L=1)
	{Name: "LDR_IMM", Mask: 0x0E500000, Value: 0x04100000, Op: LDR_IMM, Fields: ldstImmFieldsWB, Post: postLdStImm},
	// STR(imm): cond:010:P:U:0:W:0:Rn:Rd:imm12 (B=0, L=0)
	{Name: "STR_IMM", Mask: 0x0E500000, Value: 0x04000000, Op: STR_IMM, Fields: ldstImmFieldsWB, Post: postLdStImm},
	// LDRB(imm): cond:010:P:U:1:W:1:Rn:Rd:imm12 (B=1, L=1)
	{Name: "LDRB_IMM", Mask: 0x0E500000, Value: 0x04500000, Op: LDRB_IMM, Fields: ldstImmFieldsWB, Post: postLdStImm},
	// STRB(imm): cond:010:P:U:1:W:0:Rn:Rd:imm12 (B=1, L=0)
	{Name: "STRB_IMM", Mask: 0x0E500000, Value: 0x04400000, Op: STRB_IMM, Fields: ldstImmFieldsWB, Post: postLdStImm},
}

// ldstRegPatterns: op1=011 — LDR/STR/LDRB/STRB (register)
var ldstRegPatterns = []InstrPattern{
	{Name: "LDR_REG", Mask: 0x0E500010, Value: 0x06100000, Op: LDR_REG, Fields: ldstRegFieldsWB, Post: postLdStReg},
	{Name: "STR_REG", Mask: 0x0E500010, Value: 0x06000000, Op: STR_REG, Fields: ldstRegFieldsWB, Post: postLdStReg},
	{Name: "LDRB_REG", Mask: 0x0E500010, Value: 0x06500000, Op: LDRB_REG, Fields: ldstRegFieldsWB, Post: postLdStReg},
	{Name: "STRB_REG", Mask: 0x0E500010, Value: 0x06400000, Op: STRB_REG, Fields: ldstRegFieldsWB, Post: postLdStReg},
}

// Halfword/signed load/store fields (immediate)
var ldstHalfImmFields = []FieldDef{
	fRd, fRn,
	{Name: "imm4H", Hi: 11, Lo: 8},
	{Name: "imm4L", Hi: 3, Lo: 0},
	{Name: "P", Hi: 24, Lo: 24},
	{Name: "U", Hi: 23, Lo: 23},
	{Name: "W", Hi: 21, Lo: 21},
	{Name: "op2", Hi: 6, Lo: 5},
}

// postLdStHalfImm processes halfword immediate offset
func postLdStHalfImm(f map[string]int64, inst *vm.Instruction) {
	imm := (f["imm4H"] << 4) | f["imm4L"]
	if f["U"] == 0 {
		imm = -imm
	}
	inst.Imm = imm

	p := int(f["P"])
	w := int(f["W"])
	if p == 1 && w == 0 {
		inst.WB = 0
	} else if p == 1 && w == 1 {
		inst.WB = 3
	} else if p == 0 {
		inst.WB = 1
	}
}

// Halfword/signed load/store fields (register)
var ldstHalfRegFields = []FieldDef{
	fRd, fRn, fRm,
	{Name: "P", Hi: 24, Lo: 24},
	{Name: "U", Hi: 23, Lo: 23},
	{Name: "W", Hi: 21, Lo: 21},
	{Name: "op2", Hi: 6, Lo: 5},
}

func postLdStHalfReg(f map[string]int64, inst *vm.Instruction) {
	if f["U"] == 0 {
		inst.Imm = -1
	} else {
		inst.Imm = 1
	}
	p := int(f["P"])
	w := int(f["W"])
	if p == 1 && w == 0 {
		inst.WB = 0
	} else if p == 1 && w == 1 {
		inst.WB = 3
	} else if p == 0 {
		inst.WB = 1
	}
}

// ldstHalfPatterns: halfword/signed byte load/store
// Encoding: cond:000:P:U:1:W:L:Rn:Rd:imm4H:1:SH:1:imm4L (immediate, bit[22]=1)
//           cond:000:P:U:0:W:L:Rn:Rd:0000:1:SH:1:Rm     (register, bit[22]=0)
var ldstHalfPatterns = []InstrPattern{
	// STRH(imm): cond:000:P:U:1:W:0:Rn:Rd:imm4H:1011:imm4L (L=0, SH=01)
	{Name: "STRH_IMM", Mask: 0x0E4000F0, Value: 0x004000B0, Op: STRH_IMM, Fields: ldstHalfImmFields, Post: postLdStHalfImm},
	// LDRH(imm): cond:000:P:U:1:W:1:Rn:Rd:imm4H:1011:imm4L (L=1, SH=01)
	{Name: "LDRH_IMM", Mask: 0x0E4000F0, Value: 0x005000B0, Op: LDRH_IMM, Fields: ldstHalfImmFields, Post: postLdStHalfImm},
	// LDRSB(imm): cond:000:P:U:1:W:1:Rn:Rd:imm4H:1101:imm4L (L=1, SH=10)
	{Name: "LDRSB_IMM", Mask: 0x0E4000F0, Value: 0x005000D0, Op: LDRSB_IMM, Fields: ldstHalfImmFields, Post: postLdStHalfImm},
	// LDRSH(imm): cond:000:P:U:1:W:1:Rn:Rd:imm4H:1111:imm4L (L=1, SH=11)
	{Name: "LDRSH_IMM", Mask: 0x0E4000F0, Value: 0x005000F0, Op: LDRSH_IMM, Fields: ldstHalfImmFields, Post: postLdStHalfImm},
	// LDRD(imm): cond:000:P:U:1:W:0:Rn:Rd:imm4H:1101:imm4L (L=0, SH=10)
	{Name: "LDRD_IMM", Mask: 0x0E4000F0, Value: 0x004000D0, Op: LDRD_IMM, Fields: ldstHalfImmFields, Post: postLdStHalfImm},
	// STRD(imm): cond:000:P:U:1:W:0:Rn:Rd:imm4H:1111:imm4L (L=0, SH=11)
	{Name: "STRD_IMM", Mask: 0x0E4000F0, Value: 0x004000F0, Op: STRD_IMM, Fields: ldstHalfImmFields, Post: postLdStHalfImm},

	// Register variants (bit[22]=0)
	{Name: "STRH_REG", Mask: 0x0E4000F0, Value: 0x000000B0, Op: STRH_REG, Fields: ldstHalfRegFields, Post: postLdStHalfReg},
	{Name: "LDRH_REG", Mask: 0x0E4000F0, Value: 0x001000B0, Op: LDRH_REG, Fields: ldstHalfRegFields, Post: postLdStHalfReg},
	{Name: "LDRSB_REG", Mask: 0x0E4000F0, Value: 0x001000D0, Op: LDRSB_REG, Fields: ldstHalfRegFields, Post: postLdStHalfReg},
	{Name: "LDRSH_REG", Mask: 0x0E4000F0, Value: 0x001000F0, Op: LDRSH_REG, Fields: ldstHalfRegFields, Post: postLdStHalfReg},
}

// ldstMultiPatterns: op1=100 — LDM/STM
var ldstMultiPatterns = []InstrPattern{
	// LDM: cond:100:P:U:0:W:1:Rn:reglist (S=0, L=1)
	{
		Name: "LDM", Mask: 0x0E500000, Value: 0x08100000, Op: LDM,
		Fields: []FieldDef{
			fRn,
			{Name: "reglist", Hi: 15, Lo: 0},
			{Name: "P", Hi: 24, Lo: 24},
			{Name: "U", Hi: 23, Lo: 23},
			{Name: "W", Hi: 21, Lo: 21},
		},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["reglist"]
			p := int(f["P"])
			u := int(f["U"])
			w := int(f["W"])
			// Encode addressing mode in ShiftType:
			// 0=IA(increment after), 1=IB(increment before), 2=DA(decrement after), 3=DB(decrement before)
			if u == 1 && p == 0 {
				inst.ShiftType = 0 // IA
			} else if u == 1 && p == 1 {
				inst.ShiftType = 1 // IB
			} else if u == 0 && p == 0 {
				inst.ShiftType = 2 // DA
			} else {
				inst.ShiftType = 3 // DB
			}
			inst.WB = w
		},
	},
	// STM: cond:100:P:U:0:W:0:Rn:reglist (S=0, L=0)
	{
		Name: "STM", Mask: 0x0E500000, Value: 0x08000000, Op: STM,
		Fields: []FieldDef{
			fRn,
			{Name: "reglist", Hi: 15, Lo: 0},
			{Name: "P", Hi: 24, Lo: 24},
			{Name: "U", Hi: 23, Lo: 23},
			{Name: "W", Hi: 21, Lo: 21},
		},
		Post: func(f map[string]int64, inst *vm.Instruction) {
			inst.Imm = f["reglist"]
			p := int(f["P"])
			u := int(f["U"])
			w := int(f["W"])
			if u == 1 && p == 0 {
				inst.ShiftType = 0
			} else if u == 1 && p == 1 {
				inst.ShiftType = 1
			} else if u == 0 && p == 0 {
				inst.ShiftType = 2
			} else {
				inst.ShiftType = 3
			}
			inst.WB = w
		},
	},
}
