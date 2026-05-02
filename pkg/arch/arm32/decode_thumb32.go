package arm32

import "github.com/vmpacker/pkg/vm"

// Thumb-2 (32-bit) instruction decoder.
// 32-bit Thumb-2 instructions: first halfword has bits[15:11] in {11101, 11110, 11111}.
// Encoding: [hw1:16][hw2:16] where hw1 is the first halfword (higher 16 bits of raw).

func (d *Decoder) decodeThumb32Inst(raw uint32, inst *vm.Instruction) {
	hw1 := uint16(raw >> 16)
	hw2 := uint16(raw & 0xFFFF)

	op1 := (hw1 >> 11) & 0x3 // bits[12:11] of hw1
	op2 := (hw1 >> 4) & 0x7F // bits[10:4] of hw1
	op := (hw2 >> 15) & 1    // bit[15] of hw2

	switch {
	case op1 == 1:
		if op2>>5 == 0 {
			// Load/store multiple, dual, exclusive, table branch
			d.decodeThumb32LdStMulti(hw1, hw2, inst)
		} else if op2>>5 == 1 {
			// Data processing (shifted register)
			d.decodeThumb32DPShifted(hw1, hw2, inst)
		} else {
			// Coprocessor / other
			inst.Op = int(UNSUPPORTED)
		}

	case op1 == 2:
		if op == 0 {
			if op2&0x20 == 0 {
				// Data processing (modified immediate)
				d.decodeThumb32DPModImm(hw1, hw2, inst)
			} else {
				// Data processing (plain binary immediate)
				d.decodeThumb32DPPlain(hw1, hw2, inst)
			}
		} else {
			// Branch and misc control
			d.decodeThumb32BranchMisc(hw1, hw2, inst)
		}

	case op1 == 3:
		if op2>>5 == 0 {
			// Load/store single
			d.decodeThumb32LdStSingle(hw1, hw2, inst)
		} else if hw1&0x100 == 0 {
			// hw1 bit[8]=0: Data processing (register) — shifts, extensions, REV, CLZ
			d.decodeThumb32DPReg(hw1, hw2, inst)
		} else {
			// hw1 bit[8]=1: Multiply / divide / long multiply
			if op2>>4 == 4 || op2>>4 == 5 {
				d.decodeThumb32MulDiv(hw1, hw2, inst)
			} else if op2>>4 == 6 || op2>>4 == 7 {
				d.decodeThumb32LongMulDiv(hw1, hw2, inst)
			} else {
				// op2 in 0x30-0x3F with bit[8]=1: short/long multiply + divide
				d.decodeThumb32MulDiv(hw1, hw2, inst)
			}
		}
	}
}

// decodeThumb32DPModImm: data processing (modified immediate)
// Encoding: 11110:i:0:op:S:Rn || 0:imm3:Rd:imm8
func (d *Decoder) decodeThumb32DPModImm(hw1, hw2 uint16, inst *vm.Instruction) {
	op := (hw1 >> 5) & 0xF
	s := (hw1 >> 4) & 1
	rn := int(hw1 & 0xF)
	rd := int((hw2 >> 8) & 0xF)

	imm := d.thumbExpandImm(hw1, hw2)

	inst.Rd = rd
	inst.Rn = rn
	inst.Imm = int64(imm)

	switch op {
	case 0x0:
		if rd == 15 && s == 1 {
			inst.Op = int(TST_IMM)
		} else if s == 1 {
			inst.Op = int(ANDS_IMM)
		} else {
			inst.Op = int(AND_IMM)
		}
	case 0x1:
		if s == 1 {
			inst.Op = int(BICS_IMM)
		} else {
			inst.Op = int(BIC_IMM)
		}
	case 0x2:
		if rn == 15 {
			if s == 1 {
				inst.Op = int(MOVS_IMM)
			} else {
				inst.Op = int(MOV_IMM)
			}
		} else {
			if s == 1 {
				inst.Op = int(ORRS_IMM)
			} else {
				inst.Op = int(ORR_IMM)
			}
		}
	case 0x3:
		if rn == 15 {
			if s == 1 {
				inst.Op = int(MVNS_IMM)
			} else {
				inst.Op = int(MVN_IMM)
			}
		} else {
			if s == 1 {
				inst.Op = int(ORRS_IMM) // ORN with S — treat as ORRS with inverted imm
			} else {
				inst.Op = int(ORR_IMM)
			}
			inst.Imm = int64(^uint32(imm))
		}
	case 0x4:
		if rd == 15 && s == 1 {
			inst.Op = int(TEQ_IMM)
		} else if s == 1 {
			inst.Op = int(EORS_IMM)
		} else {
			inst.Op = int(EOR_IMM)
		}
	case 0x8:
		if rd == 15 && s == 1 {
			inst.Op = int(CMN_IMM)
		} else if s == 1 {
			inst.Op = int(ADDS_IMM)
		} else {
			inst.Op = int(ADD_IMM)
		}
	case 0xA:
		if s == 1 {
			inst.Op = int(ADCS_IMM)
		} else {
			inst.Op = int(ADC_IMM)
		}
	case 0xB:
		if s == 1 {
			inst.Op = int(SBCS_IMM)
		} else {
			inst.Op = int(SBC_IMM)
		}
	case 0xD:
		if rd == 15 && s == 1 {
			inst.Op = int(CMP_IMM)
		} else if s == 1 {
			inst.Op = int(SUBS_IMM)
		} else {
			inst.Op = int(SUB_IMM)
		}
	case 0xE:
		if s == 1 {
			inst.Op = int(RSBS_IMM)
		} else {
			inst.Op = int(RSB_IMM)
		}
	}
}

// thumbExpandImm decodes Thumb-2 modified immediate constant
func (d *Decoder) thumbExpandImm(hw1, hw2 uint16) uint32 {
	i := uint32((hw1 >> 10) & 1)
	imm3 := uint32((hw2 >> 12) & 0x7)
	imm8 := uint32(hw2 & 0xFF)
	imm12 := (i << 11) | (imm3 << 8) | imm8

	if imm12>>10 == 0 {
		switch (imm12 >> 8) & 0x3 {
		case 0:
			return imm8
		case 1:
			return (imm8 << 16) | imm8
		case 2:
			return (imm8 << 24) | (imm8 << 8)
		case 3:
			return (imm8 << 24) | (imm8 << 16) | (imm8 << 8) | imm8
		}
	}
	// ROR rotation
	unrot := uint32(0x80) | (imm12 & 0x7F)
	shift := imm12 >> 7
	return (unrot >> shift) | (unrot << (32 - shift))
}

// decodeThumb32DPPlain: data processing (plain binary immediate)
// MOVW, MOVT, ADDW, SUBW, etc.
func (d *Decoder) decodeThumb32DPPlain(hw1, hw2 uint16, inst *vm.Instruction) {
	op := (hw1 >> 4) & 0x1F
	rn := int(hw1 & 0xF)
	rd := int((hw2 >> 8) & 0xF)

	i := int64((hw1 >> 10) & 1)
	imm3 := int64((hw2 >> 12) & 0x7)
	imm8 := int64(hw2 & 0xFF)
	imm12 := (i << 11) | (imm3 << 8) | imm8

	inst.Rd = rd
	inst.Rn = rn

	switch op {
	case 0x00:
		if rn == 15 {
			inst.Op = int(ADR)
			inst.Imm = imm12
		} else {
			inst.Op = int(ADD_IMM)
			inst.Imm = imm12
		}
	case 0x04:
		// MOVW
		imm16 := (int64(hw1&0xF) << 12) | imm12
		inst.Op = int(MOVW)
		inst.Imm = imm16
	case 0x0A:
		if rn == 15 {
			inst.Op = int(ADR)
			inst.Imm = -imm12
		} else {
			inst.Op = int(SUB_IMM)
			inst.Imm = imm12
		}
	case 0x0C:
		// MOVT
		imm16 := (int64(hw1&0xF) << 12) | imm12
		inst.Op = int(MOVT)
		inst.Imm = imm16
	case 0x16:
		// UBFX: 11110:0:11:110:0:Rn:0:imm3:Rd:imm2:widthm1
		// For now map to UNSUPPORTED, can be expanded later
		inst.Op = int(UNSUPPORTED)
	case 0x14:
		// SBFX
		inst.Op = int(UNSUPPORTED)
	}
}

// decodeThumb32DPShifted: data processing (shifted register)
func (d *Decoder) decodeThumb32DPShifted(hw1, hw2 uint16, inst *vm.Instruction) {
	op := (hw1 >> 5) & 0xF
	s := (hw1 >> 4) & 1
	rn := int(hw1 & 0xF)
	rd := int((hw2 >> 8) & 0xF)
	rm := int(hw2 & 0xF)

	imm3 := int((hw2 >> 12) & 0x7)
	imm2 := int((hw2 >> 6) & 0x3)
	shtype := int((hw2 >> 4) & 0x3)
	shamt := (imm3 << 2) | imm2

	inst.Rd = rd
	inst.Rn = rn
	inst.Rm = rm
	inst.Shift = shamt
	inst.ShiftType = shtype

	switch op {
	case 0x0:
		if rd == 15 && s == 1 {
			inst.Op = int(TST_REG)
		} else if s == 1 {
			inst.Op = int(ANDS_REG)
		} else {
			inst.Op = int(AND_REG)
		}
	case 0x1:
		if s == 1 {
			inst.Op = int(BICS_REG)
		} else {
			inst.Op = int(BIC_REG)
		}
	case 0x3:
		if rn == 15 {
			// MVN (shifted register)
			if s == 1 {
				inst.Op = int(MVNS_REG)
			} else {
				inst.Op = int(MVN_REG)
			}
		} else {
			// ORN (OR NOT) — treat as ORR with inverted Rm semantics
			if s == 1 {
				inst.Op = int(ORRS_REG)
			} else {
				inst.Op = int(ORR_REG)
			}
			inst.WB = 2 // flag for ORN (invert Rm before OR)
		}
	case 0x2:
		if rn == 15 {
			// MOV/shift: LSL, LSR, ASR, ROR, RRX
			if s == 1 {
				inst.Op = int(MOVS_REG)
			} else {
				inst.Op = int(MOV_REG)
			}
		} else {
			if s == 1 {
				inst.Op = int(ORRS_REG)
			} else {
				inst.Op = int(ORR_REG)
			}
		}
	case 0x4:
		if rd == 15 && s == 1 {
			inst.Op = int(TEQ_REG)
		} else if s == 1 {
			inst.Op = int(EORS_REG)
		} else {
			inst.Op = int(EOR_REG)
		}
	case 0x8:
		if rd == 15 && s == 1 {
			inst.Op = int(CMN_REG)
		} else if s == 1 {
			inst.Op = int(ADDS_REG)
		} else {
			inst.Op = int(ADD_REG)
		}
	case 0xA:
		if s == 1 {
			inst.Op = int(ADCS_REG)
		} else {
			inst.Op = int(ADC_REG)
		}
	case 0xB:
		if s == 1 {
			inst.Op = int(SBCS_REG)
		} else {
			inst.Op = int(SBC_REG)
		}
	case 0xD:
		if rd == 15 && s == 1 {
			inst.Op = int(CMP_REG)
		} else if s == 1 {
			inst.Op = int(SUBS_REG)
		} else {
			inst.Op = int(SUB_REG)
		}
	case 0xE:
		if s == 1 {
			inst.Op = int(RSBS_REG)
		} else {
			inst.Op = int(RSB_REG)
		}
	}
}

// decodeThumb32BranchMisc: branch and misc control
func (d *Decoder) decodeThumb32BranchMisc(hw1, hw2 uint16, inst *vm.Instruction) {
	op1 := (hw2 >> 12) & 0x7
	op2 := (hw1 >> 4) & 0x7F

	switch {
	case op1&0x5 == 0 && op2&0x38 != 0x38:
		// B.W (conditional): 11110:S:cond:imm6 || 10:J1:0:J2:imm11
		// op1 = 0 or 2 → hw2[15:14]=10, hw2[12]=0
		cond := int((hw1 >> 6) & 0xF)
		s := int64((hw1 >> 10) & 1)
		imm6 := int64(hw1 & 0x3F)
		j1 := int64((hw2 >> 13) & 1)
		j2 := int64((hw2 >> 11) & 1)
		imm11 := int64(hw2 & 0x7FF)
		offset := (s << 20) | (j2 << 19) | (j1 << 18) | (imm6 << 12) | (imm11 << 1)
		inst.Op = int(B)
		inst.Cond = cond
		inst.Imm = SignExtend(uint32(offset), 21)

	case op1&0x5 == 1:
		// B.W (unconditional): 11110:S:imm10 || 10:J1:1:J2:imm11
		// op1 = 1 or 3 → hw2[15:14]=10, hw2[12]=1
		s := int64((hw1 >> 10) & 1)
		imm10 := int64(hw1 & 0x3FF)
		j1 := int64((hw2 >> 13) & 1)
		j2 := int64((hw2 >> 11) & 1)
		imm11 := int64(hw2 & 0x7FF)
		i1 := ^(j1 ^ s) & 1
		i2 := ^(j2 ^ s) & 1
		offset := (s << 24) | (i1 << 23) | (i2 << 22) | (imm10 << 12) | (imm11 << 1)
		inst.Op = int(B)
		inst.Imm = SignExtend(uint32(offset), 25)

	case op1&0x5 == 4:
		// BLX (immediate): 11110:S:imm10H || 11:J1:0:J2:imm10L:0
		// op1 = 4 or 6 → hw2[15:14]=11, hw2[12]=0
		s := int64((hw1 >> 10) & 1)
		imm10h := int64(hw1 & 0x3FF)
		j1 := int64((hw2 >> 13) & 1)
		j2 := int64((hw2 >> 11) & 1)
		imm10l := int64((hw2 >> 1) & 0x3FF)
		i1 := ^(j1 ^ s) & 1
		i2 := ^(j2 ^ s) & 1
		offset := (s << 24) | (i1 << 23) | (i2 << 22) | (imm10h << 12) | (imm10l << 2)
		inst.Op = int(BLX_IMM)
		inst.Imm = SignExtend(uint32(offset), 25)

	case op1&0x5 == 5:
		// BL: 11110:S:imm10 || 11:J1:1:J2:imm11
		// op1 = 5 or 7 → hw2[15:14]=11, hw2[12]=1
		s := int64((hw1 >> 10) & 1)
		imm10 := int64(hw1 & 0x3FF)
		j1 := int64((hw2 >> 13) & 1)
		j2 := int64((hw2 >> 11) & 1)
		imm11 := int64(hw2 & 0x7FF)
		i1 := ^(j1 ^ s) & 1
		i2 := ^(j2 ^ s) & 1
		offset := (s << 24) | (i1 << 23) | (i2 << 22) | (imm10 << 12) | (imm11 << 1)
		inst.Op = int(BL)
		inst.Imm = SignExtend(uint32(offset), 25)

	case op1 == 2 && op2 == 0x3F:
		// MSR
		inst.Op = int(MSR)
		inst.Rm = int(hw1 & 0xF)
		inst.Imm = int64((hw2 >> 8) & 0xF)

	case op1 == 2 && op2&0x70 == 0x30:
		// Hints / barriers
		hint := (hw2 >> 4) & 0xF
		switch hint {
		case 4:
			inst.Op = int(DMB)
		case 5:
			inst.Op = int(DSB)
		case 6:
			inst.Op = int(ISB)
		default:
			inst.Op = int(NOP)
		}

	case op1 == 6 && op2 == 0x3F:
		// MRS
		inst.Op = int(MRS)
		inst.Rd = int((hw2 >> 8) & 0xF)
		inst.Imm = int64(hw2 & 0xFF)
	}
}

// decodeThumb32LdStMulti: load/store multiple, dual
func (d *Decoder) decodeThumb32LdStMulti(hw1, hw2 uint16, inst *vm.Instruction) {
	op := (hw1 >> 7) & 0x3
	l := (hw1 >> 4) & 1
	w := (hw1 >> 5) & 1
	rn := int(hw1 & 0xF)

	switch {
	case op == 1 && l == 0:
		// STM.W
		inst.Op = int(STM)
		inst.Rn = rn
		inst.Imm = int64(hw2)
		inst.ShiftType = 0 // IA
		inst.WB = int(w)
	case op == 1 && l == 1:
		// LDM.W
		inst.Op = int(LDM)
		inst.Rn = rn
		inst.Imm = int64(hw2)
		inst.ShiftType = 0 // IA
		inst.WB = int(w)
	case op == 2 && l == 0:
		// STMDB
		inst.Op = int(STM)
		inst.Rn = rn
		inst.Imm = int64(hw2)
		inst.ShiftType = 3 // DB
		inst.WB = int(w)
	case op == 2 && l == 1:
		// LDMDB
		inst.Op = int(LDM)
		inst.Rn = rn
		inst.Imm = int64(hw2)
		inst.ShiftType = 3 // DB
		inst.WB = int(w)
	default:
		// LDRD/STRD or other — simplified
		if (hw1>>4)&0x1 == 1 {
			inst.Op = int(LDRD_IMM)
		} else {
			inst.Op = int(STRD_IMM)
		}
		inst.Rd = int((hw2 >> 12) & 0xF)
		inst.Rn = rn
		inst.Rm = int((hw2 >> 8) & 0xF) // Rd2
		imm8 := int64(hw2&0xFF) * 4
		if (hw1>>7)&1 == 0 {
			imm8 = -imm8
		}
		inst.Imm = imm8
		inst.WB = int(w)
	}
}

// decodeThumb32LdStSingle: load/store single data item
func (d *Decoder) decodeThumb32LdStSingle(hw1, hw2 uint16, inst *vm.Instruction) {
	// hw1 bit layout: 1111 100 [8]sign [7]imm12 [6:5]size [4]load [3:0]Rn
	rn := int(hw1 & 0xF)
	rd := int((hw2 >> 12) & 0xF)
	inst.Rd = rd
	inst.Rn = rn

	isLoad := (hw1>>4)&1 == 1
	size := (hw1 >> 5) & 3  // 0=byte, 1=half, 2=word
	isImm12 := (hw1>>7)&1 == 1
	isSigned := (hw1>>8)&1 == 1

	setOp := func() {
		if isSigned && isLoad {
			switch size {
			case 0:
				inst.Op = int(LDRSB_IMM)
			case 1:
				inst.Op = int(LDRSH_IMM)
			}
			return
		}
		switch size {
		case 0:
			if isLoad {
				inst.Op = int(LDRB_IMM)
			} else {
				inst.Op = int(STRB_IMM)
			}
		case 1:
			if isLoad {
				inst.Op = int(LDRH_IMM)
			} else {
				inst.Op = int(STRH_IMM)
			}
		case 2:
			if isLoad {
				inst.Op = int(LDR_IMM)
			} else {
				inst.Op = int(STR_IMM)
			}
		}
	}

	setOpReg := func() {
		if isSigned && isLoad {
			switch size {
			case 0:
				inst.Op = int(LDRSB_REG)
			case 1:
				inst.Op = int(LDRSH_REG)
			}
			return
		}
		switch size {
		case 0:
			if isLoad {
				inst.Op = int(LDRB_REG)
			} else {
				inst.Op = int(STRB_REG)
			}
		case 1:
			if isLoad {
				inst.Op = int(LDRH_REG)
			} else {
				inst.Op = int(STRH_REG)
			}
		case 2:
			if isLoad {
				inst.Op = int(LDR_REG)
			} else {
				inst.Op = int(STR_REG)
			}
		}
	}

	if isImm12 {
		// 12-bit unsigned immediate: Rn + imm12
		inst.Imm = int64(hw2 & 0xFFF)
		setOp()
		return
	}

	// hw1 bit[7] = 0: register offset or 8-bit immediate
	if hw2&0x0800 == 0 {
		// Register offset: hw2 = Rt:000000:shift:Rm
		rm := int(hw2 & 0xF)
		shift := int((hw2 >> 4) & 0x3)
		inst.Rm = rm
		inst.Shift = shift
		inst.ShiftType = 0 // LSL
		inst.Imm = 1       // positive direction
		setOpReg()
		return
	}

	// 8-bit immediate with P/U/W indexing: hw2 = Rt:1PUW:imm8
	imm8 := int64(hw2 & 0xFF)
	u := (hw2 >> 9) & 1
	p := (hw2 >> 10) & 1
	w := (hw2 >> 8) & 1

	if u == 0 {
		imm8 = -imm8
	}
	inst.Imm = imm8

	if p == 1 && w == 0 {
		inst.WB = 0
	} else if p == 1 && w == 1 {
		inst.WB = 3 // pre-index
	} else if p == 0 && w == 1 {
		inst.WB = 1 // post-index
	}

	setOp()
}

// decodeThumb32DPReg: data processing (register)
func (d *Decoder) decodeThumb32DPReg(hw1, hw2 uint16, inst *vm.Instruction) {
	// hw1[7:4] includes S bit for shifts: 1111 1010 xxxS Rn
	op1 := (hw1 >> 4) & 0xF
	op2 := (hw2 >> 4) & 0xF
	rn := int(hw1 & 0xF)
	rd := int((hw2 >> 8) & 0xF)
	rm := int(hw2 & 0xF)

	inst.Rd = rd
	inst.Rn = rn
	inst.Rm = rm

	switch {
	// Shift register: op1[3:1] selects shift type, op1[0]=S flag
	case op1>>1 == 0x0 && op2 == 0: // op1=000S → LSL
		inst.Op = int(LSL_REG)
	case op1>>1 == 0x1 && op2 == 0: // op1=001S → LSR
		inst.Op = int(LSR_REG)
	case op1>>1 == 0x2 && op2 == 0: // op1=010S → ASR
		inst.Op = int(ASR_REG)
	case op1>>1 == 0x3 && op2 == 0: // op1=011S → ROR
		inst.Op = int(ROR_REG)

	// Sign/zero extend: op1=0..5, Rn=0xF, op2=10xx
	// SXTH: op1=0, UXTH: op1=1, SXTB16: op1=2, UXTB16: op1=3, SXTB: op1=4, UXTB: op1=5
	case op1 <= 5 && rn == 15 && op2>>2 == 2:
		switch op1 {
		case 0: // SXTH
			inst.Op = int(MOV_REG)
			inst.Imm = 16
		case 1: // UXTH
			inst.Op = int(MOV_REG)
			inst.Imm = -16
		case 4: // SXTB
			inst.Op = int(MOV_REG)
			inst.Imm = 8
		case 5: // UXTB
			inst.Op = int(MOV_REG)
			inst.Imm = -8
		default: // SXTB16/UXTB16 — rare, treat as unsupported
			inst.Op = int(UNSUPPORTED)
		}

	// REV/REV16/RBIT: op1=1001
	case op1 == 0x9 && op2 == 0x8:
		inst.Op = int(REV)
	case op1 == 0x9 && op2 == 0x9:
		inst.Op = int(REV16)
	case op1 == 0x9 && op2 == 0xA:
		inst.Op = int(RBIT)

	// CLZ: op1=1011, op2=1000
	case op1 == 0xB && op2 == 0x8:
		inst.Op = int(CLZ)
	}
}

// decodeThumb32MulDiv: multiply, divide, accumulate
func (d *Decoder) decodeThumb32MulDiv(hw1, hw2 uint16, inst *vm.Instruction) {
	op1 := (hw1 >> 4) & 0x7
	op2 := (hw2 >> 4) & 0x3
	rn := int(hw1 & 0xF)
	rd := int((hw2 >> 8) & 0xF)
	rm := int(hw2 & 0xF)
	ra := int((hw2 >> 12) & 0xF)

	inst.Rd = rd
	inst.Rn = rn
	inst.Rm = rm

	switch op1 {
	case 0x0:
		if ra == 15 {
			inst.Op = int(MUL)
		} else {
			inst.Op = int(MLA)
			inst.Imm = int64(ra)
		}
	case 0x1:
		if op2 == 0 && ra != 15 {
			// MLS
			inst.Op = int(MLA)
			inst.Imm = int64(ra)
			inst.SF = true // flag to indicate subtraction
		} else if op2 == 0xF {
			inst.Op = int(SDIV)
		}
	case 0x3:
		if op2 == 0xF {
			inst.Op = int(UDIV)
		}
	}
}

// decodeThumb32LongMulDiv: long multiply, divide
func (d *Decoder) decodeThumb32LongMulDiv(hw1, hw2 uint16, inst *vm.Instruction) {
	op1 := (hw1 >> 4) & 0x7
	rn := int(hw1 & 0xF)
	rdLo := int((hw2 >> 12) & 0xF)
	rdHi := int((hw2 >> 8) & 0xF)
	rm := int(hw2 & 0xF)

	inst.Rd = rdLo
	inst.Rn = rn
	inst.Rm = rm
	inst.Imm = int64(rdHi) // store RdHi in Imm

	switch op1 {
	case 0x0:
		inst.Op = int(SMULL)
	case 0x2:
		inst.Op = int(UMULL)
	case 0x4:
		inst.Op = int(SMLAL)
	case 0x6:
		inst.Op = int(UMLAL)
	}
}
