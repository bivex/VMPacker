package arm64

import (
	

	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// 特殊指令翻译 — ADRP / ADR
// ============================================================

// emitAddrWithSlide emits bytecode that computes (link_time_addr + slide)
// and stores it to register rd. For ET_EXEC slide=0 so this is a no-op addition.
func (t *Translator) emitAddrWithSlide(rd byte, addr uint64) {
	bcOff := t.pos() + 2 // Offset of the imm64 in OpSPushImm64
	t.sPushImm64(addr)
	t.addReloc(bcOff, addr, false)
	t.emit(vm.OpSLoadSlide)
	t.emit(vm.OpSAdd)
	t.emit(vm.OpSVstore, rd)
}

func (t *Translator) trADRP(instructions []vm.Instruction, idx int) (int, error) {
	inst := instructions[idx]
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return 0, err
	}

	pc := t.funcAddr + uint64(inst.Offset)
	pageBase := pc &^ 0xFFF
	adrpResult := pageBase + uint64(inst.Imm)

	if idx+1 < len(instructions) {
		next := instructions[idx+1]
		if Op(next.Op) == ADD_IMM && next.Rd == inst.Rd && next.Rn == inst.Rd {
			finalAddr := adrpResult + uint64(next.Imm)
			t.emitAddrWithSlide(rd, finalAddr)
			return 1, nil
		}
		if (Op(next.Op) == LDR_IMM || Op(next.Op) == LDR_REG) && next.Rn == inst.Rd {
			// ADRP + LDR: often used for loading constants
			// We can't easily merge the LDR into a single VM op yet, 
			// but we can ensure ADRP result is correct.
		}
	}

	t.emitAddrWithSlide(rd, adrpResult)
	return 0, nil
}

func (t *Translator) trADR(inst vm.Instruction) (int, error) {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return 0, err
	}
	pc := t.funcAddr + uint64(inst.Offset)
	addr := pc + uint64(inst.Imm)
	t.emitAddrWithSlide(rd, addr)
	return 0, nil
}

// trSVC 翻译 SVC #imm16
// 字节码: [OpSvc][imm16_lo][imm16_hi] = 3B
// handler 使用 inline asm 执行 svc #0，从 VM 寄存器传递 syscall 参数
func (t *Translator) trSVC(inst vm.Instruction) error {
	imm16 := uint16(inst.Imm)
	t.emit(vm.OpSvc, byte(imm16), byte(imm16>>8))
	return nil
}
