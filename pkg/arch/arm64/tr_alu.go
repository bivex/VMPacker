package arm64

import (
	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// ALU — only keeps special-format insns that cannot use stack mode
// ============================================================

// trCCMP translates CCMP/CCMN (reg/imm)
// Bytecode: [op][cond][nzcv][rn][rm_or_imm5][sf] = 6B
// inst.Cond = condition, inst.WB = nzcv (default flags)
// isNeg: true=CCMN, false=CCMP
// isImm: true=imm5 variant (inst.Rm reused as imm5), false=reg variant
func (t *Translator) trCCMP(inst vm.Instruction, isNeg bool, isImm bool) error {
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return err
	}

	var vmOp byte
	if isNeg {
		if isImm {
			vmOp = vm.OpCcmnImm
		} else {
			vmOp = vm.OpCcmnReg
		}
	} else {
		if isImm {
			vmOp = vm.OpCcmpImm
		} else {
			vmOp = vm.OpCcmpReg
		}
	}

	var rmOrImm byte
	if isImm {
		rmOrImm = byte(inst.Rm) // Rm field reused as imm5
	} else {
		rm, err := t.mapReg(inst.Rm)
		if err != nil {
			return err
		}
		rmOrImm = rm
	}

	var sf byte
	if inst.SF {
		sf = 1
	}

	t.emit(vmOp, byte(inst.Cond), byte(inst.WB), rn, rmOrImm, sf)
	return nil
}

// trMRS translates MRS Xd, <sysreg> — read system register
// Format: [OpMrs][d][sysreg_lo][sysreg_hi] = 4B
// sysreg is 15-bit encoded, stored as uint16 LE
func (t *Translator) trMRS(inst vm.Instruction) error {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}
	sysreg := uint16(inst.Imm & 0x7FFF)
	t.emit(vm.OpMrs, rd, byte(sysreg&0xFF), byte(sysreg>>8))
	return nil
}
