package arm64

import (
	"fmt"

	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// FP / SIMD 翻译 — 基础 ALU
// ============================================================

func (t *Translator) trFAddSub(inst vm.Instruction, vmOp byte) error {
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

	fpType := byte(0) // float
	if inst.SF {
		fpType = 1 // double
	}

	t.emit(vmOp, rd, rn, rm, fpType)
	return nil
}

func (t *Translator) trFMov(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	fpType := byte(0)
	if inst.SF {
		fpType = 1
	}

	t.emit(vm.OpSFMov, rd, rn, fpType)
	return nil
}

func (t *Translator) trFCmp(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	fpType := byte(0)
	if inst.SF {
		fpType = 1
	}

	t.emit(vm.OpSFCmp, rn, rm, fpType)
	return nil
}

func (t *Translator) trFUnary(inst vm.Instruction, vmOp byte) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	fpType := byte(0)
	if inst.SF {
		fpType = 1
	}

	t.emit(vmOp, rd, rn, fpType)
	return nil
}

func (t *Translator) trFCvt(inst vm.Instruction, vmOp byte) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	// Combined type byte: bits[7:1]=reserved, bit 1: SF, bit 0: FPType
	// SF: 0=32-bit int, 1=64-bit int
	// FPType: 0=float, 1=double
	typeByte := byte(0)
	if inst.SF {
		typeByte |= 0x2
	}
	// Note: for conversions, type bit 22 from ARM64 often matches our FPType.
	// But let's check the inst.Cond or other field if needed.
	// For SCVTF: bit 22 is 'type'.
	raw := inst.Raw
	if (raw>>22)&1 != 0 {
		typeByte |= 0x1
	}

	t.emit(vmOp, rd, rn, typeByte)
	return nil
}

func (t *Translator) trFMoveRV(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	typeByte := byte(0)
	if inst.SF {
		typeByte = 1
	}

	t.emit(vm.OpSFMovRV, rd, rn, typeByte)
	return nil
}

// 代理方法到 translator.go dispatch
func (t *Translator) translateFP(inst vm.Instruction) (int, error) {
	op := Op(inst.Op)
	switch op {
	case FADD:
		return 0, t.trFAddSub(inst, vm.OpSFAdd)
	case FSUB:
		return 0, t.trFAddSub(inst, vm.OpSFSub)
	case FMUL:
		return 0, t.trFAddSub(inst, vm.OpSFMul)
	case FDIV:
		return 0, t.trFAddSub(inst, vm.OpSFDiv)
	case FMAX:
		return 0, t.trFAddSub(inst, vm.OpSFMax)
	case FMIN:
		return 0, t.trFAddSub(inst, vm.OpSFMin)
	case FMOV:
		// Check if it's general-to-SIMD move (like DUP scalar)
		if inst.Rd >= vm.REG_V_BASE && inst.Rn < vm.REG_V_BASE {
			return 0, t.trFMoveRV(inst)
		}
		return 0, t.trFMov(inst)
	case FCMP:
		return 0, t.trFCmp(inst)
	case FNEG:
		return 0, t.trFUnary(inst, vm.OpSFNeg)
	case FABS:
		return 0, t.trFUnary(inst, vm.OpSFAbs)
	case FSQRT:
		return 0, t.trFUnary(inst, vm.OpSFSqrt)
	case FCVT:
		// FCVT float/double conversion
		return 0, t.trFUnary(inst, vm.OpSFCvt)
	case FCVTZS, FCVTZU:
		return 0, t.trFCvt(inst, vm.OpSFCvtFI)
	case SCVTF, UCVTF:
		return 0, t.trFCvt(inst, vm.OpSFCvtIF)
	default:
		return 0, fmt.Errorf("unsupported FP instruction: %v", OpName(op))
	}
}
