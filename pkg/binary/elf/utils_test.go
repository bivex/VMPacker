package elf

import (
	"encoding/binary"
	"testing"

	"github.com/vmpacker/pkg/vm"
)

// buildBytecode builds a simple bytecode stream from a list of instructions.
// Each entry is [opcode, ...operandBytes].
func buildBytecode(insts [][]byte) []byte {
	var bc []byte
	for _, inst := range insts {
		bc = append(bc, inst...)
	}
	return bc
}

func TestReverseInstructions_TrailingNOP(t *testing.T) {
	// Ensure the reversed bytecode ends with a trailing NOP so that
	// branch targets to the first instruction don't equal bc_len.
	bc := buildBytecode([][]byte{
		{vm.OpNop},            // offset 0, size 1
		{vm.OpRet, 0},         // offset 1, size 2
	})
	codeLen := len(bc)

	reversed, _, _ := reverseInstructions(bc, codeLen)

	// Last 2 bytes should be OpNop + size marker (1)
	if reversed[len(reversed)-2] != vm.OpNop {
		t.Errorf("expected trailing NOP opcode, got 0x%02X", reversed[len(reversed)-2])
	}
	if reversed[len(reversed)-1] != 1 {
		t.Errorf("expected trailing NOP size marker 1, got %d", reversed[len(reversed)-1])
	}
}

func TestRemapBranchTargets_FirstInstructionTarget(t *testing.T) {
	// Simulate a loop: OpJmp at offset 5 targets offset 0 (first instruction).
	// After reversal + remap, the target must be < len(reversed) (i.e. < bc_len).
	bc := buildBytecode([][]byte{
		{vm.OpNop},                           // offset 0, size 1
		{vm.OpNop},                           // offset 1, size 1
		{vm.OpNop},                           // offset 2, size 1
		{vm.OpNop},                           // offset 3, size 1
		{vm.OpNop},                           // offset 4, size 1
		append([]byte{vm.OpJmp}, u32be(0)...), // offset 5, size 5, target=0
	})
	codeLen := len(bc)

	reversed, offsetMap, _ := reverseInstructions(bc, codeLen)
	newCodeLen := len(reversed)

	// Before remap: verify the target is 0
	// Find the OpJmp in the reversed bytecode
	jmpTargetOff := -1
	for pc := 0; pc < newCodeLen; {
		op := reversed[pc]
		sz := vm.InstructionSize(op)
		if sz == 0 {
			sz = 1
		}
		if op == vm.OpJmp {
			jmpTargetOff = pc + 1
			break
		}
		pc += sz + 1
	}
	if jmpTargetOff < 0 {
		t.Fatal("OpJmp not found in reversed bytecode")
	}

	remapBranchTargets(reversed, newCodeLen, offsetMap, false)

	remapped := binary.LittleEndian.Uint32(reversed[jmpTargetOff:])
	if int(remapped) >= newCodeLen {
		t.Errorf("remapped target 0x%04X >= bc_len %d (would fail BRANCH_TARGET_VALID)", remapped, newCodeLen)
	}
	if int(remapped) <= 0 {
		t.Errorf("remapped target 0x%04X is zero or negative", remapped)
	}

	// Simulate C VM dispatch: pc = remapped, then pc--, sz = bc[pc], pc -= sz
	pc := int(remapped)
	pc--
	if pc < 0 || pc >= newCodeLen {
		t.Fatalf("dispatch: pc-- = %d out of range", pc)
	}
	sz := int(reversed[pc])
	pc -= sz
	if pc < 0 || pc >= newCodeLen {
		t.Fatalf("dispatch: pc after subtract = %d out of range (sz=%d)", pc, sz)
	}
	// Should land on the original first instruction (OpNop)
	if reversed[pc] != vm.OpNop {
		t.Errorf("dispatch landed on opcode 0x%02X, expected OpNop (0x%02X)", reversed[pc], vm.OpNop)
	}
}

func TestRemapBranchTargets_NestedLoopExit(t *testing.T) {
	// Nested loop: inner loop exit targets the outer loop start (offset 0).
	// This is the exact pattern that caused the infinite loop bug.
	bc := buildBytecode([][]byte{
		{vm.OpNop},                            // offset 0: outer loop start (size 1)
		{vm.OpNop},                            // offset 1 (size 1)
		{vm.OpNop},                            // offset 2 (size 1)
		append([]byte{vm.OpJe}, u32be(9)...),  // offset 3: inner exit → outer start (size 5)
		{vm.OpNop},                            // offset 8 (size 1)
		append([]byte{vm.OpJmp}, u32be(3)...), // offset 9: inner loop back (size 5)
		{vm.OpNop},                            // offset 14 (size 1)
		append([]byte{vm.OpJmp}, u32be(1)...), // offset 15: outer loop body entry (size 5)
		{vm.OpRet, 0},                         // offset 20: exit
	})
	codeLen := len(bc)

	reversed, offsetMap, _ := reverseInstructions(bc, codeLen)
	newCodeLen := len(reversed)

	remapBranchTargets(reversed, newCodeLen, offsetMap, false)

	// Verify ALL branch targets are < bc_len
	for pc := 0; pc < newCodeLen; {
		op := reversed[pc]
		sz := vm.InstructionSize(op)
		if sz == 0 {
			sz = 1
		}
		if toff := branchTargetOffset(op); toff > 0 && pc+toff+4 <= newCodeLen {
			target := binary.LittleEndian.Uint32(reversed[pc+toff:])
			if int(target) >= newCodeLen {
				t.Errorf("branch at reversed pc=0x%04X op=0x%02X has target 0x%04X >= bc_len %d",
					pc, op, target, newCodeLen)
			}
			if int(target) <= 0 {
				t.Errorf("branch at reversed pc=0x%04X has target 0x%04X <= 0", pc, target)
			}
		}
		pc += sz + 1
	}
}

func TestRemapBranchTargets_AllValidDispatch(t *testing.T) {
	// For every branch in the reversed bytecode, simulate the C VM dispatch
	// to verify each target lands on a valid instruction boundary.
	bc := buildBytecode([][]byte{
		{vm.OpNop},                            // offset 0
		{vm.OpNop},                            // offset 1
		{vm.OpNop},                            // offset 2
		append([]byte{vm.OpJe}, u32be(0)...),  // offset 3 → offset 0
		{vm.OpNop},                            // offset 8
		append([]byte{vm.OpJmp}, u32be(0)...), // offset 9 → offset 0
		{vm.OpNop},                            // offset 14
		append([]byte{vm.OpJne}, u32be(8)...), // offset 15 → offset 8
		{vm.OpRet, 0},                         // offset 20
	})
	codeLen := len(bc)

	reversed, offsetMap, _ := reverseInstructions(bc, codeLen)
	newCodeLen := len(reversed)

	remapBranchTargets(reversed, newCodeLen, offsetMap, false)

	// Collect instruction boundary positions (start of each instruction in reversed bytecode)
	boundaries := map[int]bool{}
	for pc := 0; pc < newCodeLen; {
		op := reversed[pc]
		sz := vm.InstructionSize(op)
		if sz == 0 {
			sz = 1
		}
		boundaries[pc] = true
		pc += sz + 1
	}

	// For each branch, simulate C VM dispatch and verify landing on a boundary
	for pc := 0; pc < newCodeLen; {
		op := reversed[pc]
		sz := vm.InstructionSize(op)
		if sz == 0 {
			sz = 1
		}
		if toff := branchTargetOffset(op); toff > 0 && pc+toff+4 <= newCodeLen {
			target := binary.LittleEndian.Uint32(reversed[pc+toff:])
			// Simulate: pc = target; pc--; sz = bc[pc]; pc -= sz;
			dispatchPC := int(target)
			dispatchPC--
			if dispatchPC < 0 || dispatchPC >= newCodeLen {
				t.Errorf("branch at 0x%04X: dispatch pc-- = %d out of range", pc, dispatchPC)
				pc += sz + 1
				continue
			}
			dispatchSZ := int(reversed[dispatchPC])
			dispatchPC -= dispatchSZ
			if dispatchPC < 0 || dispatchPC >= newCodeLen {
				t.Errorf("branch at 0x%04X: final pc = %d (sz=%d) out of range", pc, dispatchPC, dispatchSZ)
			} else if !boundaries[dispatchPC] {
				t.Errorf("branch at 0x%04X: dispatch lands at %d, not an instruction boundary", pc, dispatchPC)
			}
		}
		pc += sz + 1
	}
}

// u32be returns 4 bytes of a uint32 in little-endian (for building bytecode)
func u32be(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}
