package arm32

import (
	"github.com/vmpacker/pkg/vm"
)

// ARM32 ALU instruction translation helpers.
// All ARM32 results are 32-bit; we always emit OpSTrunc32 at the end.

// emitBarrelShifterOnStack pushes the barrel shifter result for a register operand.
// The source register is already loaded onto the eval stack.
// inst.ShiftType: 0=LSL, 1=LSR, 2=ASR, 3=ROR
// inst.Shift: immediate shift amount (inst.Imm == -1 means shift by register Rs)
func (t *Translator) emitBarrelShifterOnStack(inst vm.Instruction) {
	if inst.Imm == -1 {
		// Shift by register (Rs)
		rs := byte(inst.Shift) // Rs register number
		t.sVload(rs)
		t.emitTrunc32()
		switch inst.ShiftType {
		case 0:
			t.emit(vm.OpSShl)
		case 1:
			t.emit(vm.OpSShr)
		case 2:
			t.emit(vm.OpSSext32)
			t.emit(vm.OpSAsr)
		case 3:
			t.emit(vm.OpSRor)
		}
	} else if inst.Shift != 0 {
		shamt := uint32(inst.Shift)
		switch inst.ShiftType {
		case 0: // LSL
			t.sPushImm32(shamt)
			t.emit(vm.OpSShl)
		case 1: // LSR
			t.sPushImm32(shamt)
			t.emit(vm.OpSShr)
		case 2: // ASR
			t.emit(vm.OpSSext32)
			t.sPushImm32(shamt)
			t.emit(vm.OpSAsr)
		case 3: // ROR
			shift := shamt & 31
			if shift != 0 {
				t.sDup()
				t.sPushImm32(shift)
				t.emit(vm.OpSShr)
				t.sSwap()
				t.sPushImm32(32 - shift)
				t.emit(vm.OpSShl)
				t.emit(vm.OpSOr)
			}
		}
	} else if inst.ShiftType == 3 {
		// RRX: shift=0, type=ROR means rotate right through carry (RRX)
		// Simplified: just ROR by 1
		t.sPushImm32(1)
		t.emit(vm.OpSRor)
	}
	t.emitTrunc32()
}

// sVloadOrPC loads a register value onto the eval stack.
// If reg is R15 (PC), pushes (link-time PC value) + slide instead of reading vm->R[15].
// ARM32 pipeline: PC = instruction address + 8 (ARM mode).
func (t *Translator) sVloadOrPC(inst vm.Instruction, armReg int) {
	if armReg == 15 {
		pcVal := uint32(int64(t.funcAddr) + int64(inst.Offset) + int64(t.pcOffset()))
		t.sPushImm32(pcVal)
		t.emit(vm.OpSLoadSlide)
		t.emit(vm.OpSAdd)
		t.emitTrunc32()
	} else {
		t.sVload(byte(armReg))
	}
}

// trCondAluImm translates Rd = Rn OP #imm with condition wrapper
func (t *Translator) trCondAluImm(inst vm.Instruction, sOp byte) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	t.sVloadOrPC(inst, inst.Rn)
	t.sPushImm(uint64(uint32(inst.Imm)))
	t.emit(sOp)
	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondAluImmFlags translates Rd = Rn OP #imm with flags
func (t *Translator) trCondAluImmFlags(inst vm.Instruction, sOp byte) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	t.sVloadOrPC(inst, inst.Rn)
	t.sPushImm(uint64(uint32(inst.Imm)))
	t.emit(sOp)
	t.sDup()
	t.sPushImm32(0)
	t.emit(vm.OpSCmp)
	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondAluReg translates Rd = Rn OP shift(Rm)
func (t *Translator) trCondAluReg(inst vm.Instruction, sOp byte) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	t.sVloadOrPC(inst, inst.Rn)
	t.sVloadOrPC(inst, inst.Rm)
	t.emitBarrelShifterOnStack(inst)
	t.emit(sOp)
	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondAluRegFlags translates Rd = Rn OP shift(Rm) with flags
func (t *Translator) trCondAluRegFlags(inst vm.Instruction, sOp byte) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	t.sVloadOrPC(inst, inst.Rn)
	t.sVloadOrPC(inst, inst.Rm)
	t.emitBarrelShifterOnStack(inst)
	t.emit(sOp)
	t.sDup()
	t.sPushImm32(0)
	t.emit(vm.OpSCmp)
	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondRSBImm translates RSB Rd, Rn, #imm → Rd = imm - Rn
func (t *Translator) trCondRSBImm(inst vm.Instruction, setFlags bool) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	t.sPushImm(uint64(uint32(inst.Imm)))
	t.sVloadOrPC(inst, inst.Rn)
	t.emit(vm.OpSSub) // imm - Rn

	if setFlags {
		t.sDup()
		t.sPushImm32(0)
		t.emit(vm.OpSCmp)
	}

	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondRSBReg translates RSB Rd, Rn, shift(Rm) → Rd = shift(Rm) - Rn
func (t *Translator) trCondRSBReg(inst vm.Instruction, setFlags bool) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	t.sVloadOrPC(inst, inst.Rm)
	t.emitBarrelShifterOnStack(inst)
	t.sVloadOrPC(inst, inst.Rn)
	t.emit(vm.OpSSub) // shift(Rm) - Rn

	if setFlags {
		t.sDup()
		t.sPushImm32(0)
		t.emit(vm.OpSCmp)
	}

	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondBICImm translates BIC Rd, Rn, #imm → Rd = Rn & ~imm
func (t *Translator) trCondBICImm(inst vm.Instruction, setFlags bool) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	t.sVload(rn)
	t.sPushImm(uint64(^uint32(inst.Imm)))
	t.emit(vm.OpSAnd)

	if setFlags {
		t.sDup()
		t.sPushImm32(0)
		t.emit(vm.OpSCmp)
	}

	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondBICReg translates BIC Rd, Rn, shift(Rm) → Rd = Rn & ~shift(Rm)
func (t *Translator) trCondBICReg(inst vm.Instruction, setFlags bool) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

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

	t.sVload(rn)
	t.sVload(rm)
	t.emitBarrelShifterOnStack(inst)
	t.emit(vm.OpSNot)
	t.emit(vm.OpSAnd)

	if setFlags {
		t.sDup()
		t.sPushImm32(0)
		t.emit(vm.OpSCmp)
	}

	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondMovImm translates MOV Rd, #imm
func (t *Translator) trCondMovImm(inst vm.Instruction, setFlags bool) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	t.sPushImm(uint64(uint32(inst.Imm)))

	if setFlags {
		t.sDup()
		t.sPushImm32(0)
		t.emit(vm.OpSCmp)
	}

	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondMvnImm translates MVN Rd, #imm → Rd = ~imm
func (t *Translator) trCondMvnImm(inst vm.Instruction, setFlags bool) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	t.sPushImm(uint64(^uint32(inst.Imm)))

	if setFlags {
		t.sDup()
		t.sPushImm32(0)
		t.emit(vm.OpSCmp)
	}

	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondMovW translates MOVW Rd, #imm16 (write low 16 bits, zero top)
func (t *Translator) trCondMovW(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	t.sPushImm32(uint32(inst.Imm) & 0xFFFF)
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondMovT translates MOVT Rd, #imm16 (write top 16 bits, keep low)
func (t *Translator) trCondMovT(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	imm16 := uint32(inst.Imm) & 0xFFFF
	t.sVload(rd)
	t.sPushImm(uint64(0xFFFF))
	t.emit(vm.OpSAnd)
	t.sPushImm(uint64(imm16 << 16))
	t.emit(vm.OpSOr)
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondMovReg translates MOV Rd, shift(Rm)
func (t *Translator) trCondMovReg(inst vm.Instruction, setFlags bool) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	// Handle sign/zero extend flags from Thumb SXTH/SXTB/UXTH/UXTB
	if inst.Imm != 0 && inst.Rm >= 0 {
		rm, err := t.mapReg(inst.Rm)
		if err != nil {
			return err
		}
		t.sVload(rm)
		switch inst.Imm {
		case 8: // SXTB
			t.sPushImm32(24)
			t.emit(vm.OpSShl)
			t.sPushImm32(24)
			t.emit(vm.OpSAsr)
		case 16: // SXTH
			t.sPushImm32(16)
			t.emit(vm.OpSShl)
			t.sPushImm32(16)
			t.emit(vm.OpSAsr)
		case -8: // UXTB
			t.sPushImm(0xFF)
			t.emit(vm.OpSAnd)
		case -16: // UXTH
			t.sPushImm(0xFFFF)
			t.emit(vm.OpSAnd)
		}
		t.emitTrunc32()
		t.sVstore(rd)
		if needsFix {
			t.patchCondSkip(skipPos)
		}
		return nil
	}

	if inst.Rm >= 0 {
		rm, err := t.mapReg(inst.Rm)
		if err != nil {
			return err
		}
		t.sVload(rm)
		if inst.Shift != 0 || inst.ShiftType != 0 {
			t.emitBarrelShifterOnStack(inst)
		}
	} else {
		t.sPushImm32(0)
	}

	if setFlags {
		t.sDup()
		t.sPushImm32(0)
		t.emit(vm.OpSCmp)
	}

	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondMvnReg translates MVN Rd, shift(Rm) → Rd = ~shift(Rm)
func (t *Translator) trCondMvnReg(inst vm.Instruction, setFlags bool) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	t.sVload(rm)
	if inst.Shift != 0 || inst.ShiftType != 0 {
		t.emitBarrelShifterOnStack(inst)
	}
	t.emit(vm.OpSNot)

	if setFlags {
		t.sDup()
		t.sPushImm32(0)
		t.emit(vm.OpSCmp)
	}

	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondCmpImm translates CMP Rn, #imm (sets flags)
func (t *Translator) trCondCmpImm(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	t.sVload(rn)
	t.sPushImm(uint64(uint32(inst.Imm)))
	t.emit(vm.OpSCmp)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondCmnImm translates CMN Rn, #imm (flags from Rn + imm)
func (t *Translator) trCondCmnImm(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	t.sVload(rn)
	t.sPushImm(uint64(uint32(inst.Imm)))
	t.emit(vm.OpSAdd)
	t.sPushImm32(0)
	t.emit(vm.OpSCmp)
	t.sDrop()

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondTstImm translates TST Rn, #imm (flags from Rn & imm)
func (t *Translator) trCondTstImm(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	t.sVload(rn)
	t.sPushImm(uint64(uint32(inst.Imm)))
	t.emit(vm.OpSAnd)
	t.sPushImm32(0)
	t.emit(vm.OpSCmp)
	t.sDrop()

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondTeqImm translates TEQ Rn, #imm (flags from Rn ^ imm)
func (t *Translator) trCondTeqImm(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	t.sVload(rn)
	t.sPushImm(uint64(uint32(inst.Imm)))
	t.emit(vm.OpSXor)
	t.sPushImm32(0)
	t.emit(vm.OpSCmp)
	t.sDrop()

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondCmpReg translates CMP Rn, shift(Rm)
func (t *Translator) trCondCmpReg(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	t.sVload(rn)
	t.sVload(rm)
	if inst.Shift != 0 || inst.ShiftType != 0 {
		t.emitBarrelShifterOnStack(inst)
	}
	t.emit(vm.OpSCmp)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondCmnReg translates CMN Rn, shift(Rm)
func (t *Translator) trCondCmnReg(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	t.sVload(rn)
	t.sVload(rm)
	if inst.Shift != 0 || inst.ShiftType != 0 {
		t.emitBarrelShifterOnStack(inst)
	}
	t.emit(vm.OpSAdd)
	t.sPushImm32(0)
	t.emit(vm.OpSCmp)
	t.sDrop()

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondTstReg translates TST Rn, shift(Rm)
func (t *Translator) trCondTstReg(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	t.sVload(rn)
	t.sVload(rm)
	if inst.Shift != 0 || inst.ShiftType != 0 {
		t.emitBarrelShifterOnStack(inst)
	}
	t.emit(vm.OpSAnd)
	t.sPushImm32(0)
	t.emit(vm.OpSCmp)
	t.sDrop()

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondTeqReg translates TEQ Rn, shift(Rm)
func (t *Translator) trCondTeqReg(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	t.sVload(rn)
	t.sVload(rm)
	if inst.Shift != 0 || inst.ShiftType != 0 {
		t.emitBarrelShifterOnStack(inst)
	}
	t.emit(vm.OpSXor)
	t.sPushImm32(0)
	t.emit(vm.OpSCmp)
	t.sDrop()

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondCLZ32 translates ARM32 CLZ (32-bit count leading zeros).
// The VM's S_CLZ uses __builtin_clzll (64-bit). A 32-bit value v zero-extended
// to u64 gives clzll(v) = clz32(v) + 32.  We emit TRUNC32 before CLZ then
// subtract 32 to recover the 32-bit result.  CLZ(0) = 64 - 32 = 32, which
// matches ARM32 spec.
func (t *Translator) trCondCLZ32(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	t.sVload(rm)
	t.emitTrunc32()
	t.emit(vm.OpSClz)
	t.sPushImm32(32)
	t.emit(vm.OpSSub)
	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondUnary translates unary instructions (RBIT, REV, etc.)
func (t *Translator) trCondUnary(inst vm.Instruction, sOp byte) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	t.sVload(rm)
	t.emit(sOp)
	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondMul translates MUL Rd, Rm, Rs
func (t *Translator) trCondMul(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	t.sVload(rm)
	t.sVload(rn)
	t.emit(vm.OpSMul)
	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondMLA translates MLA Rd, Rm, Rs, Ra → Rd = Ra + Rm * Rs
func (t *Translator) trCondMLA(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	ra := byte(inst.Imm)
	t.sVload(ra)
	t.sVload(rm)
	t.sVload(rn)
	t.emit(vm.OpSMul)

	if inst.SF {
		// MLS: Ra - Rm*Rs
		t.emit(vm.OpSSub)
	} else {
		// MLA: Ra + Rm*Rs
		t.emit(vm.OpSAdd)
	}

	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondLongMul translates UMULL/SMULL/UMLAL/SMLAL
// RdHi:RdLo = [accumulate +] Rm * Rn
func (t *Translator) trCondLongMul(inst vm.Instruction, signed bool, accumulate bool) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rdLo, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rdHi := byte(inst.Imm) // RdHi stored in Imm
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	// Push Rm and Rn as 64-bit values
	t.sVload(rm)
	if signed {
		t.emit(vm.OpSSext32)
	} else {
		t.emitTrunc32()
	}
	t.sVload(rn)
	if signed {
		t.emit(vm.OpSSext32)
	} else {
		t.emitTrunc32()
	}
	t.emit(vm.OpSMul) // 64-bit result on stack

	if accumulate {
		// Add existing RdHi:RdLo
		t.sVload(rdHi)
		t.sPushImm32(32)
		t.emit(vm.OpSShl)
		t.sVload(rdLo)
		t.emitTrunc32()
		t.emit(vm.OpSOr) // old 64-bit value
		t.emit(vm.OpSAdd)
	}

	// Split into RdLo and RdHi
	t.sDup()
	t.emitTrunc32()
	t.sVstore(rdLo)

	t.sPushImm32(32)
	t.emit(vm.OpSShr)
	t.emitTrunc32()
	t.sVstore(rdHi)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}
