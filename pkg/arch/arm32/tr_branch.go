package arm32

import (
	"fmt"

	"github.com/vmpacker/pkg/vm"
)

// ARM32 branch instruction translation.

// pcOffset returns the PC offset for ARM32 pipeline:
// ARM mode: PC = instruction address + 8
// Thumb mode: PC = instruction address + 4
func (t *Translator) pcOffset() int {
	if t.thumbMode {
		return 4
	}
	return 8
}

// trCondBranch translates B (unconditional or conditional)
func (t *Translator) trCondBranch(inst vm.Instruction) error {
	target := inst.Offset + int(inst.Imm) + t.pcOffset()

	if inst.Cond == COND_AL || inst.Cond < 0 {
		if target < 0 || target > t.funcSize {
			// Tail call: unconditional B to external address (after POP {regs,LR})
			absTarget := uint64(int64(t.funcAddr) + int64(inst.Offset) + inst.Imm + int64(t.pcOffset()))
			t.emit(vm.OpCallNative)
			t.emitU64(absTarget)
			t.emit(vm.OpRet, 0)
			return nil
		}
		t.emit(vm.OpJmp)
		fixPos := t.pos()
		t.emitU32(0)
		t.fixups = append(t.fixups, branchFixup{vmOffset: fixPos, arm32Target: target})
		return nil
	}

	// Conditional branch
	if target < 0 || target > t.funcSize {
		// Conditional tail call: B.cond to external address
		absTarget := uint64(int64(t.funcAddr) + int64(inst.Offset) + inst.Imm + int64(t.pcOffset()))
		skipPos, needsFix := t.emitCondCheck(inst.Cond)
		t.emit(vm.OpCallNative)
		t.emitU64(absTarget)
		t.emit(vm.OpRet, 0)
		if needsFix {
			t.patchCondSkip(skipPos)
		}
		return nil
	}

	vmOp := condToJcc(inst.Cond)
	if vmOp == 0 {
		return fmt.Errorf("unsupported condition code 0x%X for branch", inst.Cond)
	}

	t.emit(vmOp)
	fixPos := t.pos()
	t.emitU32(0)
	t.fixups = append(t.fixups, branchFixup{vmOffset: fixPos, arm32Target: target})
	return nil
}

// trCondBL translates BL (branch with link)
func (t *Translator) trCondBL(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	target := uint64(int64(t.funcAddr) + int64(inst.Offset) + inst.Imm + int64(t.pcOffset()))
	t.emit(vm.OpCallNative)
	t.emitU64(target)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondBX translates BX Rm (branch and exchange)
func (t *Translator) trCondBX(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	if inst.Rm == 14 {
		// BX LR = RET
		t.emit(vm.OpRet, 0)
	} else {
		rm, err := t.mapReg(inst.Rm)
		if err != nil {
			return err
		}
		t.emit(vm.OpBrReg, rm)
	}

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondBLX translates BLX Rm (branch with link and exchange)
func (t *Translator) trCondBLX(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	t.emit(vm.OpCallReg, rm)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCBZCBNZ translates CBZ/CBNZ (compare and branch if zero/not-zero).
// isZero=true for CBZ, false for CBNZ.
// Emits: CmpImm Rn, 0 → Je/Jne target
func (t *Translator) trCBZCBNZ(inst vm.Instruction, isZero bool) error {
	// CBZ/CBNZ target is PC-relative, forward-only: target = inst.Offset + 4 + imm
	target := inst.Offset + 4 + int(inst.Imm)

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	if target < 0 || target > t.funcSize {
		// External CBZ/CBNZ: conditional tail call to code past function boundary
		absTarget := uint64(int64(t.funcAddr) + int64(target))
		t.emit(vm.OpCmpImm, rn)
		t.emitU32(0)
		// Skip over call+ret if condition NOT met (invert the condition)
		var skipOp byte
		if isZero {
			skipOp = vm.OpJne // skip if NOT zero
		} else {
			skipOp = vm.OpJe // skip if zero
		}
		t.emit(skipOp)
		skipPos := t.pos()
		t.emitU32(0)
		t.emit(vm.OpCallNative)
		t.emitU64(absTarget)
		t.emit(vm.OpRet, 0)
		t.patchCondSkip(skipPos)
		return nil
	}

	t.emit(vm.OpCmpImm, rn)
	t.emitU32(0)

	var jmpOp byte
	if isZero {
		jmpOp = vm.OpJe
	} else {
		jmpOp = vm.OpJne
	}

	t.emit(jmpOp)
	fixPos := t.pos()
	t.emitU32(0)
	t.fixups = append(t.fixups, branchFixup{vmOffset: fixPos, arm32Target: target})
	return nil
}

// condToJcc maps ARM condition code to VM conditional jump opcode
func condToJcc(cond int) byte {
	switch cond {
	case COND_EQ:
		return vm.OpJe
	case COND_NE:
		return vm.OpJne
	case COND_CS:
		return vm.OpJae
	case COND_CC:
		return vm.OpJb
	case COND_MI:
		return vm.OpJl
	case COND_PL:
		return vm.OpJge
	case COND_VS:
		return vm.OpJvs
	case COND_VC:
		return vm.OpJvc
	case COND_HI:
		return vm.OpJa
	case COND_LS:
		return vm.OpJbe
	case COND_GE:
		return vm.OpJge
	case COND_LT:
		return vm.OpJl
	case COND_GT:
		return vm.OpJgt
	case COND_LE:
		return vm.OpJle
	default:
		return 0
	}
}
