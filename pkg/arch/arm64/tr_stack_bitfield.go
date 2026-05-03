package arm64

import (
	"github.com/vmpacker/pkg/vm"
)

// ---- 栈模式 Bitfield 翻译函数 ----

// trStackBFM 翻译 BFM Xd, Xn, #immr, #imms — 位域移动 — 栈模式
// BFI alias:   imms < immr → dst_lsb = regsize-immr, width = imms+1
// BFXIL alias: imms >= immr → dst_lsb = 0, width = imms-immr+1
func (t *Translator) trStackBFM(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	immr := uint32(inst.Imm)
	imms := uint32(inst.Shift)
	regsize := uint32(64)
	if !inst.SF {
		regsize = 32
	}

	var width, srcLSB, dstLSB uint32
	if imms >= immr {
		width = imms - immr + 1
		srcLSB = immr
		dstLSB = 0
	} else {
		width = imms + 1
		srcLSB = 0
		dstLSB = regsize - immr
	}

	mask := uint64((1 << width) - 1)

	// --- 栈操作: extracted = (Rn >> srcLSB) & mask ---
	t.sVload(rn)
	if srcLSB > 0 {
		t.sPushImm32(srcLSB)
		t.emit(vm.OpSShr)
	}
	// & mask
	t.sPushImm(mask)
	t.emit(vm.OpSAnd)

	// << dstLSB
	if dstLSB > 0 {
		t.sPushImm32(dstLSB)
		t.emit(vm.OpSShl)
	}
	// stack: [extracted_shifted]

	// --- Rd = (Rd & ~(mask << dstLSB)) | extracted_shifted ---
	clearMask := ^(mask << dstLSB)
	if !inst.SF {
		clearMask &= 0xFFFFFFFF
	}
	t.sVload(rd)
	t.sPushImm(clearMask)
	t.emit(vm.OpSAnd)

	// OR with extracted
	t.emit(vm.OpSOr)

	if !inst.SF {
		t.emit(vm.OpSTrunc32)
	}
	t.sVstore(rd)
	return nil
}

// trStackEXTR 翻译 EXTR Xd, Xn, Xm, #lsb — 位域提取 — 栈模式
// ROR alias: Rn == Rm → rotate right
// General:   result = (Rm >> lsb) | (Rn << (regSize-lsb))
func (t *Translator) trStackEXTR(inst vm.Instruction) error {
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
	lsb := uint32(inst.Imm)
	regSize := uint32(32)
	if inst.SF {
		regSize = 64
	}

	if inst.Rn == inst.Rm {
		// ROR alias: 栈模式
		t.sVload(rn)
		t.sPushImm32(lsb)
		t.sPushImm32(regSize)
		t.emit(vm.OpSRor)
	} else {
		// General EXTR: (Rm >> lsb) | (Rn << (regSize-lsb))
		// Part 1: Rm >> lsb
		t.sVload(rm)
		t.sPushImm32(lsb)
		t.emit(vm.OpSShr)

		// Part 2: Rn << (regSize-lsb)
		t.sVload(rn)
		t.sPushImm32(regSize - lsb)
		t.emit(vm.OpSShl)

		// OR them
		t.emit(vm.OpSOr)
	}

	if !inst.SF {
		t.emit(vm.OpSTrunc32)
	}
	t.sVstore(rd)
	return nil
}

// trStackUBFM 翻译 UBFM — 栈模式
// 覆盖所有 case: LSR, LSL, UXTB, UXTH, UBFX, UBFIZ
func (t *Translator) trStackUBFM(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	immr := uint32(inst.Imm)
	imms := uint32(inst.Shift)

	regSize := uint32(32)
	if inst.SF {
		regSize = 64
	}

	switch {
	case imms == regSize-1:
		// LSR
		t.sVload(rn)
		t.sPushImm32(immr)
		t.emit(vm.OpSShr)
	case imms+1 == immr:
		// LSL
		t.sVload(rn)
		t.sPushImm32(regSize - immr)
		t.emit(vm.OpSShl)
	case imms == 7 && immr == 0:
		// UXTB
		t.sVload(rn)
		t.sPushImm(0xFF)
		t.emit(vm.OpSAnd)
	case imms == 15 && immr == 0:
		// UXTH
		t.sVload(rn)
		t.sPushImm(0xFFFF)
		t.emit(vm.OpSAnd)
	default:
		if imms >= immr {
			// UBFX: (Rn >> immr) & mask
			width := imms - immr + 1
			t.sVload(rn)
			t.sPushImm32(immr)
			t.emit(vm.OpSShr)
			mask := uint64((1 << width) - 1)
			t.sPushImm(mask)
			t.emit(vm.OpSAnd)
		} else {
			// UBFIZ: (Rn & mask) << shift
			width := imms + 1
			shift := regSize - immr
			mask := uint64((1 << width) - 1)
			t.sVload(rn)
			t.sPushImm(mask)
			t.emit(vm.OpSAnd)
			t.sPushImm32(shift)
			t.emit(vm.OpSShl)
		}
	}

	if !inst.SF {
		t.emit(vm.OpSTrunc32)
	}
	t.sVstore(rd)
	return nil
}
