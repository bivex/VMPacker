package elf

import (
	"github.com/vmpacker/pkg/vm"
)

// AddrSpec 按地址指定函数
type AddrSpec struct {
	Addr uint64
	End  uint64 // 0 = 自动检测
	Name string // 可选名称
}

// FuncBytecode 保存单个函数的加密字节码和元信息
type FuncBytecode struct {
	FI          *vm.FuncInfo
	Encrypted   []byte
	XorKey      byte
	Relocations []vm.Relocation
}

// Packer ELF VMP 打包器
type Packer struct {
	inputPath       string
	outputPath      string
	funcNames       []string
	addrSpecs       []AddrSpec
	verbose         bool
	stripSymbols    bool
	debug           bool
	tokenEntry      bool // Token 化入口模式
	data            []byte
	interpBlob      []byte          // ARM64 blob
	interpBlobARM32 []byte          // ARM32 blob (optional)
	isARM32         bool            // detected at Process() time
	thumbFuncs      map[uint64]bool // Thumb-mode function addresses (bit0 stripped)
	relocations     []vm.Relocation // 运行时待修复的重定位 (主要是 .so ASLR)
}
