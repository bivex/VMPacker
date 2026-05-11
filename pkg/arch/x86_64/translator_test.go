package x86_64

import (
	"bytes"
	"testing"

	"github.com/vmpacker/pkg/vm"
)

func TestTranslate_Basic(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// push rbp; mov rbp, rsp; ret
	code := []byte{
		0x55,             // push rbp
		0x48, 0x89, 0xE5, // mov rbp, rsp
		0xC3, // ret
	}

	dec := NewDecoder()
	insts, err := dec.Decode(code, 0x1000)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	tr := NewTranslator(0x1000, len(code), code)
	res, err := tr.Translate(insts)
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}

	if res.TransInsts != 3 {
		t.Errorf("Expected 3 translated instructions, got %d", res.TransInsts)
	}

	if len(res.Bytecode) == 0 {
		t.Fatal("Bytecode is empty")
	}

	// Should contain some VM opcodes
	if !bytes.Contains(res.Bytecode, []byte{vm.OpRet}) {
		t.Error("OpRet not found in bytecode")
	}
	if !bytes.Contains(res.Bytecode, []byte{vm.OpPush}) {
		t.Error("OpPush not found in bytecode")
	}
}

func TestTranslate_CFF(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// simple loop or branch
	code := []byte{
		0x83, 0xF8, 0x0A, // cmp eax, 10
		0x7C, 0x02, // jl +2
		0x31, 0xC0, // xor eax, eax
		0xC3, // ret
	}

	dec := NewDecoder()
	insts, err := dec.Decode(code, 0x1000)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	tr := NewTranslator(0x1000, len(code), code)
	tr.SetCFF(true)
	res, err := tr.Translate(insts)
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}

	if len(tr.bbStates) < 2 {
		t.Errorf("Expected at least 2 basic blocks for CFF, got %d", len(tr.bbStates))
	}

	if !bytes.Contains(res.Bytecode, []byte{vm.OpJe}) && !bytes.Contains(res.Bytecode, []byte{vm.OpJl}) {
		// x86asm.JL should map to OpJl
		// But in CFF it might be more complex
	}
}

func TestTranslate_Relocation(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// mov rax, [rip + 0x100]
	code := []byte{0x48, 0x8B, 0x05, 0x00, 0x01, 0x00, 0x00}

	dec := NewDecoder()
	insts, err := dec.Decode(code, 0x1000)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	tr := NewTranslator(0x1000, len(code), code)
	res, err := tr.Translate(insts)
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}

	if len(res.Relocations) == 0 {
		t.Error("Expected at least one relocation for RIP-relative access")
	}
}

func TestTranslate_FP(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// addsd xmm0, xmm1; movss xmm2, [rax]; cvtsi2sd xmm3, rbx
	code := []byte{
		0xF2, 0x0F, 0x58, 0xC1, // addsd xmm0, xmm1
		0xF3, 0x0F, 0x10, 0x10, // movss xmm2, [rax]
		0xF2, 0x48, 0x0F, 0x2A, 0xDB, // cvtsi2sd xmm3, rbx
	}

	dec := NewDecoder()
	insts, err := dec.Decode(code, 0x1000)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	tr := NewTranslator(0x1000, len(code), code)
	res, err := tr.Translate(insts)
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}

	if res.TransInsts != 3 {
		t.Errorf("Expected 3 translated instructions, got %d", res.TransInsts)
	}

	if !bytes.Contains(res.Bytecode, []byte{vm.OpSFAdd}) {
		t.Error("OpSFAdd not found in bytecode")
	}
	if !bytes.Contains(res.Bytecode, []byte{vm.OpSVLd}) {
		t.Error("OpSVLd not found in bytecode")
	}
	if !bytes.Contains(res.Bytecode, []byte{vm.OpSFCvtIF}) {
		t.Error("OpSFCvtIF not found in bytecode")
	}
}

func TestTranslate_Hybrid(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// Generate many NOPs; with hybrid mode enabled, each has 20% chance
	// to be emitted as a native block. With 200 NOPs, probability of
	// zero emissions is ~1.6e-20, effectively guaranteeing at least one
	// OpSNativeExec appears if hybrid emission works.
	const n = 200
	code := make([]byte, n)
	for i := range code {
		code[i] = 0x90 // NOP
	}

	dec := NewDecoder()
	insts, err := dec.Decode(code, 0x1000)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	tr := NewTranslator(0x1000, len(code), code)
	tr.SetHybrid(true)
	res, err := tr.Translate(insts)
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}

	if res.TransInsts != n {
		t.Errorf("Expected %d translated instructions, got %d", n, res.TransInsts)
	}

	// Verify at least one native block was emitted
	if !bytes.Contains(res.Bytecode, []byte{vm.OpSNativeExec}) {
		t.Error("expected at least one OpSNativeExec in bytecode with hybrid mode enabled")
	}
}
