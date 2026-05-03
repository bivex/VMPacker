package arm64

import (
	"github.com/vmpacker/pkg/vm"
)

// ---- STP/LDP 栈模式翻译 ----

// abs64 返回 int64 绝对值
func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// imm7 extracts the signed 7-bit immediate from raw instruction bits[21:15]
func (t *Translator) imm7(raw uint32) int {
	imm7 := (raw >> 15) & 0x7F
	if imm7&0x40 != 0 {
		return int(int8(imm7 | 0x80))
	}
	return int(imm7)
}

// trStackSTP 翻译 STP (Store Pair) — 栈模式
func (t *Translator) trStackSTP(inst vm.Instruction) error {
	isSIMD := inst.Rd >= vm.REG_V_BASE
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rt1, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rt2, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	if isSIMD {
		return t.trStackSTPSIMD(inst, rt1, rt2, rn)
	}

	var sStOp byte
	stride := int64(8)
	if !inst.SF {
		sStOp = vm.OpSSt32
		stride = 4
	} else {
		sStOp = vm.OpSSt64
	}

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
		// pre-index: Rn += imm, then store [Rn] and [Rn+stride]
		emitWriteback()
		// store Rt1 at [Rn]
		t.sVload(rn)
		t.pushRegOrZero(inst.Rd, rt1)
		t.emit(sStOp)
		// store Rt2 at [Rn+stride]
		t.sVload(rn)
		t.sPushImm(uint64(stride))
		t.emit(vm.OpSAdd)
		t.pushRegOrZero(inst.Rm, rt2)
		t.emit(sStOp)
	} else {
		storeImm := inst.Imm
		if inst.WB == 1 {
			storeImm = 0 // post-index
		}
		// store Rt1 at [Rn+storeImm]
		t.sVload(rn)
		if storeImm != 0 {
			t.sPushImm(uint64(abs64(storeImm)))
			if storeImm > 0 {
				t.emit(vm.OpSAdd)
			} else {
				t.emit(vm.OpSSub)
			}
		}
		t.pushRegOrZero(inst.Rd, rt1)
		t.emit(sStOp)
		// store Rt2 at [Rn+storeImm+stride]
		t.sVload(rn)
		off2 := storeImm + stride
		if off2 != 0 {
			t.sPushImm(uint64(abs64(off2)))
			if off2 > 0 {
				t.emit(vm.OpSAdd)
			} else {
				t.emit(vm.OpSSub)
			}
		}
		t.pushRegOrZero(inst.Rm, rt2)
		t.emit(sStOp)
		// post-index writeback
		if inst.WB == 1 {
			emitWriteback()
		}
	}
	return nil
}

func (t *Translator) trStackSTPSIMD(inst vm.Instruction, rt1, rt2, rn byte) error {
	szType := byte(inst.Shift)
	stride := inst.Imm / int64(t.imm7(inst.Raw)) // scale derived from Imm/imm7
	if t.imm7(inst.Raw) == 0 {
		stride = int64(1 << szType)
	}

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
		emitWriteback()
		t.sVload(rn)
		t.emit(vm.OpSVSt, rt1, szType)
		t.sVload(rn)
		t.sPushImm(uint64(stride))
		t.emit(vm.OpSAdd)
		t.emit(vm.OpSVSt, rt2, szType)
	} else {
		storeImm := inst.Imm
		if inst.WB == 1 {
			storeImm = 0
		}
		t.sVload(rn)
		if storeImm != 0 {
			t.sPushImm(uint64(storeImm))
			t.emit(vm.OpSAdd)
		}
		t.emit(vm.OpSVSt, rt1, szType)

		t.sVload(rn)
		if storeImm+stride != 0 {
			t.sPushImm(uint64(storeImm + stride))
			t.emit(vm.OpSAdd)
		}
		t.emit(vm.OpSVSt, rt2, szType)

		if inst.WB == 1 {
			emitWriteback()
		}
	}
	return nil
}

func (t *Translator) trStackLDPSIMD(inst vm.Instruction, rt1, rt2, rn byte) error {
	szType := byte(inst.Shift)
	stride := inst.Imm / int64(t.imm7(inst.Raw))
	if t.imm7(inst.Raw) == 0 {
		stride = int64(1 << szType)
	}

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
		emitWriteback()
		t.sVload(rn)
		t.emit(vm.OpSVLd, rt1, szType)
		t.sVload(rn)
		t.sPushImm(uint64(stride))
		t.emit(vm.OpSAdd)
		t.emit(vm.OpSVLd, rt2, szType)
	} else {
		loadImm := inst.Imm
		if inst.WB == 1 {
			loadImm = 0
		}
		t.sVload(rn)
		if loadImm != 0 {
			t.sPushImm(uint64(loadImm))
			t.emit(vm.OpSAdd)
		}
		t.emit(vm.OpSVLd, rt1, szType)

		t.sVload(rn)
		if loadImm+stride != 0 {
			t.sPushImm(uint64(loadImm + stride))
			t.emit(vm.OpSAdd)
		}
		t.emit(vm.OpSVLd, rt2, szType)

		if inst.WB == 1 {
			emitWriteback()
		}
	}
	return nil
}

// trStackLDP 翻译 LDP (Load Pair) — 栈模式
func (t *Translator) trStackLDP(inst vm.Instruction) error {
	isSIMD := inst.Rd >= vm.REG_V_BASE
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rt1, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rt2, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}

	if isSIMD {
		return t.trStackLDPSIMD(inst, rt1, rt2, rn)
	}

	var sLdOp byte
	stride := int64(8)
	if !inst.SF {
		sLdOp = vm.OpSLd32
		stride = 4
	} else {
		sLdOp = vm.OpSLd64
	}

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
		// pre-index
		emitWriteback()
		// Rt1 = [Rn]
		t.sVload(rn)
		t.emit(sLdOp)
		t.sVstore(rt1)
		// Rt2 = [Rn+stride]
		t.sVload(rn)
		t.sPushImm(uint64(stride))
		t.emit(vm.OpSAdd)
		t.emit(sLdOp)
		t.sVstore(rt2)
	} else {
		loadImm := inst.Imm
		if inst.WB == 1 {
			loadImm = 0
		}
		// 栈模式不需要 pickTemp! 当 rt1==rn 时:
		// 先计算 addr2 并保存到栈, 再 load
		// 但更简单的方式: 先计算 base+offset, load rt1, 再计算 base+offset+stride, load rt2

		// 先保存 base 地址到栈: addr_base = Rn + loadImm
		t.sVload(rn)
		if loadImm != 0 {
			t.sPushImm(uint64(abs64(loadImm)))
			if loadImm > 0 {
				t.emit(vm.OpSAdd)
			} else {
				t.emit(vm.OpSSub)
			}
		}
		t.sDup() // duplicate base addr for second load

		// load Rt1 from addr_base
		t.emit(sLdOp)
		t.sVstore(rt1)

		// stack now has: [addr_base]
		// load Rt2 from addr_base + stride
		t.sPushImm(uint64(stride))
		t.emit(vm.OpSAdd)
		t.emit(sLdOp)
		t.sVstore(rt2)

		if inst.WB == 1 {
			emitWriteback()
		}
	}
	return nil
}

// trStackLdpsw 翻译 LDPSW — Load pair of signed words — 栈模式
// 加载两个 32-bit 值并 sign-extend 到 64-bit
func (t *Translator) trStackLdpsw(inst vm.Instruction) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}
	rt1, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	rt2, err := t.mapReg(inst.Rm)
	if err != nil {
		return err
	}
	const stride = int64(4)

	emitWriteback := func(imm int64) {
		if imm >= 0 {
			t.sVload(rn)
			t.sPushImm(uint64(imm))
			t.emit(vm.OpSAdd)
			t.sVstore(rn)
		} else {
			t.sVload(rn)
			t.sPushImm(uint64(-imm))
			t.emit(vm.OpSSub)
			t.sVstore(rn)
		}
	}

	sextW := func(reg byte) {
		// sign-extend 32→64: SHL 32, ASR 32
		t.sVload(reg)
		t.sPushImm32(32)
		t.emit(vm.OpSShl)
		t.sPushImm32(32)
		t.emit(vm.OpSAsr)
		t.sVstore(reg)
	}

	if inst.WB == 3 { // pre-index
		emitWriteback(inst.Imm)
		// load [Rn+0]
		t.sVload(rn)
		t.emit(vm.OpSLd32)
		t.sVstore(rt1)
		// load [Rn+4]
		t.sVload(rn)
		t.sPushImm(uint64(stride))
		t.emit(vm.OpSAdd)
		t.emit(vm.OpSLd32)
		t.sVstore(rt2)
	} else {
		loadImm := inst.Imm
		if inst.WB == 1 {
			loadImm = 0
		}
		// load [Rn+loadImm] — 栈模式不需要 pickTemp, 即使 rt1==rn 也安全
		// 因为 VLOAD(rn) 在栈上复制了值，后续 VSTORE(rt1) 不影响栈上的地址
		t.sVload(rn)
		if loadImm != 0 {
			t.sPushImm(uint64(loadImm))
			t.emit(vm.OpSAdd)
		}
		t.emit(vm.OpSDup) // dup addr for second load
		t.emit(vm.OpSLd32)
		t.sVstore(rt1)
		// [addr still on stack] + stride
		t.sPushImm(uint64(stride))
		t.emit(vm.OpSAdd)
		t.emit(vm.OpSLd32)
		t.sVstore(rt2)

		if inst.WB == 1 {
			emitWriteback(inst.Imm)
		}
	}

	// Sign-extend both 32→64
	sextW(rt1)
	sextW(rt2)
	return nil
}
