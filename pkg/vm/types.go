package vm

// ============================================================
// 公共类型 + 接口定义
//
// 所有架构解码器和翻译器都遵循这些接口，
// 以便将来扩展到新架构（x86, RISC-V）或新二进制格式（PE, Mach-O）。
// ============================================================

// REG_XZR ARM64 零寄存器标记值。
// 在 ARM64 中 register 31 根据指令类型可以是 SP 或 XZR。
// decoder 在解码后对 XZR 语境的 reg=31 替换为此值，
// translator 的 mapReg 统一处理。
const REG_XZR = -2

// REG_V_BASE SIMD/FP 寄存器 (V0-V31) 的起始偏移
const REG_V_BASE = 64

// Instruction 通用指令表示（架构无关）
type Instruction struct {
	Raw    uint32
	Op     int
	Rd     int // 目标寄存器
	Rn     int // 第一源寄存器
	Rm     int // 第二源寄存器
	Imm    int64
	Shift     int
	ShiftType int // 0=LSL, 1=LSR, 2=ASR, 3=ROR
	Cond      int
	SF     bool // 64-bit (true) vs 32-bit (false)
	Offset int  // 指令在函数内的偏移
	WB     int  // Writeback 模式 (0=无, 1=post, 3=pre)
}

// Decoder 架构解码器接口
type Decoder interface {
	// Decode 解码一条原始指令
	Decode(raw uint32, offset int) Instruction
	// InstName 返回指令名称
	InstName(op int) string
}

// Relocation 表示字节码中的运行时重定位需求 (主要用于 Android .so ASLR)
type Relocation struct {
	BcOffset   int    // 字节码中待修复的位置偏移
	TargetAddr uint64 // 链接时的目标绝对地址 (runtime_addr = TargetAddr + slide)
	IsInternal bool   // 目标是否在同一个被保护函数内
}

// TranslateResult 翻译结果
type TranslateResult struct {
	Bytecode    []byte
	CodeLen     int // 纯字节码长度 (不含 trailer)
	Unsupported []string
	TotalInsts  int
	TransInsts  int
	Relocations []Relocation
}

// Translator 字节码翻译器接口
type Translator interface {
	// Translate 将一组指令翻译为 VM 字节码
	Translate(instructions []Instruction) (*TranslateResult, error)
}

// FuncInfo 函数元信息
type FuncInfo struct {
	Name    string
	Addr    uint64
	Size    uint64
	Offset  uint64
	Section string
}

// FuncBytecode 加密后的字节码
type FuncBytecode struct {
	Info      *FuncInfo
	Encrypted []byte
	XorKey    byte
}

// Packer 二进制格式注入器接口
type Packer interface {
	// Process 执行完整的 VMP 保护流程
	Process() error
}
