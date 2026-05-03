package arm32

import "github.com/vmpacker/pkg/vm"

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
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsIfNeeded(true)
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsIfNeeded(true)
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsIfNeeded(setFlags)
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsIfNeeded(setFlags)
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsIfNeeded(setFlags)
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
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
	t.emitFlagsIfNeeded(setFlags)
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
	return nil
}
