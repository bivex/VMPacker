package arm64

import (
	"encoding/binary"
	"fmt"

	"github.com/vmpacker/pkg/vm"
)

// ---- Stack-mode misc translation functions ----

// trStackMovReg translates MOV Xd, Xn (stack mode)
func (t *Translator) trStackMovReg(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	if inst.Rn == vm.REG_XZR {
		t.sPushImm32(0)
	} else {
		rn, err := t.mapReg(inst.Rn)
		if err != nil {
			return err
		}
		t.sVload(rn)
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

// trStackCBZ translates CBZ/CBNZ (stack mode)
func (t *Translator) trStackCBZ(inst vm.Instruction, isZero bool) error {
	target := inst.Offset + int(inst.Imm)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	// Pure stack cmp: VLOAD(rd) PUSH(0) S_CMP
	t.sVload(rd)
	t.sPushImm32(0)
	t.emit(vm.OpSCmp)

	var vmOp byte
	if isZero {
		vmOp = vm.OpJe
	} else {
		vmOp = vm.OpJne
	}

	if t.cff {
		t.emitCFFCondBranch(vmOp, target, inst.Offset+4)
		return nil
	}

	t.emit(vmOp)
	fixPos := t.pos()
	t.emitU32(0)
	t.fixups = append(t.fixups, branchFixup{vmOffset: fixPos, arm64Target: target})
	return nil
}

// trStackMADD translates MADD/MSUB (stack mode)
// MADD: Rd = Ra + Rn * Rm
// MSUB: Rd = Ra - Rn * Rm
func (t *Translator) trStackMADD(inst vm.Instruction, isSub bool) error {
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

	// Ra from bits[14:10]
	raIdx := int((inst.Raw >> 10) & 0x1F)

	// push Ra (mapped through regMap for register shuffling)
	if raIdx == 31 || raIdx == vm.REG_XZR {
		t.sPushImm32(0) // XZR
	} else {
		ra, err := t.mapReg(raIdx)
		if err != nil {
			return err
		}
		t.sVload(ra)
	}

	// push Rn * Rm
	t.pushRegOrZero(inst.Rn, rn)
	t.pushRegOrZero(inst.Rm, rm)
	t.emit(vm.OpSMul)

	if isSub {
		t.emit(vm.OpSSub) // Ra - (Rn*Rm)
	} else {
		t.emit(vm.OpSAdd) // Ra + (Rn*Rm)
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

// trStackCSEL translates CSEL/CSINC/CSINV/CSNEG (stack mode)
func (t *Translator) trStackCSEL(inst vm.Instruction) error {
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

	// Condition code mapping
	var vmOp byte
	switch inst.Cond {
	case COND_EQ:
		vmOp = vm.OpJe
	case COND_NE:
		vmOp = vm.OpJne
	case COND_LT:
		vmOp = vm.OpJl
	case COND_GE:
		vmOp = vm.OpJge
	case COND_GT:
		vmOp = vm.OpJgt
	case COND_LE:
		vmOp = vm.OpJle
	case COND_CS:
		vmOp = vm.OpJae
	case COND_CC:
		vmOp = vm.OpJb
	case COND_HI:
		vmOp = vm.OpJa
	case COND_LS:
		vmOp = vm.OpJbe
	case COND_MI:
		vmOp = vm.OpJl
	case COND_PL:
		vmOp = vm.OpJge
	default:
		return fmt.Errorf("CSEL: unsupported condition code 0x%X", inst.Cond)
	}

	// CSEL branch logic cannot be rewritten with stack ops (uses VM branch)
	// But XZR handling changed to stack mode push 0
	if inst.Rn == vm.REG_XZR {
		t.sPushImm32(0)
		t.sVstore(rn)
	}
	if inst.Rm == vm.REG_XZR {
		t.sPushImm32(0)
		t.sVstore(rm)
	}

	// Conditional jump to true path
	t.emit(vmOp)
	jccPos := t.pos()
	t.emitU32(0)

	// false path: CSEL → Rd=Rm, CSINC → Rd=Rm+1, etc.
	op := Op(inst.Op)
	switch op {
	case CSINC:
		// Rd = Rm + 1
		t.sVload(rm)
		t.sPushImm32(1)
		t.emit(vm.OpSAdd)
		t.sVstore(rd)
	case CSINV:
		// Rd = ~Rm
		t.sVload(rm)
		t.emit(vm.OpSNot)
		t.sVstore(rd)
	case CSNEG:
		// Rd = ~Rm + 1 (= -Rm)
		t.sVload(rm)
		t.emit(vm.OpSNot)
		t.sPushImm32(1)
		t.emit(vm.OpSAdd)
		t.sVstore(rd)
	default:
		// CSEL: Rd = Rm
		t.sVload(rm)
		t.sVstore(rd)
	}

	t.emit(vm.OpJmp)
	jmpPos := t.pos()
	t.emitU32(0)

	// true path: Rd = Rn
	truePos := t.pos()
	t.sVload(rn)
	t.sVstore(rd)
	endPos := t.pos()

	binary.LittleEndian.PutUint32(t.code[jccPos:], uint32(truePos))
	binary.LittleEndian.PutUint32(t.code[jmpPos:], uint32(endPos))

	return nil
}

// ---- Stack-mode bit logic translation functions ----

// trStackBitLogicalNot translates BIC/ORN/EON — stack mode
// Rd = Rn OP NOT(shift(Rm))
// vmStackOp: OpSAnd → BIC, OpSOr → ORN, OpSXor → EON
func (t *Translator) trStackBitLogicalNot(inst vm.Instruction, sOp byte, setFlags bool) error {
	rd, rn, rm, err := t.mapReg3(inst)
	if err != nil {
		return err
	}

	// push Rn
	t.pushRegOrZero(inst.Rn, rn)

	// push shift(Rm) then NOT
	t.pushRegOrZero(inst.Rm, rm)
	if inst.Shift != 0 {
		t.emitShiftOnStack(inst.ShiftType, uint32(inst.Shift), inst.SF)
	}
	t.emit(vm.OpSNot) // NOT(shift(Rm))

	// Rd = Rn OP NOT(shift(Rm))
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

// trStackEON translates EON — stack mode
// EON = Rd = Rn XOR NOT(shift(Rm))
func (t *Translator) trStackEON(inst vm.Instruction) error {
	return t.trStackBitLogicalNot(inst, vm.OpSXor, false)
}

// ---- Stack-mode extended register translation functions ----

// trStackAddSubExt translates ADD/SUB (extended register) — stack mode
// Rd = Rn op extend(Rm, shift)
func (t *Translator) trStackAddSubExt(inst vm.Instruction, sOp byte, setFlags bool) error {
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

	// push Rn
	t.sVload(rn)

	// push extend(Rm)
	t.pushRegOrZero(inst.Rm, rm)
	option := inst.ShiftType
	switch option {
	case 0: // UXTB
		t.sPushImm32(0xFF)
		t.emit(vm.OpSAnd)
	case 1: // UXTH
		t.sPushImm32(0xFFFF)
		t.emit(vm.OpSAnd)
	case 2: // UXTW
		t.emit(vm.OpSTrunc32)
	case 3: // UXTX — no-op
	case 4: // SXTB: SHL 56, ASR 56
		t.sPushImm32(56)
		t.emit(vm.OpSShl)
		t.sPushImm32(56)
		t.emit(vm.OpSAsr)
	case 5: // SXTH: SHL 48, ASR 48
		t.sPushImm32(48)
		t.emit(vm.OpSShl)
		t.sPushImm32(48)
		t.emit(vm.OpSAsr)
	case 6: // SXTW: SHL 32, ASR 32
		t.emit(vm.OpSSext32)
	case 7: // SXTX — no-op
	}

	// Extra left shift
	if inst.Shift > 0 {
		t.sPushImm32(uint32(inst.Shift))
		t.emit(vm.OpSShl)
	}

	// Rn op extend(Rm)
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

// ---- Stack-mode atomic operation translation functions ----

// trStackLdadd translates LDADD — atomic add (single-thread simplified) — stack mode
// Semantics: old = Mem[Rn]; Mem[Rn] = old + Rs; Rt = old
func (t *Translator) trStackLdadd(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rt, err := t.mapReg(inst.Rd) // Rt: receives old value
	if err != nil {
		return err
	}
	rs, err := t.mapReg(inst.Rm) // Rs: addend
	if err != nil {
		return err
	}

	var sLdOp, sStOp byte
	if inst.Shift <= 4 {
		sLdOp = vm.OpSLd32
		sStOp = vm.OpSSt32
	} else {
		sLdOp = vm.OpSLd64
		sStOp = vm.OpSSt64
	}

	// SSt pops addr(top), val(second) → Mem[addr] = val
	// SLd pops addr(top) → pushes Mem[addr]

	// 1) load old value
	t.sVload(rn)  // push addr
	t.emit(sLdOp) // pop addr, push old = Mem[addr]
	// stack: [old]

	// 2) store old → Rt
	t.emit(vm.OpSDup) // dup old
	t.sVstore(rt)     // Rt = old
	// stack: [old]

	// 3) compute new = old + Rs
	t.sVload(rs)      // push Rs
	t.emit(vm.OpSAdd) // new = old + Rs
	// stack: [new]

	// 4) store new → Mem[Rn]
	t.sVload(rn)  // push addr
	t.emit(sStOp) // Mem[addr] = new, pops both
	// stack: []

	return nil
}

// trStackCas translates CAS — compare and swap (single-thread simplified) — stack mode
// Semantics: old = Mem[Rn]; if old == Xs then Mem[Rn] = Xt; Xs = old
// Single-threaded: always succeeds, simplified to: old=[Rn]; [Rn]=Xt; Rs=old
func (t *Translator) trStackCas(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rt, err := t.mapReg(inst.Rd) // Rt: new value to store
	if err != nil {
		return err
	}
	rs, err := t.mapReg(inst.Rm) // Rs: compare value, receives old
	if err != nil {
		return err
	}

	var sLdOp, sStOp byte
	if inst.Shift <= 4 {
		sLdOp = vm.OpSLd32
		sStOp = vm.OpSSt32
	} else {
		sLdOp = vm.OpSLd64
		sStOp = vm.OpSSt64
	}

	// Step 1: old = [Rn]
	t.sVload(rn)
	t.emit(sLdOp) // old on stack

	// Step 2: store Rt → [Rn]
	t.sVload(rt)  // push new value
	t.sVload(rn)  // push addr
	t.emit(sStOp) // Mem[addr] = new

	// Step 3: Rs = old (still on stack from step 1)
	t.sVstore(rs)

	return nil
}

// ---- Stack-mode multiply translation functions ----

// trStackSMADDL translates SMADDL/SMSUBL — stack mode
// SMADDL: Xd = Xa + SEXT(Wn) * SEXT(Wm)
// SMSUBL: Xd = Xa - SEXT(Wn) * SEXT(Wm)
func (t *Translator) trStackSMADDL(inst vm.Instruction, isSub bool) error {
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
	raIdx := int((inst.Raw >> 10) & 0x1F)
	if raIdx == 31 {
		raIdx = vm.REG_XZR
	}
	ra, err := t.mapReg(raIdx)
	if err != nil {
		return err
	}

	// Push Ra (or 0 if XZR)
	t.pushRegOrZero(raIdx, ra)

	// SEXT(Wn): SHL 32, ASR 32
	t.sVload(rn)
	t.sPushImm32(32)
	t.emit(vm.OpSShl)
	t.sPushImm32(32)
	t.emit(vm.OpSAsr)

	// SEXT(Wm): SHL 32, ASR 32
	t.sVload(rm)
	t.sPushImm32(32)
	t.emit(vm.OpSShl)
	t.sPushImm32(32)
	t.emit(vm.OpSAsr)

	// multiply
	t.emit(vm.OpSMul)

	// Ra +/- product
	if isSub {
		// stack: [Ra, product] → Ra - product
		// SSub pops b(top), a(second), pushes a-b
		t.emit(vm.OpSSub)
	} else {
		t.emit(vm.OpSAdd)
	}

	t.sVstore(rd)
	return nil
}

// trStackUMADDL translates UMADDL/UMSUBL — stack mode
// UMADDL: Xd = Xa + ZEXT(Wn) * ZEXT(Wm)
// UMSUBL: Xd = Xa - ZEXT(Wn) * ZEXT(Wm)
func (t *Translator) trStackUMADDL(inst vm.Instruction, isSub bool) error {
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
	raIdx := int((inst.Raw >> 10) & 0x1F)
	if raIdx == 31 {
		raIdx = vm.REG_XZR
	}
	ra, err := t.mapReg(raIdx)
	if err != nil {
		return err
	}

	// Push Ra (or 0 if XZR)
	t.pushRegOrZero(raIdx, ra)

	// ZEXT(Wn): trunc32 on stack
	t.sVload(rn)
	t.emit(vm.OpSTrunc32)

	// ZEXT(Wm): trunc32 on stack
	t.sVload(rm)
	t.emit(vm.OpSTrunc32)

	// multiply
	t.emit(vm.OpSMul)

	// Ra +/- product
	if isSub {
		t.emit(vm.OpSSub)
	} else {
		t.emit(vm.OpSAdd)
	}

	t.sVstore(rd)
	return nil
}
