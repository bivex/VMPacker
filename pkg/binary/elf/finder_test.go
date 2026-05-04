package elf

import (
	"bytes"
	"debug/elf"
	"os"
	"path/filepath"
	"testing"
)

func TestParseAddrSpec(t *testing.T) {
	tests := []struct {
		input    string
		expected AddrSpec
		hasErr   bool
	}{
		{"0x400100", AddrSpec{Addr: 0x400100, End: 0, Name: "sub_400100"}, false},
		{"0x400100-0x400200", AddrSpec{Addr: 0x400100, End: 0x400200, Name: "sub_400100"}, false},
		{"0x400100-0x400200:my_func", AddrSpec{Addr: 0x400100, End: 0x400200, Name: "my_func"}, false},
		{"4194560", AddrSpec{Addr: 4194560, End: 0, Name: "sub_400100"}, false},
		{"invalid", AddrSpec{}, true},
		{"0x400200-0x400100", AddrSpec{}, true}, // end < start
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			res, err := ParseAddrSpec(tc.input)
			if tc.hasErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if res.Addr != tc.expected.Addr || res.End != tc.expected.End || res.Name != tc.expected.Name {
				t.Errorf("for input %q: expected %+v, got %+v", tc.input, tc.expected, res)
			}
		})
	}
}

func getDemoFile(t *testing.T) (*elf.File, *Packer, []byte) {
	projectRoot := filepath.Join("..", "..", "..")
	inputPath := filepath.Join(projectRoot, "demo", "demo_simple")
	
	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Skipf("Skipping test because demo binary not found at %s", inputPath)
	}

	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Failed to parse demo binary: %v", err)
	}

	packer := NewPacker(inputPath, "", nil, nil, false, false, false, false, []byte{})
	packer.data = data
	_, err = packer.validateArch(f)
	if err != nil {
		t.Fatalf("validateArch failed: %v", err)
	}

	return f, packer, data
}

func TestFindFunction(t *testing.T) {
	f, packer, _ := getDemoFile(t)
	defer f.Close()

	// Find an existing function
	fi, err := packer.FindFunction(f, "check_simple")
	if err != nil {
		t.Fatalf("FindFunction failed: %v", err)
	}

	if fi.Name != "check_simple" {
		t.Errorf("Expected func name 'check_simple', got '%s'", fi.Name)
	}
	if fi.Size == 0 {
		t.Errorf("Expected func size > 0, got 0")
	}

	// Try to find a non-existent function
	_, err = packer.FindFunction(f, "non_existent_func")
	if err == nil {
		t.Fatalf("Expected error when searching for non_existent_func")
	}
}

func TestFindFunctionByAddr(t *testing.T) {
	f, packer, _ := getDemoFile(t)
	defer f.Close()

	// First, dynamically resolve the address of check_simple
	fiOrig, err := packer.FindFunction(f, "check_simple")
	if err != nil {
		t.Fatalf("Failed to get check_simple info: %v", err)
	}

	spec := AddrSpec{
		Addr: fiOrig.Addr,
		End:  fiOrig.Addr + fiOrig.Size,
		Name: "test_check_simple",
	}

	fi, err := packer.FindFunctionByAddr(f, spec)
	if err != nil {
		t.Fatalf("FindFunctionByAddr failed: %v", err)
	}

	if fi.Addr != fiOrig.Addr {
		t.Errorf("Expected Addr 0x%X, got 0x%X", fiOrig.Addr, fi.Addr)
	}
	if fi.Size != fiOrig.Size {
		t.Errorf("Expected Size %d, got %d", fiOrig.Size, fi.Size)
	}
	if fi.Name != "test_check_simple" {
		t.Errorf("Expected Name 'test_check_simple', got '%s'", fi.Name)
	}

	// Auto-detect size
	specAuto := AddrSpec{
		Addr: fiOrig.Addr,
		End:  0, // 0 = auto-detect
		Name: "test_auto",
	}

	fiAuto, err := packer.FindFunctionByAddr(f, specAuto)
	if err != nil {
		t.Fatalf("FindFunctionByAddr (auto) failed: %v", err)
	}

	if fiAuto.Size == 0 {
		t.Errorf("Auto-detected size is 0")
	}
}

func TestExtractFuncCode(t *testing.T) {
	f, packer, _ := getDemoFile(t)
	defer f.Close()

	fi, err := packer.FindFunction(f, "check_simple")
	if err != nil {
		t.Fatalf("FindFunction failed: %v", err)
	}

	code, err := packer.ExtractFuncCode(f, fi)
	if err != nil {
		t.Fatalf("ExtractFuncCode failed: %v", err)
	}

	if len(code) != int(fi.Size) {
		t.Errorf("Expected code length %d, got %d", fi.Size, len(code))
	}
}
