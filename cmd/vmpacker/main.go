package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"

	elfpacker "github.com/vmpacker/pkg/binary/elf"
	"github.com/vmpacker/pkg/vm"
)

// ============================================================
// vmpacker - ARM64/ARM32 ELF VMP protection tool (modular version)
//
// Usage:
//   vmpacker -func check_license [-v] [-o output] input.elf
//   vmpacker -info input.elf
//
// Functionality:
//   Read a compiled ARM64/ARM32 ELF, decode instructions of specified
//   functions, translate to custom VM bytecode, replace original
//   functions with VM trampolines.
// ============================================================

//go:embed vm_interp.bin
var interpBlob []byte

//go:embed vm_interp_arm32.bin
var interpBlobARM32 []byte

func main() {
	funcList := flag.String("func", "", "comma-separated function names to protect")
	addrList := flag.String("addr", "", "protect by address (format: 0xADDR:SIZE[:name], comma-separated)")
	output := flag.String("o", "", "output file path (default: original.vmp)")
	verbose := flag.Bool("v", false, "verbose output (show disassembly)")
	strip := flag.Bool("strip", true, "strip symbol table (prevent strip from breaking protection)")
	debug := flag.Bool("debug", false, "generate debug mapping file (ARM64 -> VM bytecode)")
	tokenEntry := flag.Bool("token", true, "enable tokenized entry mode (3-inst trampoline) - default on")
	cff := flag.Bool("cff", false, "enable Control Flow Flattening (CFF) obfuscation")
	mba := flag.Bool("mba", false, "enable Mixed Boolean-Arithmetic (MBA) instruction substitution")
	mangle := flag.Bool("mangle", false, "enable Symbol Mangling for protected functions")
	info := flag.Bool("info", false, "print ELF info only, do not protect")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `vmpacker - ARM64/ARM32 ELF VMP protection tool

Usage:
  vmpacker -func <function> [-v] [-o output] <input.elf>
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
  vmpacker -addr "0x4006AC:0x400790" app.elf
  vmpacker -addr "0x4006AC:0x400790:main" -func verify app.elf
  vmpacker -info app.elf
`)
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	inputPath := flag.Arg(0)

	// check if input file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[!] File not found: %s\n", inputPath)
		os.Exit(1)
	}

	// info only mode
	if *info {
		if err := elfpacker.PrintELFInfo(inputPath); err != nil {
			fmt.Fprintf(os.Stderr, "[!] %v\n", err)
			os.Exit(1)
		}
		return
	}

	// function must be specified
	if *funcList == "" && *addrList == "" {
		fmt.Fprintf(os.Stderr, "[!] Use -func or -addr to specify functions to protect\n")
		flag.Usage()
		os.Exit(1)
	}

	// parse function name list
	var funcs []string
	if *funcList != "" {
		for _, f := range strings.Split(*funcList, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				funcs = append(funcs, f)
			}
		}
	}

	// parse address list
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

	// output path
	outPath := *output
	if outPath == "" {
		outPath = inputPath + ".vmp"
	}

	// execute
	fmt.Println("========================================")
	fmt.Println("  vmpacker - ARM64/ARM32 ELF VMP protection tool")
	fmt.Println("========================================")
	fmt.Printf("[*] Input:  %s\n", inputPath)
	fmt.Printf("[*] Output: %s\n", outPath)
	fmt.Printf("[*] Functions: %v\n", funcs)
	fmt.Println()

	// generate dynamic opcodes
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	packer := elfpacker.NewPacker(inputPath, outPath, funcs, addrSpecs, *verbose, *strip, *debug, *tokenEntry, interpBlob)
	packer.SetInterpBlobARM32(interpBlobARM32)
	packer.SetCFF(*cff)
	packer.SetMBA(*mba)
	packer.SetMangle(*mangle)
	if err := packer.Process(); err != nil {
		fmt.Fprintf(os.Stderr, "\n[!] Failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n[+] VMP protection complete!")
}
