package x86_64

import (
	"testing"
)

func TestDecode_Basic(t *testing.T) {
	d := NewDecoder()

	// push rbp (55)
	code := []byte{0x55}
	insts, err := d.Decode(code, 0x1000)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(insts) != 1 {
		t.Fatalf("Expected 1 instruction, got %d", len(insts))
	}
	if insts[0].Size != 1 {
		t.Errorf("Expected size 1, got %d", insts[0].Size)
	}

	// mov rax, 0x12345678 (48 B8 78 56 34 12 00 00 00 00)
	code = []byte{0x48, 0xB8, 0x78, 0x56, 0x34, 0x12, 0x00, 0x00, 0x00, 0x00}
	insts, err = d.Decode(code, 0x2000)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(insts) != 1 {
		t.Fatalf("Expected 1 instruction, got %d", len(insts))
	}
	if insts[0].Size != 10 {
		t.Errorf("Expected size 10, got %d", insts[0].Size)
	}
}

func TestDecode_Sequence(t *testing.T) {
	d := NewDecoder()

	// push rbp; mov rbp, rsp; sub rsp, 16; ret
	code := []byte{
		0x55,             // push rbp
		0x48, 0x89, 0xE5, // mov rbp, rsp
		0x48, 0x83, 0xEC, 0x10, // sub rsp, 16
		0xC3,             // ret
	}
	insts, err := d.Decode(code, 0x3000)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(insts) != 4 {
		t.Fatalf("Expected 4 instructions, got %d", len(insts))
	}
	offsets := []int{0, 1, 4, 8}
	sizes := []uint32{1, 3, 4, 1}
	for i, inst := range insts {
		if inst.Offset != offsets[i] {
			t.Errorf("Inst %d: expected offset %d, got %d", i, offsets[i], inst.Offset)
		}
		if inst.Size != sizes[i] {
			t.Errorf("Inst %d: expected size %d, got %d", i, sizes[i], inst.Size)
		}
	}
}
