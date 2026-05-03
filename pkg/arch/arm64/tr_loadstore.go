package arm64

import (
	"encoding/binary"

	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// Load/Store - only special format instructions that can't use stack mode
// Acquire/Release instructions - simple semantics, no temp register involvement
// ============================================================

// trLdar translates LDAR/LDARB/LDARH/LDAXR/LDAXRB/LDAXRH
// In single-threaded VM equivalent to normal load from [Rn] with offset=0
func (t *Translator) trLdar(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	var ldOp byte
	switch inst.Shift {
	case 1:
		ldOp = vm.OpLoad8
	case 2:
		ldOp = vm.OpLoad16
	case 4:
		ldOp = vm.OpLoad32
	default:
		ldOp = vm.OpLoad64
	}

	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 0)
	t.emit(ldOp, rd, rn)
	t.code = append(t.code, b...)
	return nil
}

// trStlr translates STLR/STLRB/STLRH
// In single-threaded VM equivalent to normal store to [Rn] with offset=0
func (t *Translator) trStlr(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	var stOp byte
	switch inst.Shift {
	case 1:
		stOp = vm.OpStore8
	case 2:
		stOp = vm.OpStore16
	case 4:
		stOp = vm.OpStore32
	default:
		stOp = vm.OpStore64
	}

	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 0)
	t.emit(stOp, rn, rd)
	t.code = append(t.code, b...)
	return nil
}

// trStlxr translates STLXR/STLXRB/STLXRH
// In single-threaded VM: store + status register = 0 (always succeeds)
func (t *Translator) trStlxr(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rt, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rs, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	var stOp byte
	switch inst.Shift {
	case 1:
		stOp = vm.OpStore8
	case 2:
		stOp = vm.OpStore16
	case 4:
		stOp = vm.OpStore32
	default:
		stOp = vm.OpStore64
	}

	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, 0)
	t.emit(stOp, rn, rt)
	t.code = append(t.code, b...)

	// status = 0 (always succeed in single-threaded VM)
	t.emit(vm.OpMovImm32, rs)
	t.emitU32(0)
	return nil
}
