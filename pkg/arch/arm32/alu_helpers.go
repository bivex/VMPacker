package arm32

import "github.com/vmpacker/pkg/vm"

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

// emitTruncAndStore emits Trunc32 and stores to rd.
func (t *Translator) emitTruncAndStore(rd byte) {
	t.emitTrunc32()
	t.sVstore(rd)
}

// emitFlagsIfNeeded emits flag-setting ops if setFlags is true.
// Expects the result value on top of stack; leaves result on top.
func (t *Translator) emitFlagsIfNeeded(setFlags bool) {
	if setFlags {
		t.sDup()
		t.sPushImm32(0)
		t.emit(vm.OpSCmp)
	}
}

// emitFlagsForCmp emits flag-setting for comparison ops (result discarded).
func (t *Translator) emitFlagsForCmp() {
	t.sPushImm32(0)
	t.emit(vm.OpSCmp)
	t.sDrop()
}

// withCondition wraps fn with conditional execution check.
// Returns skipPos, needsFix from emitCondCheck.
func (t *Translator) withCondition(inst vm.Instruction, fn func()) (int, bool) {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)
	fn()
	return skipPos, needsFix
}

// patchIfNeeded patches the conditional skip if needed.
func (t *Translator) patchIfNeeded(skipPos int, needsFix bool) {
	if needsFix {
		t.patchCondSkip(skipPos)
	}
}
