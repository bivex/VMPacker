package arm32

import (
	"github.com/vmpacker/pkg/vm"
)

// ARM32 special instruction translation: SVC, MRS, ADR

// trCondSVC translates SVC (system call)
// ARM32 syscall ABI: R7 = syscall number, R0-R5 = args
func (t *Translator) trCondSVC(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	t.emit(vm.OpSvc, byte(inst.Imm&0xFF), byte((inst.Imm>>8)&0xFF))

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondMRS translates MRS Rd, CPSR/SPSR
func (t *Translator) trCondMRS(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	sysreg := uint16(inst.Imm & 0x1F)
	t.emit(vm.OpMrs, rd, byte(sysreg&0xFF), byte(sysreg>>8))

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}

// trCondADR translates ADR Rd, label (PC-relative address)
func (t *Translator) trCondADR(inst vm.Instruction) error {
	skipPos, needsFix := t.emitCondCheck(inst.Cond)

	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return err
	}

	// ARM32: PC = current instruction + 8; Thumb: +4
	absAddr := t.funcAddr + uint64(inst.Offset) + uint64(t.pcOffset()) + uint64(inst.Imm)
	t.sPushImm(absAddr)
	t.emitTrunc32()
	t.sVstore(rd)

	if needsFix {
		t.patchCondSkip(skipPos)
	}
	return nil
}
