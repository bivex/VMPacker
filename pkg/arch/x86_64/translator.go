package x86_64

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math/rand"
	"sort"

	"golang.org/x/arch/x86/x86asm"

	"github.com/vmpacker/pkg/vm"
)

// x86_64 Register IDs (matches vm_types.h)
const (
	X86_RAX = 0
	X86_RCX = 1
	X86_RDX = 2
	X86_RBX = 3
	X86_RSP = 4
	X86_RBP = 5
	X86_RSI = 6
	X86_RDI = 7
	X86_R8  = 8
	X86_R9  = 9
	X86_R10 = 10
	X86_R11 = 11
	X86_R12 = 12
	X86_R13 = 13
	X86_R14 = 14
	X86_R15 = 15
	X86_RIP = 16
)

var x86RegMap = map[x86asm.Reg]int{
	x86asm.RAX: X86_RAX, x86asm.EAX: X86_RAX, x86asm.AX: X86_RAX, x86asm.AL: X86_RAX,
	x86asm.RCX: X86_RCX, x86asm.ECX: X86_RCX, x86asm.CX: X86_RCX, x86asm.CL: X86_RCX,
	x86asm.RDX: X86_RDX, x86asm.EDX: X86_RDX, x86asm.DX: X86_RDX, x86asm.DL: X86_RDX,
	x86asm.RBX: X86_RBX, x86asm.EBX: X86_RBX, x86asm.BX: X86_RBX, x86asm.BL: X86_RBX,
	x86asm.RSP: X86_RSP, x86asm.ESP: X86_RSP, x86asm.SP: X86_RSP,
	x86asm.RBP: X86_RBP, x86asm.EBP: X86_RBP, x86asm.BP: X86_RBP,
	x86asm.RSI: X86_RSI, x86asm.ESI: X86_RSI, x86asm.SI: X86_RSI,
	x86asm.RDI: X86_RDI, x86asm.EDI: X86_RDI, x86asm.DI: X86_RDI,
	x86asm.R8: X86_R8,
	x86asm.R9: X86_R9,
	x86asm.R10: X86_R10,
	x86asm.R11: X86_R11,
	x86asm.R12: X86_R12,
	x86asm.R13: X86_R13,
	x86asm.R14: X86_R14,
	x86asm.R15: X86_R15,
	x86asm.RIP: X86_RIP,
}

type branchFixup struct {
	vmOffset    int
	x86Target   int
	isRelToFunc bool
}

// Translator for x86_64
type Translator struct {
	code        []byte
	labels      map[int]int
	fixups      []branchFixup
	relocations []vm.Relocation
	funcSize    int
	funcAddr    uint64
	unsupported []string
	debug       bool
	regMap      [32]byte
	ocKey       uint32
}

// NewTranslator creates a new x86_64 translator
func NewTranslator(funcAddr uint64, funcSize int, code []byte) *Translator {
	t := &Translator{
		code:        make([]byte, 0, funcSize*4),
		labels:      make(map[int]int),
		relocations: make([]vm.Relocation, 0),
		funcAddr:    funcAddr,
		funcSize:    funcSize,
		ocKey:       rand.Uint32(),
	}

	// Initialize register map (shuffled for obfuscation)
	for i := 0; i < 32; i++ {
		t.regMap[i] = byte(i)
	}
	// Shuffle R0-R15 (General purpose)
	rand.Shuffle(16, func(i, j int) {
		t.regMap[i], t.regMap[j] = t.regMap[j], t.regMap[i]
	})

	return t
}

func (t *Translator) SetDebug(debug bool) {
	t.debug = debug
}

// emit appends bytes to output
func (t *Translator) emit(b ...byte) {
	t.code = append(t.code, b...)
}

func (t *Translator) emitU32(v uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	t.emit(b...)
}

func (t *Translator) emitU64(v uint64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	t.emit(b...)
}

func (t *Translator) reg(r x86asm.Reg) byte {
	id, ok := x86RegMap[r]
	if !ok {
		return 0xFF
	}
	return t.regMap[id]
}

func (t *Translator) pos() int {
	return len(t.code)
}

// Translate converts x86_64 instructions to VM bytecode
func (t *Translator) Translate(insts []vm.Instruction) (*vm.TranslateResult, error) {
	t.labels = make(map[int]int)
	t.fixups = nil

	for _, inst := range insts {
		t.labels[inst.Offset] = t.pos()
		
		xInst, err := x86asm.Decode(inst.RawBytes, 64)
		if err != nil {
			t.unsupported = append(t.unsupported, fmt.Sprintf("offset 0x%X: decode error: %v", inst.Offset, err))
			t.emit(vm.OpHalt)
			continue
		}

		if err := t.translateInst(xInst, inst.Offset); err != nil {
			t.unsupported = append(t.unsupported, fmt.Sprintf("offset 0x%X: %v (%s)", inst.Offset, err, inst.Str))
			t.emit(vm.OpHalt)
		}
	}

	return t.finishTranslate()
}

func (t *Translator) finishTranslate() (*vm.TranslateResult, error) {
	t.labels[t.funcSize] = t.pos()
	t.emit(vm.OpHalt)

	for _, fix := range t.fixups {
		target, ok := t.labels[fix.x86Target]
		if !ok {
			// Cross-function jump or outside range?
			continue
		}
		binary.LittleEndian.PutUint32(t.code[fix.vmOffset:], uint32(target))
	}

	codeLen := t.pos()

	// ---- append CRC section ----
	bcCrc := crc32.ChecksumIEEE(t.code[:codeLen])
	t.emitU64(0)          // stub_va placeholder
	t.emitU32(0)          // stub_size placeholder
	t.emitU32(0)          // stub_crc placeholder
	t.emitU32(bcCrc)      // bc_crc
	t.emitU32(0x43524332) // CRC_MAGIC

	// ---- append trailer ----
	t.emit(t.regMap[:]...)
	t.emit(vm.GlobalOpMap[:]...)

	// Address map
	var sortedOffsets []int
	for off := range t.labels {
		sortedOffsets = append(sortedOffsets, off)
	}
	sort.Ints(sortedOffsets)
	for _, off := range sortedOffsets {
		t.emitU32(uint32(off))
		t.emitU32(uint32(t.labels[off]))
	}

	// Final trailer info
	t.emit(0) // reverse flag
	t.emitU32(t.ocKey)
	t.emitU32(uint32(len(sortedOffsets)))
	t.emitU64(t.funcAddr)
	t.emitU32(uint32(t.funcSize))

	// Encrypt opcodes
	if t.ocKey != 0 {
		for i := 0; i < codeLen; i++ {
			t.code[i] ^= byte(t.ocKey ^ uint32(i*0x9E3779B9))
		}
	}

	res := &vm.TranslateResult{
		Bytecode:    t.code,
		CodeLen:     codeLen,
		Unsupported: t.unsupported,
		TotalInsts:  len(t.labels) - 1,
		TransInsts:  len(t.labels) - 1 - len(t.unsupported),
		Relocations: t.relocations,
	}

	return res, nil
}

func (t *Translator) translateInst(inst x86asm.Inst, offset int) error {
	switch inst.Op {
	case x86asm.PUSH:
		return t.trPush(inst)
	case x86asm.POP:
		return t.trPop(inst)
	case x86asm.MOV:
		return t.trMov(inst, offset)
	case x86asm.LEA:
		return t.trLea(inst, offset)
	case x86asm.ADD, x86asm.SUB, x86asm.XOR, x86asm.AND, x86asm.OR:
		return t.trAlu(inst)
	case x86asm.CMP:
		return t.trCmp(inst)
	case x86asm.RET:
		t.emit(vm.OpRet, t.regMap[X86_RAX])
		return nil
	case x86asm.NOP:
		t.emit(vm.OpNop)
		return nil
	case x86asm.CALL:
		return t.trCall(inst, offset)
	case x86asm.JMP:
		return t.trJmp(inst, offset)
	default:
		name := inst.Op.String()
		if len(name) > 0 && name[0] == 'J' {
			return t.trJcc(inst, offset)
		}
		return fmt.Errorf("unsupported opcode: %v", inst.Op)
	}
}

func (t *Translator) trPush(inst x86asm.Inst) error {
	if reg, ok := inst.Args[0].(x86asm.Reg); ok {
		t.emit(vm.OpPush, t.reg(reg))
		return nil
	}
	return fmt.Errorf("unsupported PUSH arg")
}

func (t *Translator) trPop(inst x86asm.Inst) error {
	if reg, ok := inst.Args[0].(x86asm.Reg); ok {
		t.emit(vm.OpPop, t.reg(reg))
		return nil
	}
	return fmt.Errorf("unsupported POP arg")
}

func (t *Translator) trMov(inst x86asm.Inst, offset int) error {
	dst := inst.Args[0]
	src := inst.Args[1]

	if dReg, ok := dst.(x86asm.Reg); ok {
		if sReg, ok := src.(x86asm.Reg); ok {
			t.emit(vm.OpMovReg, t.reg(dReg), t.reg(sReg))
			return nil
		}
		if imm, ok := src.(x86asm.Imm); ok {
			t.emit(vm.OpMovImm, t.reg(dReg))
			t.emitU64(uint64(imm))
			return nil
		}
		if mem, ok := src.(x86asm.Mem); ok {
			t.emitMemAddr(mem, inst, offset)
			t.emit(vm.OpSLd64)
			t.emit(vm.OpSVstore, t.reg(dReg))
			return nil
		}
	}

	if dMem, ok := dst.(x86asm.Mem); ok {
		if sReg, ok := src.(x86asm.Reg); ok {
			t.emit(vm.OpSVload, t.reg(sReg))
			t.emitMemAddr(dMem, inst, offset)
			t.emit(vm.OpSSt64)
			return nil
		}
		if imm, ok := src.(x86asm.Imm); ok {
			t.emit(vm.OpSPushImm64)
			t.emitU64(uint64(imm))
			t.emitMemAddr(dMem, inst, offset)
			t.emit(vm.OpSSt64)
			return nil
		}
	}

	return fmt.Errorf("unsupported MOV")
}

func (t *Translator) trLea(inst x86asm.Inst, offset int) error {
	dst, ok1 := inst.Args[0].(x86asm.Reg)
	mem, ok2 := inst.Args[1].(x86asm.Mem)
	if !ok1 || !ok2 {
		return fmt.Errorf("unsupported LEA args")
	}
	t.emitMemAddr(mem, inst, offset)
	t.emit(vm.OpSVstore, t.reg(dst))
	return nil
}

func (t *Translator) trAlu(inst x86asm.Inst) error {
	dst, ok := inst.Args[0].(x86asm.Reg)
	if !ok {
		return fmt.Errorf("unsupported ALU dest")
	}
	src := inst.Args[1]
	var op byte
	switch inst.Op {
	case x86asm.ADD: op = vm.OpSAdd
	case x86asm.SUB: op = vm.OpSSub
	case x86asm.XOR: op = vm.OpSXor
	case x86asm.AND: op = vm.OpSAnd
	case x86asm.OR:  op = vm.OpSOr
	}
	if sReg, ok := src.(x86asm.Reg); ok {
		t.emit(vm.OpSVload, t.reg(sReg))
		t.emit(vm.OpSVload, t.reg(dst))
		t.emit(op)
		t.emit(vm.OpSVstore, t.reg(dst))
		return nil
	}
	if imm, ok := src.(x86asm.Imm); ok {
		t.emit(vm.OpSPushImm64)
		t.emitU64(uint64(imm))
		t.emit(vm.OpSVload, t.reg(dst))
		t.emit(op)
		t.emit(vm.OpSVstore, t.reg(dst))
		return nil
	}
	return fmt.Errorf("unsupported ALU src")
}

func (t *Translator) trCmp(inst x86asm.Inst) error {
	a, ok := inst.Args[0].(x86asm.Reg)
	if !ok {
		return fmt.Errorf("unsupported CMP dest")
	}
	b := inst.Args[1]
	if bReg, ok := b.(x86asm.Reg); ok {
		t.emit(vm.OpSVload, t.reg(bReg))
		t.emit(vm.OpSVload, t.reg(a))
		t.emit(vm.OpSCmp)
		return nil
	}
	if imm, ok := b.(x86asm.Imm); ok {
		t.emit(vm.OpSPushImm64)
		t.emitU64(uint64(imm))
		t.emit(vm.OpSVload, t.reg(a))
		t.emit(vm.OpSCmp)
		return nil
	}
	return fmt.Errorf("unsupported CMP src")
}

func (t *Translator) trCall(inst x86asm.Inst, offset int) error {
	if imm, ok := inst.Args[0].(x86asm.Imm); ok {
		target := int(int64(offset) + int64(inst.Len) + int64(imm))
		t.emit(vm.OpCallNative)
		t.emitU64(uint64(target))
		return nil
	}
	return fmt.Errorf("unsupported CALL")
}

func (t *Translator) trJmp(inst x86asm.Inst, offset int) error {
	if imm, ok := inst.Args[0].(x86asm.Imm); ok {
		target := int(int64(offset) + int64(inst.Len) + int64(imm))
		t.emit(vm.OpJmp)
		t.fixups = append(t.fixups, branchFixup{vmOffset: t.pos(), x86Target: target})
		t.emitU32(0)
		return nil
	}
	return fmt.Errorf("unsupported JMP")
}

func (t *Translator) trJcc(inst x86asm.Inst, offset int) error {
	var op byte
	switch inst.Op {
	case x86asm.JE: op = vm.OpJe
	case x86asm.JNE: op = vm.OpJne
	case x86asm.JL: op = vm.OpJl
	case x86asm.JGE: op = vm.OpJge
	case x86asm.JLE: op = vm.OpJle
	case x86asm.JB: op = vm.OpJb
	case x86asm.JAE: op = vm.OpJae
	case x86asm.JBE: op = vm.OpJbe
	case x86asm.JA: op = vm.OpJa
	default: return fmt.Errorf("unsupported Jcc")
	}
	imm, ok := inst.Args[0].(x86asm.Imm)
	if !ok {
		return fmt.Errorf("unsupported Jcc arg")
	}
	target := int(int64(offset) + int64(inst.Len) + int64(imm))
	t.emit(op)
	t.fixups = append(t.fixups, branchFixup{vmOffset: t.pos(), x86Target: target})
	t.emitU32(0)
	return nil
}

func (t *Translator) emitMemAddr(mem x86asm.Mem, inst x86asm.Inst, instOffset int) {
	t.emit(vm.OpSPushImm64)
	disp := mem.Disp
	if mem.Base == x86asm.RIP {
		nextRIP := t.funcAddr + uint64(instOffset) + uint64(inst.Len)
		disp += int64(nextRIP)
	}
	t.emitU64(uint64(disp))
	if mem.Index != 0 {
		t.emit(vm.OpSVload, t.reg(mem.Index))
		if mem.Scale > 1 {
			t.emit(vm.OpSPushImm32)
			t.emitU32(uint32(mem.Scale))
			t.emit(vm.OpSMul)
		}
		t.emit(vm.OpSAdd)
	}
	if mem.Base != 0 && mem.Base != x86asm.RIP {
		t.emit(vm.OpSVload, t.reg(mem.Base))
		t.emit(vm.OpSAdd)
	}
}
