package arm64

import (
	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// Stack-mode translator — converts all ARM64 instructions to pure stack ops
//
// Translation strategy:
//   register-based: emit(OpAdd, rd, rn, rm)
//   stack-based:    VLOAD(rn) → VLOAD(rm) → S_ADD → VSTORE(rd)
//
// Advantage: eliminates register conflicts completely, no need for pickTemp/pickTemp2
// ============================================================

// ---- Stack-mode emit helpers ----

// sVload push R[reg] onto eval stack
func (t *Translator) sVload(reg byte) {
	t.emit(vm.OpSVload, reg)
}

// sVstore pop eval stack → R[reg]
func (t *Translator) sVstore(reg byte) {
	t.emit(vm.OpSVstore, reg)
}

// sPushImm32 push a 32-bit immediate
func (t *Translator) sPushImm32(v uint32) {
	t.emit(vm.OpSPushImm32)
	t.emitU32(v)
}

// sPushImm64 push a 64-bit immediate
func (t *Translator) sPushImm64(v uint64) {
	t.emit(vm.OpSPushImm64)
	t.emitU64(v)
}

// sPushImm push immediate, auto-select 32 vs 64 bit
func (t *Translator) sPushImm(v uint64) {
	if v <= 0xFFFFFFFF {
		t.sPushImm32(uint32(v))
	} else {
		t.sPushImm64(v)
	}
}

// sDup duplicate TOS
func (t *Translator) sDup() { t.emit(vm.OpSDup) }

// sSwap swap TOS and TOS-1
func (t *Translator) sSwap() { t.emit(vm.OpSSwap) }

// sDrop pop and discard TOS
func (t *Translator) sDrop() { t.emit(vm.OpSDrop) }

// ---- Stack-mode ALU translation functions ----

// trStackAluReg translates 3-reg ALU (stack mode)
// ARM64: op Xd, Xn, Xm  →  VLOAD(rn) VLOAD(rm) S_OP VSTORE(rd)
func (t *Translator) trStackAluReg(inst vm.Instruction, sOp byte) error {
	rd, rn, rm, err := t.mapReg3(inst)
	if err != nil {
		return err
	}

	// XZR handling: push 0 instead of VLOAD
	t.pushRegOrZero(inst.Rn, rn)

	// Shift handling
	if inst.Shift != 0 {
		t.pushRegOrZero(inst.Rm, rm)
		t.emitShiftOnStack(inst.ShiftType, uint32(inst.Shift), inst.SF)
	} else {
		t.pushRegOrZero(inst.Rm, rm)
	}

	if sOp == vm.OpSRor {
		if inst.SF {
			t.sPushImm32(64)
		} else {
			t.sPushImm32(32)
		}
	}

	t.emit(sOp) // Binary operation

	if !inst.SF {
		t.emit(vm.OpSTrunc32) // W register mode truncation
	}

	if inst.Rd == vm.REG_XZR {
		t.sDrop() // Discard result
	} else {
		t.sVstore(rd)
	}
	return nil
}

// trStackAluRegFlags translates 3-reg ALU + set flags (stack mode)
func (t *Translator) trStackAluRegFlags(inst vm.Instruction, sOp byte, setFlags bool) error {
	rd, rn, rm, err := t.mapReg3(inst)
	if err != nil {
		return err
	}

	t.pushRegOrZero(inst.Rn, rn)

	if inst.Shift != 0 {
		t.pushRegOrZero(inst.Rm, rm)
		t.emitShiftOnStack(inst.ShiftType, uint32(inst.Shift), inst.SF)
	} else {
		t.pushRegOrZero(inst.Rm, rm)
	}

	t.emit(sOp)

	if setFlags {
		t.sDup()          // duplicate result for CMP
		t.sPushImm32(0)   // push 0
		t.emit(vm.OpSCmp) // compare result with 0 → set flags
	}

	if !inst.SF {
		t.emit(vm.OpSTrunc32)
	}

	if inst.Rd == vm.REG_XZR {
		t.sDrop()
	} else {
		t.sVstore(rd)
	}
	return nil
}

// trStackAluImm translates reg+imm ALU (stack mode)
// ARM64: op Xd, Xn, #imm  →  VLOAD(rn) PUSH(imm) S_OP VSTORE(rd)
func (t *Translator) trStackAluImm(inst vm.Instruction, sOp byte) error {
	return t.trStackAluImmFlags(inst, sOp, false)
}

// trStackAluImmFlags translates reg+imm ALU + flags (stack mode)
func (t *Translator) trStackAluImmFlags(inst vm.Instruction, sOp byte, setFlags bool) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	t.pushRegOrZero(inst.Rn, rn)
	t.sPushImm(uint64(inst.Imm))

	t.emit(sOp)

	if setFlags {
		t.sDup()
		t.sPushImm32(0)
		t.emit(vm.OpSCmp)
	}

	if !inst.SF {
		t.emit(vm.OpSTrunc32)
	}

	if inst.Rd == vm.REG_XZR {
		t.sDrop()
	} else {
		t.sVstore(rd)
	}
	return nil
}

// trStackUnary translates unary op (stack mode)
// ARM64: op Xd, Xn  →  VLOAD(rn) S_OP VSTORE(rd)
func (t *Translator) trStackUnary(inst vm.Instruction, sOp byte) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	t.sVload(rn)
	t.emit(sOp)

	if !inst.SF {
		t.emit(vm.OpSTrunc32)
	}

	t.sVstore(rd)
	return nil
}

// ---- Stack-mode MOV translation functions ----

// trStackMov translates MOV (stack mode)
// MOVZ: Xd = imm << shift
// MOVN: Xd = ~(imm << shift)
func (t *Translator) trStackMov(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	imm := uint64(inst.Imm) << uint64(inst.Shift)
	t.sPushImm(imm)

	if !inst.SF {
		t.emit(vm.OpSTrunc32)
	}

	t.sVstore(rd)
	return nil
}

// trStackMovN translates MOVN (stack mode)
func (t *Translator) trStackMovN(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	val := uint64(inst.Imm) << uint64(inst.Shift)
	val = ^val
	if !inst.SF {
		val &= 0xFFFFFFFF
	}
	t.sPushImm(val)
	t.sVstore(rd)
	return nil
}

// trStackMovK translates MOVK (stack mode)
// Keep other Rd fields, only replace specified 16-bit segment
func (t *Translator) trStackMovK(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	hw := uint64(inst.Shift) // 0, 16, 32, 48
	imm := uint64(inst.Imm)
	mask := uint64(0xFFFF) << hw // 16-bit region to clear

	// Rd = (Rd & ~mask) | (imm << hw)
	t.sVload(rd)          // push Rd
	t.sPushImm(^mask)     // push ~mask
	t.emit(vm.OpSAnd)     // Rd & ~mask
	t.sPushImm(imm << hw) // push (imm << hw)
	t.emit(vm.OpSOr)      // (Rd & ~mask) | (imm << hw)

	if !inst.SF {
		t.emit(vm.OpSTrunc32)
	}

	t.sVstore(rd)
	return nil
}

// ---- Stack-mode CMP translation functions ----

// trStackCmp translates CMP reg,reg (stack mode)
// CMP Xn, Xm → VLOAD(rn) VLOAD(rm) S_CMP
func (t *Translator) trStackCmp(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	t.pushRegOrZero(inst.Rn, rn)
	t.pushRegOrZero(inst.Rm, rm)
	t.emit(vm.OpSCmp)
	return nil
}

// trStackCmpImm translates CMP reg,#imm (stack mode)
func (t *Translator) trStackCmpImm(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	t.pushRegOrZero(inst.Rn, rn)
	t.sPushImm(uint64(inst.Imm))
	t.emit(vm.OpSCmp)
	return nil
}

// ---- Helper functions ----

// mapReg3 maps Rd/Rn/Rm 3 registers (XZR→16 but no conflict concerns)
func (t *Translator) mapReg3(inst vm.Instruction) (byte, byte, byte, error) {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return 0, 0, 0, err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return 0, 0, 0, err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return 0, 0, 0, err
	}
	return rd, rn, rm, nil
}

// pushRegOrZero push register value, or push 0 if XZR
func (t *Translator) pushRegOrZero(arm64Reg int, vmReg byte) {
	if arm64Reg == vm.REG_XZR {
		t.sPushImm32(0)
	} else {
		t.sVload(vmReg)
	}
}

// emitShiftOnStack performs shift on TOS (for shifted register operands)
// TOS = value to shift, output TOS = shifted value
func (t *Translator) emitShiftOnStack(shiftType int, amount uint32, sf bool) {
	if amount == 0 {
		return
	}

	// 32-bit mode: first truncate to 32 bits
	if !sf {
		t.emit(vm.OpSTrunc32)
	}

	switch shiftType {
	case 0: // LSL
		t.sPushImm32(amount)
		t.emit(vm.OpSShl)
	case 1: // LSR
		t.sPushImm32(amount)
		t.emit(vm.OpSShr)
	case 2: // ASR
		if !sf {
			// 32-bit ASR: need sign extension first
			t.emit(vm.OpSSext32)
			t.sPushImm32(amount)
			t.emit(vm.OpSAsr)
		} else {
			t.sPushImm32(amount)
			t.emit(vm.OpSAsr)
		}
	case 3: // ROR
		if !sf {
			// 32-bit ROR: simulate with SHR+SHL+OR
			shift := amount & 31
			if shift != 0 {
				t.sDup() // dup value
				t.sPushImm32(shift)
				t.emit(vm.OpSShr) // value >> shift
				t.sSwap()         // bring original value up
				t.sPushImm32(32 - shift)
				t.emit(vm.OpSShl) // value << (32-shift)
				t.emit(vm.OpSOr)  // combine
			}
		} else {
			t.sPushImm32(amount)
			t.sPushImm32(64)
			t.emit(vm.OpSRor)
		}
	}

	// 32-bit mode: truncate shift result
	if !sf {
		t.emit(vm.OpSTrunc32)
	}
}
