package arm32

import "github.com/vmpacker/pkg/vm"

// trCondMovImm translates MOV Rd, #imm
func (t *Translator) trCondMovImm(inst vm.Instruction, setFlags bool) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	t.sPushImm(uint64(uint32(inst.Imm)))
	t.emitFlagsIfNeeded(setFlags)
	t.sVstore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsIfNeeded(setFlags)
	t.sVstore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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

	t.patchIfNeeded(skipPos, needsFix)
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

	t.patchIfNeeded(skipPos, needsFix)
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
		t.patchIfNeeded(skipPos, needsFix)
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

	t.emitFlagsIfNeeded(setFlags)
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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

	t.emitFlagsIfNeeded(setFlags)
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
	return nil
}
