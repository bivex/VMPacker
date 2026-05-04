package arm64

import (
	"bytes"
	"testing"

	"github.com/vmpacker/pkg/vm"
)

func TestRegisterShuffling(t *testing.T) {
	vm.GenerateDynamicISA()

	// 1. Create two translators. They should have different regMaps.
	tr1 := NewTranslator(0x1000, 0x100)
	tr2 := NewTranslator(0x1000, 0x100)

	if bytes.Equal(tr1.regMap[:], tr2.regMap[:]) {
		t.Errorf("Translators should have different random regMaps")
	}

	// 2. Verify mapReg uses the map
	reg0 := 0
	mapped1, _ := tr1.mapReg(reg0)
	mapped2, _ := tr2.mapReg(reg0)

	if mapped1 != tr1.regMap[0] {
		t.Errorf("tr1: mapReg(0) should be %d, got %d", tr1.regMap[0], mapped1)
	}
	if mapped2 != tr2.regMap[0] {
		t.Errorf("tr2: mapReg(0) should be %d, got %d", tr2.regMap[0], mapped2)
	}

	if mapped1 == mapped2 {
		t.Logf("Warning: mapReg(0) produced same result in two random maps (statistically possible but unlikely)")
	}
}

func TestShuffledBytecode(t *testing.T) {
	vm.GenerateDynamicISA()

	// Simple MOV X0, X1; RET
	insts := []vm.Instruction{
		{Offset: 0x00, Op: int(ORR_REG), Rd: 0, Rn: vm.REG_XZR, Rm: 1}, // MOV X0, X1
		{Offset: 0x04, Op: int(RET)},
	}

	// Translate twice. The bytecode should be different because registers are mapped differently.
	tr1 := NewTranslator(0x1000, 0x100)
	res1, _ := tr1.Translate(insts)

	tr2 := NewTranslator(0x1000, 0x100)
	res2, _ := tr2.Translate(insts)

	if bytes.Equal(res1.Bytecode, res2.Bytecode) {
		t.Errorf("Bytecode should be different due to register shuffling")
	}
}

func TestTrailerRegMap(t *testing.T) {
	vm.GenerateDynamicISA()

	tr := NewTranslator(0x1000, 0x100)
	res, _ := tr.Translate([]vm.Instruction{{Offset: 0, Op: int(RET)}})

	code := res.Bytecode
	// Trailer starts with regMap (64 bytes)
	// Fixed trailer fields at the end: funcSize(4), funcAddr(8), mapCount(4), ocKey(4), reverse(1) = 21 bytes
	// Before that: mapEntries (mapCount*8)
	// Before that: OpMap (256 bytes)
	// Before that: regMap (64 bytes)

	// In translator.go, trailer starts after CRC section (24 bytes)
	// Bytecode structure: [PureCode][CRCSection(24)][regMap(64)][OpMap(256)]...
	trailerOffset := res.CodeLen + 24
	
	extractedRegMap := code[trailerOffset : trailerOffset+64]
	if !bytes.Equal(extractedRegMap, tr.regMap[:]) {
		t.Errorf("Extracted regMap from trailer does not match translator's regMap")
	}
}
