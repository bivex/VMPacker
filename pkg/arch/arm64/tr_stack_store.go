package arm64

import (
	"github.com/vmpacker/pkg/vm"
)

// ---- 栈模式 Store 翻译函数 ----

// trStackStore 翻译 STR (栈模式)
// STR Rt, [Rn, #off] → VLOAD(rn) PUSH(off) S_ADD VLOAD(rt) S_ST{8|16|32|64}
func (t *Translator) trStackStore(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rd, err := t.mapReg(inst.Rd) // Rt = source value
	if err != nil {
		return err
	}

	if inst.Rd >= vm.REG_V_BASE {
		szType := byte(inst.Shift)
		t.sVload(rn)
		if inst.Imm != 0 {
			t.sPushImm(uint64(inst.Imm))
			t.emit(vm.OpSAdd)
		}
		t.emit(vm.OpSVSt, rd, szType)
		return nil
	}

	op := Op(inst.Op)
	var sStOp byte
	switch op {
	case STRB_IMM:
		sStOp = vm.OpSSt8
	case STRH_IMM:
		sStOp = vm.OpSSt16
	case STR_IMM:
		if inst.SF {
			sStOp = vm.OpSSt64
		} else {
			sStOp = vm.OpSSt32
		}
	default:
		sStOp = vm.OpSSt64
	}

	emitWriteback := func() {
		t.sVload(rn)
		wbImm := inst.Imm
		if wbImm >= 0 {
			t.sPushImm(uint64(wbImm))
			t.emit(vm.OpSAdd)
		} else {
			t.sPushImm(uint64(-wbImm))
			t.emit(vm.OpSSub)
		}
		t.sVstore(rn)
	}

	if inst.WB == 3 {
		// pre-index: Rn += imm, then store [Rn]
		emitWriteback()
		// addr
		t.sVload(rn)
		// value
		t.pushRegOrZero(inst.Rd, rd)
		t.emit(sStOp)
	} else if inst.WB == 1 {
		// post-index: store [Rn], then Rn += imm
		t.sVload(rn)
		t.pushRegOrZero(inst.Rd, rd)
		t.emit(sStOp)
		emitWriteback()
	} else {
		// offset mode
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
		t.pushRegOrZero(inst.Rd, rd)
		t.emit(sStOp)
	}

	return nil
}

// trStackStoreReg 翻译 STR (register offset) — 栈模式
func (t *Translator) trStackStoreReg(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rd, err := t.mapReg(inst.Rd) // Rt source
	if err != nil {
		return err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	s := (inst.Raw >> 12) & 1
	size := (inst.Raw >> 30) & 3
	shift := uint32(0)
	if s == 1 {
		shift = size
	}

	// addr = Rn + (Rm << shift)
	t.sVload(rn)
	t.sVload(rm)
	if shift > 0 {
		t.sPushImm32(shift)
		t.emit(vm.OpSShl)
	}
	t.emit(vm.OpSAdd) // addr on stack

	op := Op(inst.Op)
	var sStOp byte
	switch op {
	case STRB_REG:
		sStOp = vm.OpSSt8
	case STRH_REG:
		sStOp = vm.OpSSt16
	default:
		if inst.SF {
			sStOp = vm.OpSSt64
		} else {
			sStOp = vm.OpSSt32
		}
	}

	t.pushRegOrZero(inst.Rd, rd)
	t.emit(sStOp)
	return nil
}

// ---- opcode 映射查找表 ----

// regToStackOp 将 register-based opcode 映射到 stack-based opcode
func regToStackOp(regOp byte) byte {
	switch regOp {
	case vm.OpAdd:
		return vm.OpSAdd
	case vm.OpSub:
		return vm.OpSSub
	case vm.OpMul:
		return vm.OpSMul
	case vm.OpXor:
		return vm.OpSXor
	case vm.OpAnd:
		return vm.OpSAnd
	case vm.OpOr:
		return vm.OpSOr
	case vm.OpShl:
		return vm.OpSShl
	case vm.OpShr:
		return vm.OpSShr
	case vm.OpAsr:
		return vm.OpSAsr
	case vm.OpRor:
		return vm.OpSRor
	case vm.OpUmulh:
		return vm.OpSUmulh
	default:
		return 0
	}
}

// immToStackOp 将 imm-based opcode 映射到 stack-based opcode
func immToStackOp(immOp byte) byte {
	switch immOp {
	case vm.OpAddImm:
		return vm.OpSAdd
	case vm.OpSubImm:
		return vm.OpSSub
	case vm.OpMulImm:
		return vm.OpSMul
	case vm.OpXorImm:
		return vm.OpSXor
	case vm.OpAndImm:
		return vm.OpSAnd
	case vm.OpOrImm:
		return vm.OpSOr
	case vm.OpShlImm:
		return vm.OpSShl
	case vm.OpShrImm:
		return vm.OpSShr
	case vm.OpAsrImm:
		return vm.OpSAsr
	default:
		return 0
	}
}
