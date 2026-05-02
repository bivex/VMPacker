package arm32

import "github.com/vmpacker/pkg/vm"

// Thumb (16-bit) instruction decoder.
// Called from decoder.go when thumbMode=true and the halfword is NOT a Thumb-2 prefix.

// decodeThumb dispatches to 16-bit or 32-bit Thumb decoder
func (d *Decoder) decodeThumb(raw uint32, offset int) vm.Instruction {
	hw1 := uint16(raw >> 16)
	if raw <= 0xFFFF {
		hw1 = uint16(raw)
	}

	inst := vm.Instruction{Raw: raw, Op: int(UNKNOWN), Offset: offset, Rd: -1, Rn: -1, Rm: -1, Cond: COND_AL}

	// Apply IT block condition if active
	if d.itState.count > 0 {
		inst.Cond = d.itCondForCurrent()
		d.itAdvance()
	}

	if raw > 0xFFFF && IsThumb32(hw1) {
		d.decodeThumb32Inst(raw, &inst)
	} else {
		d.decodeThumb16Inst(uint16(raw&0xFFFF), &inst)
	}

	if inst.Op == int(UNKNOWN) {
		inst.Op = int(UNSUPPORTED)
	}
	return inst
}

// itCondForCurrent returns the condition code for the current IT block instruction
func (d *Decoder) itCondForCurrent() int {
	if d.itState.count <= 0 {
		return COND_AL
	}
	// Top bit of mask determines if condition is inverted
	topBit := (d.itState.mask >> 3) & 1
	if topBit == 0 {
		return d.itState.baseCond
	}
	return d.itState.baseCond ^ 1
}

// itAdvance advances the IT block state
func (d *Decoder) itAdvance() {
	d.itState.count--
	d.itState.mask = (d.itState.mask << 1) & 0xF
}

func (d *Decoder) decodeThumb16Inst(hw uint16, inst *vm.Instruction) {
	op := hw >> 10

	switch {
	case hw>>8 == 0xBF && hw&0xFF == 0x00:
		// NOP: 1011:1111:0000:0000
		inst.Op = int(NOP)

	case hw>>8 == 0xBF && (hw&0x0F) != 0:
		// IT: 1011:1111:firstcond:mask
		inst.Op = int(IT)
		firstCond := int((hw >> 4) & 0xF)
		mask := int(hw & 0xF)
		inst.Cond = firstCond
		inst.Imm = int64(mask)
		// Set up IT block state
		d.itState.baseCond = firstCond
		d.itState.mask = byte(mask)
		count := 0
		for i := 3; i >= 0; i-- {
			if mask&(1<<uint(i)) != 0 {
				count = 4 - i
				break
			}
		}
		d.itState.count = count

	case op>>4 == 0:
		// Shift/add/sub/mov/cmp (bits[15:14] = 00)
		d.decodeThumb16ShiftAddSubMovCmp(hw, inst)

	case hw>>10 == 0x10:
		// Data processing (bits[15:10] = 010000)
		d.decodeThumb16DataProc(hw, inst)

	case hw>>10 == 0x11:
		// Special + BX (bits[15:10] = 010001)
		d.decodeThumb16Special(hw, inst)

	case hw>>11 == 0x09:
		// LDR (PC-relative): 01001:Rd:imm8
		rd := int((hw >> 8) & 0x7)
		imm := int64(hw&0xFF) * 4
		inst.Op = int(LDR_IMM)
		inst.Rd = rd
		inst.Rn = 15 // PC
		inst.Imm = imm

	case hw>>12 == 0x5:
		// Load/store register offset: 0101:opc:Rm:Rn:Rd
		d.decodeThumb16LdStReg(hw, inst)

	case hw>>13 == 0x3:
		// Load/store word/byte immediate: 011:B:L:imm5:Rn:Rd
		d.decodeThumb16LdStImm(hw, inst)

	case hw>>12 == 0x8:
		// Load/store halfword immediate: 1000:L:imm5:Rn:Rd
		l := (hw >> 11) & 1
		imm := int64((hw>>6)&0x1F) * 2
		rn := int((hw >> 3) & 0x7)
		rd := int(hw & 0x7)
		if l == 1 {
			inst.Op = int(LDRH_IMM)
		} else {
			inst.Op = int(STRH_IMM)
		}
		inst.Rd = rd
		inst.Rn = rn
		inst.Imm = imm

	case hw>>12 == 0x9:
		// Load/store SP-relative: 1001:L:Rd:imm8
		l := (hw >> 11) & 1
		rd := int((hw >> 8) & 0x7)
		imm := int64(hw&0xFF) * 4
		if l == 1 {
			inst.Op = int(LDR_IMM)
		} else {
			inst.Op = int(STR_IMM)
		}
		inst.Rd = rd
		inst.Rn = 13 // SP
		inst.Imm = imm

	case hw>>12 == 0xA:
		// ADR / ADD (SP + imm): 1010:S:Rd:imm8
		s := (hw >> 11) & 1
		rd := int((hw >> 8) & 0x7)
		imm := int64(hw&0xFF) * 4
		if s == 0 {
			// ADR: Rd = PC + imm
			inst.Op = int(ADR)
			inst.Rd = rd
			inst.Imm = imm
		} else {
			// ADD Rd, SP, #imm
			inst.Op = int(ADD_IMM)
			inst.Rd = rd
			inst.Rn = 13 // SP
			inst.Imm = imm
		}

	case hw>>12 == 0xB:
		// Misc 16-bit instructions
		d.decodeThumb16Misc(hw, inst)

	case hw>>12 == 0xC:
		// LDM/STM: 1100:L:Rn:reglist
		d.decodeThumb16LdmStm(hw, inst)

	case hw>>12 == 0xD:
		// Conditional branch / SVC
		cond := int((hw >> 8) & 0xF)
		if cond == 0xE {
			// UDF (permanently undefined instruction) — used as trap
			inst.Op = int(BKPT)
		} else if cond == 0xF {
			// SVC
			inst.Op = int(SVC)
			inst.Imm = int64(hw & 0xFF)
		} else {
			// B<cond>: 1101:cond:imm8
			inst.Op = int(B)
			inst.Cond = cond
			inst.Imm = SignExtend(uint32(hw&0xFF), 8) * 2
		}

	case hw>>11 == 0x1C:
		// Unconditional branch: 11100:imm11
		inst.Op = int(B)
		inst.Imm = SignExtend(uint32(hw&0x7FF), 11) * 2
	}
}

func (d *Decoder) decodeThumb16ShiftAddSubMovCmp(hw uint16, inst *vm.Instruction) {
	op := (hw >> 11) & 0x1F

	switch {
	case op <= 0x01:
		// LSL(imm): 00000:imm5:Rm:Rd
		imm := int((hw >> 6) & 0x1F)
		rm := int((hw >> 3) & 0x7)
		rd := int(hw & 0x7)
		inst.Op = int(MOV_REG)
		inst.Rd = rd
		inst.Rm = rm
		inst.Shift = imm
		inst.ShiftType = 0 // LSL

	case op == 0x02:
		// LSR(imm): 00001:imm5:Rm:Rd
		imm := int((hw >> 6) & 0x1F)
		if imm == 0 {
			imm = 32
		}
		rm := int((hw >> 3) & 0x7)
		rd := int(hw & 0x7)
		inst.Op = int(MOV_REG)
		inst.Rd = rd
		inst.Rm = rm
		inst.Shift = imm
		inst.ShiftType = 1 // LSR

	case op == 0x03:
		// ASR(imm): 00010:imm5:Rm:Rd
		imm := int((hw >> 6) & 0x1F)
		if imm == 0 {
			imm = 32
		}
		rm := int((hw >> 3) & 0x7)
		rd := int(hw & 0x7)
		inst.Op = int(MOV_REG)
		inst.Rd = rd
		inst.Rm = rm
		inst.Shift = imm
		inst.ShiftType = 2 // ASR

	case (hw>>9)&0x7F == 0x0C:
		// ADD(reg) T1: 0001100:Rm:Rn:Rd
		rm := int((hw >> 6) & 0x7)
		rn := int((hw >> 3) & 0x7)
		rd := int(hw & 0x7)
		inst.Op = int(ADD_REG)
		inst.Rd = rd
		inst.Rn = rn
		inst.Rm = rm

	case (hw>>9)&0x7F == 0x0D:
		// SUB(reg) T1: 0001101:Rm:Rn:Rd
		rm := int((hw >> 6) & 0x7)
		rn := int((hw >> 3) & 0x7)
		rd := int(hw & 0x7)
		inst.Op = int(SUB_REG)
		inst.Rd = rd
		inst.Rn = rn
		inst.Rm = rm

	case (hw>>9)&0x7F == 0x0E:
		// ADD(imm3) T1: 0001110:imm3:Rn:Rd
		imm := int64((hw >> 6) & 0x7)
		rn := int((hw >> 3) & 0x7)
		rd := int(hw & 0x7)
		inst.Op = int(ADD_IMM)
		inst.Rd = rd
		inst.Rn = rn
		inst.Imm = imm

	case (hw>>9)&0x7F == 0x0F:
		// SUB(imm3) T1: 0001111:imm3:Rn:Rd
		imm := int64((hw >> 6) & 0x7)
		rn := int((hw >> 3) & 0x7)
		rd := int(hw & 0x7)
		inst.Op = int(SUB_IMM)
		inst.Rd = rd
		inst.Rn = rn
		inst.Imm = imm

	case op == 0x04:
		// MOV(imm) T1: 00100:Rd:imm8
		rd := int((hw >> 8) & 0x7)
		inst.Op = int(MOV_IMM)
		inst.Rd = rd
		inst.Imm = int64(hw & 0xFF)

	case op == 0x05:
		// CMP(imm) T1: 00101:Rn:imm8
		rn := int((hw >> 8) & 0x7)
		inst.Op = int(CMP_IMM)
		inst.Rn = rn
		inst.Imm = int64(hw & 0xFF)

	case op == 0x06:
		// ADD(imm8) T2: 00110:Rd:imm8 (Rd = Rn)
		rd := int((hw >> 8) & 0x7)
		inst.Op = int(ADD_IMM)
		inst.Rd = rd
		inst.Rn = rd
		inst.Imm = int64(hw & 0xFF)

	case op == 0x07:
		// SUB(imm8) T2: 00111:Rd:imm8
		rd := int((hw >> 8) & 0x7)
		inst.Op = int(SUB_IMM)
		inst.Rd = rd
		inst.Rn = rd
		inst.Imm = int64(hw & 0xFF)
	}
}

func (d *Decoder) decodeThumb16DataProc(hw uint16, inst *vm.Instruction) {
	op := (hw >> 6) & 0xF
	rm := int((hw >> 3) & 0x7)
	rd := int(hw & 0x7)

	inst.Rd = rd
	inst.Rn = rd
	inst.Rm = rm

	switch op {
	case 0x0:
		inst.Op = int(AND_REG)
	case 0x1:
		inst.Op = int(EOR_REG)
	case 0x2:
		inst.Op = int(LSL_REG)
		inst.Rn = rd
	case 0x3:
		inst.Op = int(LSR_REG)
		inst.Rn = rd
	case 0x4:
		inst.Op = int(ASR_REG)
		inst.Rn = rd
	case 0x5:
		inst.Op = int(ADC_REG)
	case 0x6:
		inst.Op = int(SBC_REG)
	case 0x7:
		inst.Op = int(ROR_REG)
		inst.Rn = rd
	case 0x8:
		inst.Op = int(TST_REG)
		inst.Rd = -1
	case 0x9:
		// NEG (RSB Rd, Rm, #0)
		inst.Op = int(RSB_IMM)
		inst.Rn = rm
		inst.Imm = 0
	case 0xA:
		inst.Op = int(CMP_REG)
		inst.Rn = rd
		inst.Rd = -1
	case 0xB:
		inst.Op = int(CMN_REG)
		inst.Rn = rd
		inst.Rd = -1
	case 0xC:
		inst.Op = int(ORR_REG)
	case 0xD:
		inst.Op = int(MUL)
		inst.Rn = rm
	case 0xE:
		inst.Op = int(BIC_REG)
	case 0xF:
		inst.Op = int(MVN_REG)
		inst.Rn = -1
	}
}

func (d *Decoder) decodeThumb16Special(hw uint16, inst *vm.Instruction) {
	op := (hw >> 8) & 0x3
	switch op {
	case 0x0:
		// ADD(reg) T2: high registers — 01000100:D:Rm:Rd
		dn := int((hw>>7)&1)<<3 | int(hw&0x7)
		rm := int((hw >> 3) & 0xF)
		inst.Op = int(ADD_REG)
		inst.Rd = dn
		inst.Rn = dn
		inst.Rm = rm
	case 0x1:
		// CMP T2: high registers — 01000101:N:Rm:Rn
		rn := int((hw>>7)&1)<<3 | int(hw&0x7)
		rm := int((hw >> 3) & 0xF)
		inst.Op = int(CMP_REG)
		inst.Rn = rn
		inst.Rm = rm
	case 0x2:
		// MOV T1: high registers — 01000110:D:Rm:Rd
		rd := int((hw>>7)&1)<<3 | int(hw&0x7)
		rm := int((hw >> 3) & 0xF)
		inst.Op = int(MOV_REG)
		inst.Rd = rd
		inst.Rm = rm
	case 0x3:
		rm := int((hw >> 3) & 0xF)
		if hw&0x80 != 0 {
			// BLX Rm: 01000111:1:Rm:000
			inst.Op = int(BLX_REG)
			inst.Rm = rm
		} else {
			// BX Rm: 01000111:0:Rm:000
			inst.Op = int(BX)
			inst.Rm = rm
		}
	}
}

func (d *Decoder) decodeThumb16LdStReg(hw uint16, inst *vm.Instruction) {
	opc := (hw >> 9) & 0x7
	rm := int((hw >> 6) & 0x7)
	rn := int((hw >> 3) & 0x7)
	rd := int(hw & 0x7)
	inst.Rd = rd
	inst.Rn = rn
	inst.Rm = rm
	inst.Imm = 1 // positive offset flag

	switch opc {
	case 0x0:
		inst.Op = int(STR_REG)
	case 0x1:
		inst.Op = int(STRH_REG)
	case 0x2:
		inst.Op = int(STRB_REG)
	case 0x3:
		inst.Op = int(LDRSB_REG)
	case 0x4:
		inst.Op = int(LDR_REG)
	case 0x5:
		inst.Op = int(LDRH_REG)
	case 0x6:
		inst.Op = int(LDRB_REG)
	case 0x7:
		inst.Op = int(LDRSH_REG)
	}
}

func (d *Decoder) decodeThumb16LdStImm(hw uint16, inst *vm.Instruction) {
	b := (hw >> 12) & 1
	l := (hw >> 11) & 1
	imm5 := int64((hw >> 6) & 0x1F)
	rn := int((hw >> 3) & 0x7)
	rd := int(hw & 0x7)

	inst.Rd = rd
	inst.Rn = rn

	if b == 0 {
		inst.Imm = imm5 * 4 // word access
		if l == 1 {
			inst.Op = int(LDR_IMM)
		} else {
			inst.Op = int(STR_IMM)
		}
	} else {
		inst.Imm = imm5 // byte access
		if l == 1 {
			inst.Op = int(LDRB_IMM)
		} else {
			inst.Op = int(STRB_IMM)
		}
	}
}

func (d *Decoder) decodeThumb16Misc(hw uint16, inst *vm.Instruction) {
	// Dispatch on bits[11:8] within the 1011:xxxx:xxxx:xxxx misc block
	nibble := (hw >> 8) & 0xF

	switch {
	case nibble == 0x0:
		// ADD/SUB SP, #imm7: 1011:0000:x:imm7
		imm := int64(hw&0x7F) * 4
		if hw&0x80 == 0 {
			inst.Op = int(ADD_IMM)
		} else {
			inst.Op = int(SUB_IMM)
		}
		inst.Rd = 13
		inst.Rn = 13
		inst.Imm = imm

	case nibble == 0x1 || nibble == 0x3:
		// CBZ: 1011:00i1:imm5:Rn (bit[11]=0)
		rn := int(hw & 0x7)
		i := (hw >> 9) & 1
		imm5 := (hw >> 3) & 0x1F
		offset := int64((uint32(i)<<5 | uint32(imm5)) << 1)
		inst.Op = int(CBZ)
		inst.Rn = rn
		inst.Imm = offset

	case nibble == 0x2:
		// SXTH/SXTB/UXTH/UXTB: 1011:0010:xx:Rm:Rd
		subop := (hw >> 6) & 0x3
		rm := int((hw >> 3) & 0x7)
		rd := int(hw & 0x7)
		inst.Rd = rd
		inst.Rm = rm
		switch subop {
		case 0:
			inst.Op = int(MOV_REG)
			inst.Imm = 16
		case 1:
			inst.Op = int(MOV_REG)
			inst.Imm = 8
		case 2:
			inst.Op = int(MOV_REG)
			inst.Imm = -16
		case 3:
			inst.Op = int(MOV_REG)
			inst.Imm = -8
		}

	case nibble == 0x4 || nibble == 0x5:
		// PUSH: 1011:010M:reglist
		reglist := int64(hw & 0xFF)
		if hw&0x100 != 0 {
			reglist |= 1 << 14 // LR
		}
		inst.Op = int(STM)
		inst.Rn = 13
		inst.Imm = reglist
		inst.ShiftType = 3 // DB (STMDB = PUSH)
		inst.WB = 1

	case nibble == 0x9 || nibble == 0xB:
		// CBNZ: 1011:10i1:imm5:Rn (bit[11]=1)
		rn := int(hw & 0x7)
		i := (hw >> 9) & 1
		imm5 := (hw >> 3) & 0x1F
		offset := int64((uint32(i)<<5 | uint32(imm5)) << 1)
		inst.Op = int(CBNZ)
		inst.Rn = rn
		inst.Imm = offset

	case nibble == 0xA:
		// REV/REV16/REVSH: 1011:1010:xx:Rm:Rd
		subop := (hw >> 6) & 0x3
		rm := int((hw >> 3) & 0x7)
		rd := int(hw & 0x7)
		inst.Rd = rd
		inst.Rm = rm
		switch subop {
		case 0:
			inst.Op = int(REV)
		case 1:
			inst.Op = int(REV16)
		case 3:
			inst.Op = int(REV16) // REVSH — treat as signed REV16
		default:
			inst.Op = int(UNSUPPORTED)
		}

	case nibble == 0xC || nibble == 0xD:
		// POP: 1011:110P:reglist
		reglist := int64(hw & 0xFF)
		if hw&0x100 != 0 {
			reglist |= 1 << 15 // PC
		}
		inst.Op = int(LDM)
		inst.Rn = 13
		inst.Imm = reglist
		inst.ShiftType = 0 // IA (LDMIA = POP)
		inst.WB = 1

	case nibble == 0xE:
		// BKPT #imm8
		inst.Op = int(BKPT)
		inst.Imm = int64(hw & 0xFF)
	}
}

func (d *Decoder) decodeThumb16LdmStm(hw uint16, inst *vm.Instruction) {
	l := (hw >> 11) & 1
	rn := int((hw >> 8) & 0x7)
	reglist := int64(hw & 0xFF)

	inst.Rn = rn
	inst.Imm = reglist
	inst.ShiftType = 0 // IA
	inst.WB = 1

	if l == 1 {
		inst.Op = int(LDM)
	} else {
		inst.Op = int(STM)
	}
}
