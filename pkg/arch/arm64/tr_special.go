package arm64

import (
	

	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// Special instruction translation — ADRP / ADR
// ============================================================

// emitAddrWithSlide emits bytecode that computes (link_time_addr + slide)
// and stores it to register rd. S_LOAD_SLIDE pushes the runtime slide value,
// so the immediate must be the raw link-time VA (NOT patched by RTLR).
func (t *Translator) emitAddrWithSlide(rd byte, addr uint64) {
	t.sPushImm64(addr)
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
			
			// Check if this points to an encrypted string
			if sref, ok := t.stringRefs[finalAddr]; ok {
				// Push address (with slide)
				t.emit(vm.OpSLoadSlide)
				t.sPushImm(sref.Addr)
				t.emit(vm.OpSAdd)

				t.sPushImm32(sref.Len)
				t.sPushImm32(sref.Key)
				t.emit(vm.OpSDecryptStr)
				t.sVstore(rd)
				return 1, nil
			}

			t.emitAddrWithSlide(rd, finalAddr)
			return 1, nil
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

// trSVC translates SVC #imm16
// Bytecode: [OpSvc][imm16_lo][imm16_hi] = 3B
// Handler uses inline asm to execute svc #0, passing syscall args via VM registers
func (t *Translator) trSVC(inst vm.Instruction) error {
	imm16 := uint16(inst.Imm)
	t.emit(vm.OpSvc, byte(imm16), byte(imm16>>8))
	return nil
}
