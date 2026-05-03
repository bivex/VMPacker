package arm32

import "github.com/vmpacker/pkg/vm"

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
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
	return nil
}

// trCondCLZ32 translates ARM32 CLZ (32-bit count leading zeros).
// The VM's S_CLZ uses __builtin_clzll (64-bit). A 32-bit value v zero-extended
// to u64 gives clzll(v) = clz32(v) + 32. We emit TRUNC32 before CLZ then
// subtract 32 to recover the 32-bit result. CLZ(0) = 64 - 32 = 32, which
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
	t.emitTruncAndStore(rd)

	t.patchIfNeeded(skipPos, needsFix)
	return nil
}
