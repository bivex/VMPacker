package arm32

import (
	"encoding/binary"

	"github.com/vmpacker/pkg/vm"
)

// ARM32 load/store instruction translation.

func ldOpForInst(op Op) byte {
	switch op {
	case LDRB_IMM, LDRB_REG:
		return vm.OpSLd8
	case LDRH_IMM, LDRH_REG:
		return vm.OpSLd16
	case LDRSB_IMM, LDRSB_REG:
		return vm.OpSLd8
	case LDRSH_IMM, LDRSH_REG:
		return vm.OpSLd16
	default:
		return vm.OpSLd32
	}
}

func stOpForInst(op Op) byte {
	switch op {
	case STRB_IMM, STRB_REG:
		return vm.OpSSt8
	case STRH_IMM, STRH_REG:
		return vm.OpSSt16
	default:
		return vm.OpSSt32
	}
}

func needsSignExtend(op Op) int {
	switch op {
	case LDRSB_IMM, LDRSB_REG:
		return 24 // sext from 8 bits: SHL 24, ASR 24
	case LDRSH_IMM, LDRSH_REG:
		return 16 // sext from 16 bits: SHL 16, ASR 16
	default:
		return 0
	}
}

// resolvePCRelativeLoad attempts to resolve a PC-relative LDR at translation time.
// ARM32: PC = instruction address + 8; Thumb: PC = instruction address + 4 (aligned down to 4).
// Returns the resolved value and true if successful, or 0 and false if unresolvable.
func (t *Translator) resolvePCRelativeLoad(inst vm.Instruction) (uint32, bool) {
	if inst.Rn != 15 || t.rawCode == nil {
		return 0, false
	}
	pcVal := inst.Offset + t.pcOffset()
	if !t.thumbMode {
		// ARM mode: PC = inst_addr + 8, target = PC + signed_imm
		targetOff := pcVal + int(inst.Imm)
		if targetOff >= 0 && targetOff+4 <= len(t.rawCode) {
			return binary.LittleEndian.Uint32(t.rawCode[targetOff:]), true
		}
	} else {
		// Thumb mode: PC = (inst_addr + 4) & ~3, target = PC + unsigned_imm
		alignedPC := (pcVal) & ^3
		targetOff := alignedPC + int(inst.Imm)
		if targetOff >= 0 && targetOff+4 <= len(t.rawCode) {
			return binary.LittleEndian.Uint32(t.rawCode[targetOff:]), true
		}
	}
	return 0, false
}

// trCondLoad translates LDR/LDRB/LDRH/LDRSB/LDRSH (immediate offset)
func (t *Translator) trCondLoad(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	// PC-relative LDR: resolve literal pool value at translation time.
	// The original literal pool data is destroyed when the trampoline replaces the function,
	// so we must inline the constant.
	if inst.Rn == 15 && inst.WB == 0 {
		op := Op(inst.Op)
		if val, ok := t.resolvePCRelativeLoad(inst); ok {
			switch op {
			case LDRB_IMM:
				t.sPushImm32(val & 0xFF)
			case LDRH_IMM:
				t.sPushImm32(val & 0xFFFF)
			case LDRSB_IMM:
				v := int32(int8(val & 0xFF))
				t.sPushImm32(uint32(v))
			case LDRSH_IMM:
				v := int32(int16(val & 0xFFFF))
				t.sPushImm32(uint32(v))
			default:
				t.sPushImm32(val)
			}
			t.emitTrunc32()
			t.sVstore(rd)
			if needsFix {
				t.patchCondSkip(skipPos)
			}
			return nil
		}
	}

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	sLdOp := ldOpForInst(Op(inst.Op))

	emitWriteback := func() {
		t.sVload(rn)
		if inst.Imm >= 0 {
			t.sPushImm(uint64(inst.Imm))
			t.emit(vm.OpSAdd)
		} else {
			t.sPushImm(uint64(-inst.Imm))
			t.emit(vm.OpSSub)
		}
		t.sVstore(rn)
	}

	if inst.WB == 3 {
		// Pre-index
		emitWriteback()
		t.sVload(rn)
		t.emit(sLdOp)
	} else if inst.WB == 1 {
		// Post-index
		t.sVload(rn)
		t.emit(sLdOp)
		t.sVstore(rd)
		emitWriteback()
		if inst.Rd == 15 {
			// LDR PC, [Rn], #imm = POP {PC} → function return
			t.emit(vm.OpRet, 0)
		}
		goto signext
	} else {
		// Offset mode
		t.sVload(rn)
		if inst.Imm != 0 {
			if inst.Imm > 0 {
				t.sPushImm(uint64(inst.Imm))
				t.emit(vm.OpSAdd)
			} else {
				t.sPushImm(uint64(-inst.Imm))
				t.emit(vm.OpSSub)
			}
		}
		t.emit(sLdOp)
	}

	// Sign extend if needed
	if sext := needsSignExtend(Op(inst.Op)); sext > 0 {
		t.sPushImm32(uint32(sext))
		t.emit(vm.OpSShl)
		t.sPushImm32(uint32(sext))
		t.emit(vm.OpSAsr)
	}

	t.emitTrunc32()
	t.sVstore(rd)

	if inst.Rd == 15 {
		// LDR PC, [...] = branch to loaded address → function return
		t.emit(vm.OpRet, 0)
	}

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil

signext:
	// Post-index: rd already stored, apply sign extension
	if sext := needsSignExtend(Op(inst.Op)); sext > 0 {
		t.sVload(rd)
		t.sPushImm32(uint32(sext))
		t.emit(vm.OpSShl)
		t.sPushImm32(uint32(sext))
		t.emit(vm.OpSAsr)
		t.emitTrunc32()
		t.sVstore(rd)
	}
	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondStore translates STR/STRB/STRH (immediate offset)
func (t *Translator) trCondStore(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	sStOp := stOpForInst(Op(inst.Op))

	emitWriteback := func() {
		t.sVload(rn)
		if inst.Imm >= 0 {
			t.sPushImm(uint64(inst.Imm))
			t.emit(vm.OpSAdd)
		} else {
			t.sPushImm(uint64(-inst.Imm))
			t.emit(vm.OpSSub)
		}
		t.sVstore(rn)
	}

	if inst.WB == 3 {
		// Pre-index
		emitWriteback()
		t.sVload(rn)
		t.sVload(rd)
		t.emit(sStOp)
	} else if inst.WB == 1 {
		// Post-index
		t.sVload(rn)
		t.sVload(rd)
		t.emit(sStOp)
		emitWriteback()
	} else {
		// Offset mode
		t.sVload(rn)
		if inst.Imm != 0 {
			if inst.Imm > 0 {
				t.sPushImm(uint64(inst.Imm))
				t.emit(vm.OpSAdd)
			} else {
				t.sPushImm(uint64(-inst.Imm))
				t.emit(vm.OpSSub)
			}
		}
		t.sVload(rd)
		t.emit(sStOp)
	}

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondLoadReg translates LDR/LDRB/LDRH/LDRSB/LDRSH (register offset)
func (t *Translator) trCondLoadReg(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

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

	sLdOp := ldOpForInst(Op(inst.Op))

	// addr = Rn +/- (Rm << shift)
	t.sVload(rn)
	t.sVload(rm)
	if inst.Shift != 0 {
		t.sPushImm32(uint32(inst.Shift))
		t.emit(vm.OpSShl)
	}

	if inst.Imm < 0 {
		t.emit(vm.OpSSub)
	} else {
		t.emit(vm.OpSAdd)
	}

	t.emit(sLdOp)

	if sext := needsSignExtend(Op(inst.Op)); sext > 0 {
		t.sPushImm32(uint32(sext))
		t.emit(vm.OpSShl)
		t.sPushImm32(uint32(sext))
		t.emit(vm.OpSAsr)
	}

	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondStoreReg translates STR/STRB/STRH (register offset)
func (t *Translator) trCondStoreReg(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd) // source value
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

	sStOp := stOpForInst(Op(inst.Op))

	// addr = Rn +/- (Rm << shift)
	t.sVload(rn)
	t.sVload(rm)
	if inst.Shift != 0 {
		t.sPushImm32(uint32(inst.Shift))
		t.emit(vm.OpSShl)
	}

	if inst.Imm < 0 {
		t.emit(vm.OpSSub)
	} else {
		t.emit(vm.OpSAdd)
	}

	t.sVload(rd)
	t.emit(sStOp)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondLoadDouble translates LDRD (load double word)
func (t *Translator) trCondLoadDouble(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rd2 := rd + 1 // LDRD loads Rd and Rd+1
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	// [Rn + imm]
	t.sVload(rn)
	if inst.Imm != 0 {
		if inst.Imm > 0 {
			t.sPushImm(uint64(inst.Imm))
			t.emit(vm.OpSAdd)
		} else {
			t.sPushImm(uint64(-inst.Imm))
			t.emit(vm.OpSSub)
		}
	}
	t.sDup() // save base addr for second load
	t.emit(vm.OpSLd32)
	t.sVstore(rd)
	// base + 4
	t.sPushImm32(4)
	t.emit(vm.OpSAdd)
	t.emit(vm.OpSLd32)
	t.sVstore(rd2)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondStoreDouble translates STRD (store double word)
func (t *Translator) trCondStoreDouble(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rd2 := rd + 1
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	// [Rn + imm]
	t.sVload(rn)
	if inst.Imm != 0 {
		if inst.Imm > 0 {
			t.sPushImm(uint64(inst.Imm))
			t.emit(vm.OpSAdd)
		} else {
			t.sPushImm(uint64(-inst.Imm))
			t.emit(vm.OpSSub)
		}
	}
	t.sDup()
	t.sVload(rd)
	t.emit(vm.OpSSt32)
	// base + 4
	t.sPushImm32(4)
	t.emit(vm.OpSAdd)
	t.sVload(rd2)
	t.emit(vm.OpSSt32)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondLDM translates LDM (load multiple)
// Expands reglist into individual loads.
func (t *Translator) trCondLDM(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	reglist := uint16(inst.Imm)
	addrMode := inst.ShiftType // 0=IA, 1=IB, 2=DA, 3=DB
	count := popcount16(reglist)

	// Calculate starting address
	t.sVload(rn)
	switch addrMode {
	case 0: // IA: start at Rn
		// no adjustment
	case 1: // IB: start at Rn+4
		t.sPushImm32(4)
		t.emit(vm.OpSAdd)
	case 2: // DA: start at Rn - 4*(count-1)
		t.sPushImm32(uint32(4 * (count - 1)))
		t.emit(vm.OpSSub)
	case 3: // DB: start at Rn - 4*count
		t.sPushImm32(uint32(4 * count))
		t.emit(vm.OpSSub)
	}

	// Load each register in reglist
	loadedPC := false
	first := true
	for i := 0; i < 16; i++ {
		if reglist&(1<<uint(i)) == 0 {
			continue
		}
		if !first {
			t.sPushImm32(4)
			t.emit(vm.OpSAdd)
		}
		first = false
		t.sDup() // keep addr for next iteration
		t.emit(vm.OpSLd32)

		if i == 15 {
			loadedPC = true
			t.sVstore(14) // store to LR temporarily, handle RET below
		} else {
			t.sVstore(byte(i))
		}
	}
	t.sDrop() // discard remaining address copy

	// Writeback
	if inst.WB != 0 {
		t.sVload(rn)
		switch addrMode {
		case 0, 1: // IA/IB: Rn += 4*count
			t.sPushImm32(uint32(4 * count))
			t.emit(vm.OpSAdd)
		case 2, 3: // DA/DB: Rn -= 4*count
			t.sPushImm32(uint32(4 * count))
			t.emit(vm.OpSSub)
		}
		t.sVstore(rn)
	}

	if loadedPC {
		// POP {PC} = return
		t.emit(vm.OpRet, 0)
	}

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondSTM translates STM (store multiple)
func (t *Translator) trCondSTM(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	reglist := uint16(inst.Imm)
	addrMode := inst.ShiftType
	count := popcount16(reglist)

	// Calculate starting address
	t.sVload(rn)
	switch addrMode {
	case 0: // IA
		// no adjustment
	case 1: // IB
		t.sPushImm32(4)
		t.emit(vm.OpSAdd)
	case 2: // DA
		t.sPushImm32(uint32(4 * (count - 1)))
		t.emit(vm.OpSSub)
	case 3: // DB
		t.sPushImm32(uint32(4 * count))
		t.emit(vm.OpSSub)
	}

	first := true
	for i := 0; i < 16; i++ {
		if reglist&(1<<uint(i)) == 0 {
			continue
		}
		if !first {
			t.sPushImm32(4)
			t.emit(vm.OpSAdd)
		}
		first = false
		t.sDup()
		t.sVload(byte(i))
		t.emit(vm.OpSSt32)
	}
	t.sDrop()

	// Writeback
	if inst.WB != 0 {
		t.sVload(rn)
		switch addrMode {
		case 0, 1:
			t.sPushImm32(uint32(4 * count))
			t.emit(vm.OpSAdd)
		case 2, 3:
			t.sPushImm32(uint32(4 * count))
			t.emit(vm.OpSSub)
		}
		t.sVstore(rn)
	}

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

func popcount16(v uint16) int {
	count := 0
	for v != 0 {
		count += int(v & 1)
		v >>= 1
	}
	return count
}
