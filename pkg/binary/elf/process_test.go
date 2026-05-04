package elf

import (
	"bytes"
	"debug/elf"
	"os"
	"path/filepath"
	"testing"

	"github.com/vmpacker/pkg/arch/arm64"
	"github.com/vmpacker/pkg/vm"
)

func TestMakeSegmentsWritable(t *testing.T) {
	// Let's use demo_simple for this test
	projectRoot := filepath.Join("..", "..", "..")
	inputPath := filepath.Join(projectRoot, "demo", "demo_simple")
	
	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Skipf("Skipping test because demo binary not found at %s", inputPath)
	}

	// Work on a copy of the data
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	f, err := elf.NewFile(bytes.NewReader(dataCopy))
	if err != nil {
		t.Fatalf("Failed to parse demo binary: %v", err)
	}
	defer f.Close()

	packer := NewPacker(inputPath, "", nil, nil, false, false, false, false, []byte{})
	packer.data = dataCopy

	// Ensure there is at least one Read-Only PT_LOAD segment before we run
	foundRO := false
	for _, prog := range f.Progs {
		if prog.Type == elf.PT_LOAD && (prog.Flags&elf.PF_R != 0) && (prog.Flags&elf.PF_W == 0) {
			foundRO = true
			break
		}
	}
	
	if !foundRO {
		t.Logf("No purely Read-Only PT_LOAD segment found initially.")
	}

	// Apply function
	packer.makeSegmentsWritable(f)

	// Now parse it again to check if it worked
	f2, err := elf.NewFile(bytes.NewReader(packer.data))
	if err != nil {
		t.Fatalf("Failed to re-parse ELF after modification: %v", err)
	}
	defer f2.Close()

	for _, prog := range f2.Progs {
		if prog.Type == elf.PT_LOAD {
			if prog.Flags&elf.PF_W == 0 && prog.Flags&elf.PF_R != 0 {
				t.Errorf("Segment at Vaddr 0x%X has Flags %v, expected PF_W to be set", prog.Vaddr, prog.Flags)
			}
		}
	}
}

func TestExtractAndEncryptStrings(t *testing.T) {
	// Create a dummy ELF and Translator context to test the string extraction heuristics
	packer := NewPacker("dummy", "", nil, nil, false, false, false, false, []byte{})
	packer.isARM32 = false

	// Dummy data buffer that simulates an ELF file
	// We'll place a dummy string "Hello" at offset 0x1000
	packer.data = make([]byte, 0x2000)
	strOffset := uint64(0x1000)
	copy(packer.data[strOffset:], []byte("Hello, VM!\x00"))

	// Create a dummy instruction array: ADRP X0, page; ADD X0, X0, offset
	// Let's say page base is 0x400000, target is 0x401000.
	insts := []vm.Instruction{
		{Op: int(arm64.ADRP), Rd: 0, Imm: 0x1000, Offset: 0},
		{Op: int(arm64.ADD_IMM), Rd: 0, Rn: 0, Imm: 0x000, Offset: 4},
	}

	fi := &vm.FuncInfo{Addr: 0x400000}

	// We need to mock resolveFileOffsetBase, but it relies on f.Progs or f.Sections.
	// We'll create a minimal fake elf.File
	f := new(elf.File)
	f.Sections = append(f.Sections, &elf.Section{
		SectionHeader: elf.SectionHeader{
			Name: ".rodata",
			Type: elf.SHT_PROGBITS,
			Addr: 0x401000,
			Size: 0x100,
			Offset: strOffset,
		},
	})

	refs := packer.extractAndEncryptStrings(f, fi, insts)

	if len(refs) != 1 {
		t.Fatalf("Expected 1 string reference, got %d", len(refs))
	}

	ref := refs[0]
	if ref.Addr != 0x401000 {
		t.Errorf("Expected string addr 0x401000, got 0x%X", ref.Addr)
	}
	if ref.Len != 10 { // "Hello, VM!" is 10 chars
		t.Errorf("Expected string length 10, got %d", ref.Len)
	}

	// Verify encryption
	decrypted := make([]byte, ref.Len)
	for i := uint32(0); i < ref.Len; i++ {
		decrypted[i] = packer.data[strOffset+uint64(i)] ^ byte(ref.Key)
	}

	if string(decrypted) != "Hello, VM!" {
		t.Errorf("Decryption failed. Expected 'Hello, VM!', got '%s'", string(decrypted))
	}

	// Ensure null terminator was encrypted correctly
	if packer.data[strOffset+uint64(ref.Len)] != byte(ref.Key) {
		t.Errorf("Null terminator was not encrypted with the key.")
	}
}
