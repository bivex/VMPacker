package main

import (
	"os"
	"path/filepath"
	"testing"

	elfpacker "github.com/vmpacker/pkg/binary/elf"
	"github.com/vmpacker/pkg/vm"
)

func TestPackerEndToEnd(t *testing.T) {
	// Generate dynamic ISA since the packer translates to it
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// Locate the demo_simple binary
	// In cmd/vmpacker, the project root is two levels up
	projectRoot := filepath.Join("..", "..")
	inputPath := filepath.Join(projectRoot, "demo", "demo_simple")
	
	// Ensure the demo binary exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Skipping test because input binary %s does not exist. Run 'make' or './rebuild-stub.sh' first.", inputPath)
	}

	// Create a temporary output file
	tmpOut, err := os.CreateTemp("", "demo_simple_vmp_*")
	if err != nil {
		t.Fatalf("Failed to create temp output file: %v", err)
	}
	outPath := tmpOut.Name()
	tmpOut.Close()
	defer os.Remove(outPath)

	// Functions to protect
	funcs := []string{"check_simple"}
	var addrSpecs []elfpacker.AddrSpec

	// Initialize the Packer (using the embedded interpBlob from main.go)
	packer := elfpacker.NewPacker(
		inputPath, 
		outPath, 
		funcs, 
		addrSpecs, 
		false, // verbose
		false, // strip
		false, // debug
		false, // tokenEntry
		interpBlob,
	)

	// Set ARM32 interp blob (it might be empty, but that's fine for an ARM64 binary)
	packer.SetInterpBlobARM32(interpBlobARM32)

	// Run the packing process
	if err := packer.Process(); err != nil {
		t.Fatalf("Packer.Process() failed: %v", err)
	}

	// Verify that the output file was created and is larger than 0
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("Failed to stat output file: %v", err)
	}

	if info.Size() == 0 {
		t.Fatalf("Output file is empty")
	}
	
	t.Logf("Successfully packed %s to %s (size: %d bytes)", inputPath, outPath, info.Size())
}

func TestPackerCFF(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	projectRoot := filepath.Join("..", "..")
	inputPath := filepath.Join(projectRoot, "demo", "demo_simple")
	
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Skipping test because input binary %s does not exist", inputPath)
	}

	tmpOut, _ := os.CreateTemp("", "demo_simple_cff_*")
	outPath := tmpOut.Name()
	tmpOut.Close()
	defer os.Remove(outPath)

	packer := elfpacker.NewPacker(inputPath, outPath, []string{"check_simple"}, nil, false, false, false, false, interpBlob)
	packer.SetCFF(true) // Enable CFF!
	
	if err := packer.Process(); err != nil {
		t.Fatalf("Packer.Process() with CFF failed: %v", err)
	}

	info, _ := os.Stat(outPath)
	if info.Size() == 0 {
		t.Fatalf("Output file is empty")
	}
	
	t.Logf("Successfully packed with CFF: %s (size: %d bytes)", outPath, info.Size())
}
