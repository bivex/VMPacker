package arm64

import (
	"encoding/binary"
	"testing"

	"github.com/vmpacker/pkg/vm"
)

func TestIdentifyBasicBlocks(t *testing.T) {
	// Mock instructions for a simple loop:
	// 0x00: MOV X0, #10
	// 0x04: SUBS X0, X0, #1 (Loop start target)
	// 0x08: B.NE 0x04       (Loop back)
	// 0x0C: RET             (End)
	
	insts := []vm.Instruction{
		{Offset: 0x00, Op: int(MOVZ), Rd: 0, Imm: 10},
		{Offset: 0x04, Op: int(SUBS_IMM), Rd: 0, Rn: 0, Imm: 1},
		{Offset: 0x08, Op: int(B_COND), Cond: COND_NE, Imm: -4}, // Target 0x04
		{Offset: 0x0C, Op: int(RET)},
	}

	tr := NewTranslator(0x1000, 0x10)
	starts := tr.identifyBasicBlocks(insts)

	// Expected BB starts:
	// 0x00: Function start
	// 0x04: Target of B.NE
	// 0x0C: Instruction following branch B.NE
	expected := map[int]bool{
		0x00: true,
		0x04: true,
		0x0C: true,
	}

	if len(starts) != len(expected) {
		t.Errorf("Expected %d blocks, got %d", len(expected), len(starts))
	}

	for addr := range expected {
		if !starts[addr] {
			t.Errorf("Address 0x%X expected as BB start, but not found", addr)
		}
	}
}

func TestTranslateCFF(t *testing.T) {
	vm.GenerateDynamicISA()
	
	// Simple conditional:
	// 0x00: CMP X0, #0
	// 0x04: B.EQ 0x0C
	// 0x08: MOV X1, #1
	// 0x0C: RET
	insts := []vm.Instruction{
		{Offset: 0x00, Op: int(SUBS_IMM), Rd: vm.REG_XZR, Rn: 0, Imm: 0},
		{Offset: 0x04, Op: int(B_COND), Cond: COND_EQ, Imm: 8}, // Target 0x0C
		{Offset: 0x08, Op: int(MOVZ), Rd: 1, Imm: 1},
		{Offset: 0x0C, Op: int(RET)},
	}

	tr := NewTranslator(0x1000, 0x10)
	tr.SetCFF(true)
	
	result, err := tr.Translate(insts)
	if err != nil {
		t.Fatalf("Translation failed: %v", err)
	}

	code := result.Bytecode

	// Verify Dispatcher exists
	// Dispatcher should have multiple OpCmpImm (R62, ID) and OpJe patterns
	foundDispatcher := false
	for i := 0; i < len(code)-10; i++ {
		if code[i] == vm.OpCmpImm && code[i+1] == 62 {
			foundDispatcher = true
			break
		}
	}
	if !foundDispatcher {
		t.Error("Dispatcher (OpCmpImm R62, ...) not found in CFF bytecode")
	}

	// Verify State Update exists (OpMovImm32 R62, ID)
	foundStateUpdate := false
	for i := 0; i < len(code)-6; i++ {
		if code[i] == vm.OpMovImm32 && code[i+1] == 62 {
			foundStateUpdate = true
			break
		}
	}
	if !foundStateUpdate {
		t.Error("State update (OpMovImm32 R62, ...) not found in CFF bytecode")
	}

	// Verify final Jmp to dispatcher exists
	foundJmpDisp := false
	for i := 0; i < len(code)-5; i++ {
		if code[i] == vm.OpJmp {
			target := binary.LittleEndian.Uint32(code[i+1 : i+5])
			if int(target) == tr.dispPos {
				foundJmpDisp = true
				break
			}
		}
	}
	if !foundJmpDisp {
		t.Error("Jump to dispatcher not found in CFF bytecode")
	}
}
