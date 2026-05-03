package arm64

import (
	"fmt"

	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// Bitfield translation - SBFM (safe, no temp register conflicts)
// UBFM migrated to tr_stack.go (trStackUBFM)
// ============================================================

func (t *Translator) trSBFM(inst vm.Instruction) error {
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

	if imms == regSize-1 {
		// ASR: for 32-bit, trunc32 first to ensure high 32 bits are 0, then use 64-bit ASR
		if !inst.SF {
			// First sign-extend source to 64-bit: SHL 32, ASR 32 to extend bit31 to bit63
			t.emit(vm.OpShlImm, rd, rn)
			t.emitU32(32)
			t.emit(vm.OpAsrImm, rd, rd)
			t.emitU32(32 + immr)
			t.trunc32(rd)
		} else {
			t.emit(vm.OpAsrImm, rd, rn)
			t.emitU32(immr)
		}
		return nil
	}
	if immr == 0 {
		// SXTB/SXTH/SXTW: sign extension
		// VM registers are 64-bit, so use 64-bit shift width for sign extension
		var shiftAmt uint32
		if inst.SF {
			shiftAmt = 64 - (imms + 1)
		} else {
			// 32-bit: shift left to bit63, then ASR back, finally trunc32
			shiftAmt = 64 - (imms + 1)
		}
		t.emit(vm.OpShlImm, rd, rn)
		t.emitU32(shiftAmt)
		t.emit(vm.OpAsrImm, rd, rd)
		t.emitU32(shiftAmt)
		if !inst.SF {
			t.trunc32(rd)
		}
		return nil
	}
	return fmt.Errorf("complex SBFM (immr=%d, imms=%d) not yet supported", immr, imms)
}
