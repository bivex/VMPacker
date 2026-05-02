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
	case FMOV:
		return 0, t.trFMov(inst)
	case FCMP:
		return 0, t.trFCmp(inst)
	default:
		return 0, fmt.Errorf("unsupported FP instruction: %v", OpName(op))
	}
}
