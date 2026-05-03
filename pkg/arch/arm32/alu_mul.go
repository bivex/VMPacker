package arm32

import "github.com/vmpacker/pkg/vm"

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
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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

	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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

	t.patchIfNeeded(skipPos, needsFix)
	return nil
}
