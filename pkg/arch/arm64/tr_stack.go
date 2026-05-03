package arm64

import (
	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// 栈模式翻译器 — 所有 ARM64 指令转为纯栈操作
//
// 翻译策略:
//   register-based: emit(OpAdd, rd, rn, rm)
//   stack-based:    VLOAD(rn) → VLOAD(rm) → S_ADD → VSTORE(rd)
//
// 优势: 彻底消除寄存器冲突，无需 pickTemp/pickTemp2
// ============================================================

// ---- 栈模式 emit 辅助函数 ----

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

// ---- 栈模式 ALU 翻译函数 ----

// trStackAluReg 翻译三寄存器 ALU (栈模式)
// ARM64: op Xd, Xn, Xm  →  VLOAD(rn) VLOAD(rm) S_OP VSTORE(rd)
func (t *Translator) trStackAluReg(inst vm.Instruction, sOp byte) error {
	rd, rn, rm, err := t.mapReg3(inst)
	if err != nil {
		return err
	}

	// XZR 处理: push 0 而不是 VLOAD
	t.pushRegOrZero(inst.Rn, rn)

	// 移位处理
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

	t.emit(sOp) // 二元操作

	if !inst.SF {
		t.emit(vm.OpSTrunc32) // W 寄存器模式截断
	}

	if inst.Rd == vm.REG_XZR {
		t.sDrop() // 结果丢弃
	} else {
		t.sVstore(rd)
	}
	return nil
}

// trStackAluRegFlags 翻译三寄存器 ALU + 设置标志位 (栈模式)
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

// trStackAluImm 翻译寄存器+立即数 ALU (栈模式)
// ARM64: op Xd, Xn, #imm  →  VLOAD(rn) PUSH(imm) S_OP VSTORE(rd)
func (t *Translator) trStackAluImm(inst vm.Instruction, sOp byte) error {
	return t.trStackAluImmFlags(inst, sOp, false)
}

// trStackAluImmFlags 翻译寄存器+立即数 ALU + 标志位 (栈模式)
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

// trStackUnary 翻译一元操作 (栈模式)
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

// ---- 栈模式 MOV 翻译函数 ----

// trStackMov 翻译 MOV (栈模式)
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

// trStackMovN 翻译 MOVN (栈模式)
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

// trStackMovK 翻译 MOVK (栈模式)
// 保留 Rd 其他字段，仅替换指定 16-bit 段
func (t *Translator) trStackMovK(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	hw := uint64(inst.Shift) // 0, 16, 32, 48
	imm := uint64(inst.Imm)
	mask := uint64(0xFFFF) << hw // 要清除的 16-bit 区域

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

// ---- 栈模式 CMP 翻译函数 ----

// trStackCmp 翻译 CMP reg,reg (栈模式)
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

// trStackCmpImm 翻译 CMP reg,#imm (栈模式)
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

// ---- 辅助工具函数 ----

// mapReg3 映射 Rd/Rn/Rm 三寄存器 (XZR→16 但不再有冲突顾虑)
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

// emitShiftOnStack 在栈顶值上执行移位操作 (用于 shifted register operands)
// TOS = value to shift, 输出 TOS = shifted value
func (t *Translator) emitShiftOnStack(shiftType int, amount uint32, sf bool) {
	if amount == 0 {
		return
	}

	// 32-bit 模式: 先截断到 32 位
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
			// 32-bit ASR: 需要先符号扩展
			t.emit(vm.OpSSext32)
			t.sPushImm32(amount)
			t.emit(vm.OpSAsr)
		} else {
			t.sPushImm32(amount)
			t.emit(vm.OpSAsr)
		}
	case 3: // ROR
		if !sf {
			// 32-bit ROR: 用 SHR+SHL+OR 模拟
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

	// 32-bit 模式: 截断移位结果
	if !sf {
		t.emit(vm.OpSTrunc32)
	}
}
