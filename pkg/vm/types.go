package vm

// ============================================================
// Common types + interface definitions
//
// All architecture decoders and translators follow these interfaces,
// enabling future expansion to new architectures (x86, RISC-V)
// or binary formats (PE, Mach-O).
// ============================================================

// REG_XZR ARM64 zero register marker.
// In ARM64, register 31 can be SP or XZR depending on instruction type.
// Decoder replaces reg=31 with this value after decoding XZR context,
// translator's mapReg handles it uniformly.
const REG_XZR = -2

// REG_V_BASE SIMD/FP register (V0-V31) base offset
const REG_V_BASE = 64

// Instruction - architecture-agnostic instruction representation
type Instruction struct {
	Raw    uint32
	Op     int
	Rd     int // destination register
	Rn     int // first source register
	Rm     int // second source register
	Imm    int64
	Shift     int
	ShiftType int // 0=LSL, 1=LSR, 2=ASR, 3=ROR
	Cond      int
	SF     bool // 64-bit (true) vs 32-bit (false)
	Offset int  // offset within function
	WB     int  // Writeback mode (0=none, 1=post, 3=pre)
}

// Decoder - architecture decoder interface
type Decoder interface {
	// Decode - decode a raw instruction
	Decode(raw uint32, offset int) Instruction
	// InstName - return instruction name
	InstName(op int) string
}

// Relocation represents runtime relocation needs in bytecode
// (mainly for Android .so ASLR)
type Relocation struct {
	BcOffset   int    // offset of location to patch in bytecode
	TargetAddr uint64 // target absolute address at link time (runtime_addr = TargetAddr + slide)
	IsInternal bool   // whether target is within the same protected function
}

// TranslateResult - translation result
type TranslateResult struct {
	Bytecode    []byte
	CodeLen     int // raw bytecode length (excluding trailer)
	Unsupported []string
	TotalInsts  int
	TransInsts  int
	Relocations []Relocation
}

// Translator - bytecode translator interface
type Translator interface {
	// Translate - translate a set of instructions to VM bytecode
	Translate(instructions []Instruction) (*TranslateResult, error)
}

// FuncInfo - function metadata
type FuncInfo struct {
	Name    string
	Addr    uint64
	Size    uint64
	Offset  uint64
	Section string
}

// FuncBytecode - encrypted bytecode
type FuncBytecode struct {
	Info      *FuncInfo
	Encrypted []byte
	XorKey    byte
}

// Packer - binary format injector interface
type Packer interface {
	// Process - execute the full VMP protection flow
	Process() error
}
