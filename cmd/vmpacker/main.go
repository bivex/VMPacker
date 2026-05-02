package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"

	elfpacker "github.com/vmpacker/pkg/binary/elf"
)

// ============================================================
// vmpacker - ARM64 ELF VMP Protection Tool (Modular Version)
//
// Usage:
//   vmpacker -func check_license [-v] [-o output] input.elf
//   vmpacker -info input.elf
//
// Features:
//   Reads compiled ARM64 ELF, decodes specified functions' instructions,
//   translates them into custom VM bytecode, and replaces original functions with VM trampolines.
// ============================================================

//go:embed vm_interp.bin
var interpBlob []byte

func main() {
	funcList := flag.String("func", "", "Function names to protect (comma-separated)")
	addrList := flag.String("addr", "", "Protect by address (format: 0xADDR:SIZE[:name], comma-separated)")
	output := flag.String("o", "", "Output file path (default: original_name.vmp)")
	verbose := flag.Bool("v", false, "Verbose output (show disassembly)")
	strip := flag.Bool("strip", true, "Strip symbol table (prevents strip from breaking protection)")
	debug := flag.Bool("debug", false, "Generate debug mapping file (ARM64 → VM bytecode mapping)")
	tokenEntry := flag.Bool("token", true, "Enable Tokenized entry mode (3-instruction trampoline) — enabled by default")
	info := flag.Bool("info", false, "Print ELF information only, do not protect")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `vmpacker - ARM64 ELF VMP Protection Tool

Usage:
  vmpacker -func <name> [-v] [-o output] <input.elf>
  vmpacker -addr <addr:size[:name]> [-v] [-o output] <input.elf>
  vmpacker -info <input.elf>

Options:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  vmpacker -func check_license -v -o protected.elf original.elf
  vmpacker -func check_license -token -v -o protected.elf original.elf
  vmpacker -func "check_license,verify_token" app.elf
  vmpacker -addr "0x4006AC-0x400790" app.elf
  vmpacker -addr "0x4006AC-0x400790:main" -func verify app.elf
  vmpacker -info app.elf
`)
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	inputPath := flag.Arg(0)

	// Check if input file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[!] File does not exist: %s\n", inputPath)
		os.Exit(1)
	}

	// Info only
	if *info {
		if err := elfpacker.PrintELFInfo(inputPath); err != nil {
			fmt.Fprintf(os.Stderr, "[!] %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Must specify functions
	if *funcList == "" && *addrList == "" {
		fmt.Fprintf(os.Stderr, "[!] Please specify functions to protect using -func or -addr\n")
		flag.Usage()
		os.Exit(1)
	}

	// Parse function names
	var funcs []string
	if *funcList != "" {
		for _, f := range strings.Split(*funcList, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				funcs = append(funcs, f)
			}
		}
	}

	// Parse address specs
	var addrSpecs []elfpacker.AddrSpec
	if *addrList != "" {
		for _, spec := range strings.Split(*addrList, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			as, err := elfpacker.ParseAddrSpec(spec)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[!] Invalid address format: %s — %v\n", spec, err)
				os.Exit(1)
			}
			addrSpecs = append(addrSpecs, as)
		}
	}

	// Output path
	outPath := *output
	if outPath == "" {
		outPath = inputPath + ".vmp"
	}

	// Execute
	fmt.Println("========================================")
	fmt.Println("  vmpacker - ARM64 ELF VMP Protection Tool")
	fmt.Println("========================================")
	fmt.Printf("[*] Input: %s\n", inputPath)
	fmt.Printf("[*] Output: %s\n", outPath)
	fmt.Printf("[*] Protecting functions: %v\n", funcs)
	fmt.Println()

	packer := elfpacker.NewPacker(inputPath, outPath, funcs, addrSpecs, *verbose, *strip, *debug, *tokenEntry, interpBlob)
	if err := packer.Process(); err != nil {
		fmt.Fprintf(os.Stderr, "\n[!] Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n[+] VMP Protection Complete!")
}
