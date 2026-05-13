package elf

import (
	"github.com/vmpacker/pkg/vm"
)

// AddrSpec specifies a function by address
type AddrSpec struct {
	Addr uint64
	End  uint64 // 0 = auto-detect
	Name string // optional name
}

// FuncBytecode stores the encrypted bytecode and meta-information for a single function
type FuncBytecode struct {
	FI          *vm.FuncInfo
	Encrypted   []byte
	XorKey      byte
	Relocations []vm.Relocation
}

// Packer is an ELF VMP packer
type Packer struct {
	inputPath       string
	outputPath      string
	funcNames       []string
	addrSpecs       []AddrSpec
	verbose         bool
	stripSymbols    bool
	debug           bool
	tokenEntry      bool // Token entry mode
	data            []byte
	interpBlob      []byte          // ARM64 blob
	interpBlobARM32 []byte          // ARM32 blob (optional)
	interpBlobX86_64 []byte         // x86_64 blob (optional)
	isARM32         bool            // detected at Process() time
	isX86_64        bool            // detected at Process() time
	thumbFuncs      map[uint64]bool // Thumb-mode function addresses (bit0 stripped)
	relocations     []vm.Relocation // relocations to be fixed at runtime (mainly for .so ASLR)
	cff             bool            // Control Flow Flattening
	mba             bool            // Mixed Boolean-Arithmetic
	mangleSymbols     bool            // Symbol mangling
	hybrid            bool            // Hybrid mode (x86_64 only)
	encryptedStrings  map[uint64]bool // strings already encrypted in-place (prevents double-encrypt)
}
