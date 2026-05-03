package arm32

import "github.com/vmpacker/pkg/vm"

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

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsForCmp()

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsForCmp()

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsForCmp()

	t.patchIfNeeded(skipPos, needsFix)
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

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsForCmp()

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsForCmp()

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsForCmp()

	t.patchIfNeeded(skipPos, needsFix)
	return nil
}
