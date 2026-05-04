package arm64

import (
	"testing"

	"github.com/vmpacker/pkg/vm"
)

func TestTranslateMBA(t *testing.T) {
	vm.GenerateDynamicISA()
	
	// Simple ADD: MOV X0, #10; ADD X0, X0, #5; RET
	insts := []vm.Instruction{
		{Offset: 0x00, Op: int(MOVZ), Rd: 0, Imm: 10},
		{Offset: 0x04, Op: int(ADD_IMM), Rd: 0, Rn: 0, Imm: 5},
		{Offset: 0x08, Op: int(SUBS_IMM), Rd: 0, Rn: 0, Imm: 1},
		{Offset: 0x0C, Op: int(ORR_IMM), Rd: 0, Rn: 0, Imm: 0xFF},
		{Offset: 0x10, Op: int(EOR_REG), Rd: 0, Rn: 0, Rm: 1},
		{Offset: 0x14, Op: int(RET)},
	}

	// 1. Translate without MBA
	trNormal := NewTranslator(0x1000, 0x10)
	trNormal.SetMBA(false)
	resNormal, _ := trNormal.Translate(insts)
	lenNormal := len(resNormal.Bytecode)

	// 2. Translate with MBA
	trMBA := NewTranslator(0x1000, 0x10)
	trMBA.SetMBA(true)
	resMBA, _ := trMBA.Translate(insts)
	lenMBA := len(resMBA.Bytecode)

	t.Logf("Normal len: %d, MBA len: %d", lenNormal, lenMBA)

	if lenMBA <= lenNormal {
		t.Errorf("MBA should produce more bytecode. Normal: %d, MBA: %d", lenNormal, lenMBA)
	}
}
