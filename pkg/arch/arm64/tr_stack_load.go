package arm64

import (
	"github.com/vmpacker/pkg/vm"
)

// ---- Stack-mode Load translation functions ----

// trStackLoad translates LDR (stack mode)
// LDR Rd, [Rn, #off] → VLOAD(rn) PUSH(off) S_ADD S_LD{8|16|32|64} VSTORE(rd)
func (t *Translator) trStackLoad(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	if inst.Rd >= vm.REG_V_BASE {
		szType := byte(inst.Shift)
		t.sVload(rn)
		if inst.Imm != 0 {
			t.sPushImm(uint64(inst.Imm))
			t.emit(vm.OpSAdd)
		}
		t.emit(vm.OpSVLd, rd, szType)
		return nil
	}

	op := Op(inst.Op)
	var sLdOp byte
	switch op {
	case LDRB_IMM:
		sLdOp = vm.OpSLd8
	case LDRH_IMM:
		sLdOp = vm.OpSLd16
	case LDRSB_IMM:
		sLdOp = vm.OpSLd8
	case LDRSH_IMM:
		sLdOp = vm.OpSLd16
	case LDRSW_IMM:
		sLdOp = vm.OpSLd32
	case LDR_IMM:
		if inst.SF {
			sLdOp = vm.OpSLd64
		} else {
			sLdOp = vm.OpSLd32
		}
	default:
		sLdOp = vm.OpSLd64
	}

	// Writeback helper (pre/post index)
	emitWriteback := func() {
		t.sVload(rn)
		wbImm := inst.Imm
		if wbImm >= 0 {
			t.sPushImm(uint64(wbImm))
			t.emit(vm.OpSAdd)
		} else {
			t.sPushImm(uint64(-wbImm))
			t.emit(vm.OpSSub)
		}
		t.sVstore(rn) // Rn updated
	}

	if inst.WB == 3 {
		// pre-index: Rn += imm first, then load [Rn]
		emitWriteback()
		t.sVload(rn)
		t.emit(sLdOp)
	} else if inst.WB == 1 {
		// post-index: load [Rn], then Rn += imm
		t.sVload(rn)
		t.emit(sLdOp)
		if inst.Rd != vm.REG_XZR {
			t.sVstore(rd)
		} else {
			t.sDrop()
		}
		emitWriteback()
		goto signext
	} else {
		// offset mode
		t.sVload(rn)
		if inst.Imm != 0 {
			if inst.Imm > 0 {
				t.sPushImm(uint64(inst.Imm))
				t.emit(vm.OpSAdd)
			} else {
				t.sPushImm(uint64(-inst.Imm))
				t.emit(vm.OpSSub)
			}
		}
		t.emit(sLdOp)
	}

	// Sign extension
	if op == LDRSW_IMM {
		t.emit(vm.OpSSext32)
	}
	if op == LDRSB_IMM {
		// sext 8→64: push 56, S_SHL, push 56, S_ASR
		t.sPushImm32(56)
		t.emit(vm.OpSShl)
		t.sPushImm32(56)
		t.emit(vm.OpSAsr)
	}
	if op == LDRSH_IMM {
		// sext 16→64: push 48, S_SHL, push 48, S_ASR
		t.sPushImm32(48)
		t.emit(vm.OpSShl)
		t.sPushImm32(48)
		t.emit(vm.OpSAsr)
	}

	if inst.Rd == vm.REG_XZR {
		t.sDrop()
	} else {
		t.sVstore(rd)
	}
	return nil

signext:
	// post-index path: rd already stored, handle sign extension
	if op == LDRSW_IMM || op == LDRSB_IMM || op == LDRSH_IMM {
		t.sVload(rd)
		if op == LDRSW_IMM {
			t.emit(vm.OpSSext32)
		}
		if op == LDRSB_IMM {
			t.sPushImm32(56)
			t.emit(vm.OpSShl)
			t.sPushImm32(56)
			t.emit(vm.OpSAsr)
		}
		if op == LDRSH_IMM {
			t.sPushImm32(48)
			t.emit(vm.OpSShl)
			t.sPushImm32(48)
			t.emit(vm.OpSAsr)
		}
		t.sVstore(rd)
	}
	return nil
}

// trStackLoadReg translates LDR (register offset) — stack mode
// addr = Rn + (shift ? Rm << size : Rm)
func (t *Translator) trStackLoadReg(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	s := (inst.Raw >> 12) & 1
	size := (inst.Raw >> 30) & 3
	shift := uint32(0)
	if s == 1 {
		shift = size
	}

	// addr = Rn + (Rm << shift)
	t.sVload(rn)
	t.sVload(rm)
	if shift > 0 {
		t.sPushImm32(shift)
		t.emit(vm.OpSShl)
	}
	t.emit(vm.OpSAdd) // addr on stack

	op := Op(inst.Op)
	var sLdOp byte
	switch op {
	case LDRB_REG:
		sLdOp = vm.OpSLd8
	case LDRH_REG:
		sLdOp = vm.OpSLd16
	default:
		if inst.SF {
			sLdOp = vm.OpSLd64
		} else {
			sLdOp = vm.OpSLd32
		}
	}

	t.emit(sLdOp)
	t.sVstore(rd)
	return nil
}

// trStackLoadRegSigned translates LDRSB/LDRSH/LDRSW (register offset) — stack mode
// addr = Rn + (Rm << shift), load, sign-extend
func (t *Translator) trStackLoadRegSigned(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	s := (inst.Raw >> 12) & 1
	size := (inst.Raw >> 30) & 3
	shift := uint32(0)
	if s == 1 {
		shift = size
	}

	// addr = Rn + (Rm << shift) on stack
	t.sVload(rn)
	t.sVload(rm)
	if shift > 0 {
		t.sPushImm32(shift)
		t.emit(vm.OpSShl)
	}
	t.emit(vm.OpSAdd)

	// load
	op := Op(inst.Op)
	var sLdOp byte
	var sextBits uint32
	switch op {
	case LDRSB_REG:
		sLdOp = vm.OpSLd8
		sextBits = 56
	case LDRSH_REG:
		sLdOp = vm.OpSLd16
		sextBits = 48
	case LDRSW_REG:
		sLdOp = vm.OpSLd32
		sextBits = 32
	default:
		sLdOp = vm.OpSLd64
		sextBits = 0
	}
	t.emit(sLdOp)

	// sign-extend: SHL sextBits, ASR sextBits
	if sextBits > 0 {
		t.sPushImm32(sextBits)
		t.emit(vm.OpSShl)
		t.sPushImm32(sextBits)
		t.emit(vm.OpSAsr)
	}

	t.sVstore(rd)
	return nil
}

// trStackLdrLiteral translates LDR literal (PC-relative) — stack mode
// ARM64: LDR Xt/Wt, [PC + imm19*4]
func (t *Translator) trStackLdrLiteral(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	absAddr := uint64(inst.Imm)

	// push absolute address on stack
	t.sPushImm(absAddr)

	op := Op(inst.Op)
	switch {
	case op == LDR_LIT && inst.SF:
		// LDR Xt, [PC+imm] — 64-bit load
		t.emit(vm.OpSLd64)
	case op == LDR_LIT && !inst.SF:
		// LDR Wt, [PC+imm] — 32-bit load
		t.emit(vm.OpSLd32)
		t.emit(vm.OpSTrunc32)
	default:
		// LDRSW literal: load 32-bit, sign-extend to 64-bit
		t.emit(vm.OpSLd32)
		t.sPushImm32(32)
		t.emit(vm.OpSShl)
		t.sPushImm32(32)
		t.emit(vm.OpSAsr)
	}

	t.sVstore(rd)
	return nil
}
