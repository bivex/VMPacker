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
	X86_RAX  = 0
	X86_RCX  = 1
	X86_RDX  = 2
	X86_RBX  = 3
	X86_RSP  = 4
	X86_RBP  = 5
	X86_RSI  = 6
	X86_RDI  = 7
	X86_R8   = 8
	X86_R9   = 9
	X86_R10  = 10
	X86_R11  = 11
	X86_R12  = 12
	X86_R13  = 13
	X86_R14  = 14
	X86_R15  = 15
	X86_RIP  = 16
	X86_XMM0 = 32
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
	x86asm.R8:  X86_R8,
	x86asm.R9:  X86_R9,
	x86asm.R10: X86_R10,
	x86asm.R11: X86_R11,
	x86asm.R12: X86_R12,
	x86asm.R13: X86_R13,
	x86asm.R14: X86_R14,
	x86asm.R15: X86_R15,
	x86asm.RIP: X86_RIP,
}

func init() {
	// Add XMM0-XMM15 mapping (x86asm indices 88-103)
	for i := 0; i < 16; i++ {
		x86RegMap[x86asm.Reg(88+i)] = X86_XMM0 + i
	}
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
	regMap      [64]byte
	ocKey       uint32
	cff         bool
	mba         bool
	hybrid      bool // Hybrid mode: emit native code snippets
	maxMBADepth int
	bbStates    map[int]uint32
	dispPos     int
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
		maxMBADepth: 2,
		bbStates:    make(map[int]uint32),
	}

	// Initialize register map
	for i := 0; i < 64; i++ {
		t.regMap[i] = byte(i)
	}
	// Shuffle R0-R15
	rand.Shuffle(16, func(i, j int) {
		t.regMap[i], t.regMap[j] = t.regMap[j], t.regMap[i]
	})

	return t
}

func (t *Translator) SetDebug(debug bool) {
	t.debug = debug
}

func (t *Translator) SetCFF(enabled bool) {
	t.cff = enabled
}

func (t *Translator) SetMBA(enabled bool) {
	t.mba = enabled
}

func (t *Translator) SetHybrid(enabled bool) {
	t.hybrid = enabled
}

func (t *Translator) SetMaxMBADepth(depth int) {
	t.maxMBADepth = depth
}

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

func (t *Translator) emitU16(v uint16) {
	t.emit(byte(v), byte(v>>8))
}

// tryEmitNative attempts to emit the instruction as a native code chunk.
// It returns true if the native chunk was emitted and normal translation should be skipped.
func (t *Translator) tryEmitNative(raw []byte, inst x86asm.Inst) bool {
	if !t.hybrid {
		return false
	}
	// Disallow unsafe instructions that could break VM semantics.
	switch inst.Op {
	case x86asm.JMP, x86asm.CALL, x86asm.RET,
		x86asm.PUSH, x86asm.POP,
		x86asm.SYSCALL, x86asm.SYSENTER, x86asm.INT:
		return false
	}
	// Disallow any instruction whose mnemonic starts with 'J' (conditional jumps)
	name := inst.Op.String()
	if len(name) > 0 && name[0] == 'J' {
		return false
	}
	// 20% chance to emit native
	if rand.Float32() >= 0.2 {
		return false
	}
	// Emit: OpNativeExec, length (uint16), raw bytes, RET
	t.emit(vm.OpSNativeExec)
	length := uint16(len(raw))
	t.emitU16(length)
	t.emit(raw...)
	t.emit(0xC3) // RET
	return true
}

func (t *Translator) sPushImm32(v uint32) {
	t.emit(vm.OpSPushImm32)
	t.emitU32(v)
}

func (t *Translator) sPushImm64(v uint64) {
	t.emit(vm.OpSPushImm64)
	t.emitU64(v)
}

func (t *Translator) reg(r x86asm.Reg) byte {
	id, ok := x86RegMap[r]
	if !ok {
		return 0xFF
	}
	if id >= 32 && id < 64 {
		return byte(id - 32) // Map XMM0-15 to V0-V15
	}
	return t.regMap[id]
}

func (t *Translator) pos() int {
	return len(t.code)
}

func (t *Translator) Translate(insts []vm.Instruction) (*vm.TranslateResult, error) {
	t.labels = make(map[int]int)
	t.fixups = nil

	if t.cff {
		return t.translateCFF(insts)
	}

	for _, inst := range insts {
		t.labels[inst.Offset] = t.pos()
		xInst, err := x86asm.Decode(inst.RawBytes, 64)
		if err != nil {
			t.unsupported = append(t.unsupported, fmt.Sprintf("offset 0x%X: decode error", inst.Offset))
			t.emit(vm.OpHalt)
			continue
		}
		// Attempt hybrid native emission before normal translation
		if t.hybrid && t.tryEmitNative(inst.RawBytes, xInst) {
			continue
		}
		if err := t.translateInst(xInst, inst.Offset); err != nil {
			t.unsupported = append(t.unsupported, fmt.Sprintf("offset 0x%X: %v", inst.Offset, err))
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
			return nil, fmt.Errorf("branch target 0x%X not found", fix.x86Target)
		}
		binary.LittleEndian.PutUint32(t.code[fix.vmOffset:], uint32(target))
	}

	codeLen := t.pos()
	bcCrc := crc32.ChecksumIEEE(t.code[:codeLen])
	t.emitU64(0)          // stub_va
	t.emitU32(0)          // stub_size
	t.emitU32(0)          // stub_crc
	t.emitU32(bcCrc)      // bc_crc
	t.emitU32(0x43524332) // CRC_MAGIC

	t.emit(t.regMap[:]...)
	t.emit(vm.GlobalOpMap[:]...)

	var sortedOffsets []int
	for off := range t.labels {
		sortedOffsets = append(sortedOffsets, off)
	}
	sort.Ints(sortedOffsets)
	for _, off := range sortedOffsets {
		t.emitU32(uint32(off))
		t.emitU32(uint32(t.labels[off]))
	}

	t.emit(0) // reverse
	t.emitU32(t.ocKey)
	t.emitU32(uint32(len(sortedOffsets)))
	t.emitU64(t.funcAddr)
	t.emitU32(uint32(t.funcSize))

	if t.ocKey != 0 {
		for i := 0; i < codeLen; i++ {
			t.code[i] ^= byte(t.ocKey ^ uint32(i*0x9E3779B9))
		}
	}

	return &vm.TranslateResult{
		Bytecode: t.code, CodeLen: codeLen, Unsupported: t.unsupported,
		TotalInsts: len(t.labels) - 1, TransInsts: len(t.labels) - 1 - len(t.unsupported),
		Relocations: t.relocations,
	}, nil
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
	case x86asm.ADD, x86asm.SUB, x86asm.XOR, x86asm.AND, x86asm.OR, x86asm.TEST:
		return t.trAlu(inst)
	case x86asm.IMUL, x86asm.IDIV, x86asm.MUL, x86asm.DIV:
		return t.trMulDiv(inst)
	case x86asm.ADDSS, x86asm.ADDSD, x86asm.SUBSS, x86asm.SUBSD, x86asm.MULSS, x86asm.MULSD, x86asm.DIVSS, x86asm.DIVSD:
		return t.trFpAlu(inst)
	case x86asm.MOVSS, x86asm.MOVSD, x86asm.MOVAPS, x86asm.MOVAPD, x86asm.MOVUPS, x86asm.MOVUPD, x86asm.MOVDQU, x86asm.MOVDQA:
		return t.trFpMov(inst, offset)
	case x86asm.UCOMISS, x86asm.UCOMISD:
		return t.trFpCmp(inst)
	case x86asm.CVTSI2SS, x86asm.CVTSI2SD, x86asm.CVTTSS2SI, x86asm.CVTTSD2SI:
		return t.trFpCvt(inst)
	case x86asm.XORPS, x86asm.XORPD, x86asm.ANDPS, x86asm.ANDPD, x86asm.ORPS, x86asm.ORPD:
		return t.trFpBitwise(inst)
	case x86asm.MOVSX, x86asm.MOVSXD, x86asm.MOVZX:
		return t.trMovExt(inst, offset)
	case x86asm.BT:
		return t.trBT(inst)
	case x86asm.SETL, x86asm.SETLE, x86asm.SETG, x86asm.SETGE, x86asm.SETE, x86asm.SETNE, x86asm.SETB, x86asm.SETBE, x86asm.SETA, x86asm.SETAE:
		return t.trSetcc(inst)
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
	return fmt.Errorf("unsupported PUSH")
}

func (t *Translator) trPop(inst x86asm.Inst) error {
	if reg, ok := inst.Args[0].(x86asm.Reg); ok {
		t.emit(vm.OpPop, t.reg(reg))
		return nil
	}
	return fmt.Errorf("unsupported POP")
}

func (t *Translator) trMov(inst x86asm.Inst, offset int) error {
	dst, src := inst.Args[0], inst.Args[1]
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
			t.sPushImm64(uint64(imm))
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
		return fmt.Errorf("unsupported LEA")
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
	case x86asm.ADD:
		op = vm.OpSAdd
	case x86asm.SUB:
		op = vm.OpSSub
	case x86asm.XOR:
		op = vm.OpSXor
	case x86asm.AND, x86asm.TEST:
		op = vm.OpSAnd
	case x86asm.OR:
		op = vm.OpSOr
	}
	pushX := func() {}
	pushY := func() { t.emit(vm.OpSVload, t.reg(dst)) }
	if sReg, ok := src.(x86asm.Reg); ok {
		pushX = func() { t.emit(vm.OpSVload, t.reg(sReg)) }
	} else if imm, ok := src.(x86asm.Imm); ok {
		pushX = func() { t.sPushImm64(uint64(imm)) }
	} else {
		return fmt.Errorf("unsupported ALU src")
	}

	if inst.Op == x86asm.TEST {
		pushX()
		pushY()
		t.emit(vm.OpSAnd)
		t.sPushImm64(0)
		t.emit(vm.OpSCmp)
		return nil
	}

	if inst.Op == x86asm.SUB && !t.mba {
		// Special case for SUB to get correct flags (including CF)
		pushX()
		pushY()
		t.emit(vm.OpSCmp)
		pushX()
		pushY()
		t.emit(vm.OpSSub)
		t.emit(vm.OpSVstore, t.reg(dst))
		return nil
	}

	if !t.emitStackMBA(op, pushX, pushY) {
		pushX()
		pushY()
		t.emit(op)
	}

	// Update ZF and SF flags based on result
	t.emit(vm.OpSDup)
	t.sPushImm64(0)
	t.emit(vm.OpSCmp)

	t.emit(vm.OpSVstore, t.reg(dst))
	return nil
}

func (t *Translator) trMulDiv(inst x86asm.Inst) error {
	if inst.Args[1] == nil {
		src, ok := inst.Args[0].(x86asm.Reg)
		if !ok {
			return fmt.Errorf("unsupported 1-op mul/div")
		}
		t.emit(vm.OpSVload, t.reg(src))
		t.emit(vm.OpSVload, t.regMap[X86_RAX])
		if inst.Op == x86asm.IMUL || inst.Op == x86asm.MUL {
			t.emit(vm.OpSDup)
			t.emit(vm.OpSVload, t.reg(src))
			if inst.Op == x86asm.IMUL {
				t.emit(vm.OpSSmulh)
			} else {
				t.emit(vm.OpSUmulh)
			}
			t.emit(vm.OpSVstore, t.regMap[X86_RDX])
			t.emit(vm.OpSMul)
			t.emit(vm.OpSVstore, t.regMap[X86_RAX])
		} else {
			if inst.Op == x86asm.IDIV {
				t.emit(vm.OpSSdiv)
			} else {
				t.emit(vm.OpSUdiv)
			}
			t.emit(vm.OpSVstore, t.regMap[X86_RAX])
		}
		return nil
	}
	dst, ok1 := inst.Args[0].(x86asm.Reg)
	if !ok1 {
		return fmt.Errorf("unsupported 2-op mul dest")
	}
	if sReg, ok := inst.Args[1].(x86asm.Reg); ok {
		t.emit(vm.OpSVload, t.reg(sReg))
		t.emit(vm.OpSVload, t.reg(dst))
		t.emit(vm.OpSMul)
		t.emit(vm.OpSVstore, t.reg(dst))
		return nil
	}
	if imm, ok := inst.Args[1].(x86asm.Imm); ok {
		t.sPushImm64(uint64(imm))
		t.emit(vm.OpSVload, t.reg(dst))
		t.emit(vm.OpSMul)
		t.emit(vm.OpSVstore, t.reg(dst))
		return nil
	}
	return fmt.Errorf("unsupported mul/div args")
}

func (t *Translator) trMovExt(inst x86asm.Inst, offset int) error {
	dst, ok := inst.Args[0].(x86asm.Reg)
	if !ok {
		return fmt.Errorf("unsupported MOVSX/ZX dest")
	}
	src := inst.Args[1]
	if sReg, ok := src.(x86asm.Reg); ok {
		t.emit(vm.OpSVload, t.reg(sReg))
	} else if mem, ok := src.(x86asm.Mem); ok {
		t.emitMemAddr(mem, inst, offset)
		t.emit(vm.OpSLd32)
	} else {
		return fmt.Errorf("unsupported MOVSX/ZX src")
	}
	if inst.Op == x86asm.MOVSX || inst.Op == x86asm.MOVSXD {
		t.emit(vm.OpSSext32)
	} else if inst.Op == x86asm.MOVZX {
		t.emit(vm.OpSTrunc32)
	}
	t.emit(vm.OpSVstore, t.reg(dst))
	return nil
}

func (t *Translator) trBT(inst x86asm.Inst) error {
	base, bit := inst.Args[0], inst.Args[1]
	if bReg, ok1 := base.(x86asm.Reg); ok1 {
		if bitImm, ok2 := bit.(x86asm.Imm); ok2 {
			t.emit(vm.OpSVload, t.reg(bReg))
			t.sPushImm32(uint32(bitImm))
			t.emit(vm.OpSAnd) // Simplification: we just need CF but VM flags are slightly different
			return nil
		}
	}
	return fmt.Errorf("unsupported BT")
}

func (t *Translator) trSetcc(inst x86asm.Inst) error {
	dst, ok := inst.Args[0].(x86asm.Reg)
	if !ok {
		return fmt.Errorf("unsupported SETcc dest")
	}

	var op byte
	switch inst.Op {
	case x86asm.SETE:
		op = vm.OpJe
	case x86asm.SETNE:
		op = vm.OpJne
	case x86asm.SETL:
		op = vm.OpJl
	case x86asm.SETGE:
		op = vm.OpJge
	case x86asm.SETLE:
		op = vm.OpJle
	case x86asm.SETG:
		op = vm.OpJgt
	case x86asm.SETB:
		op = vm.OpJb
	case x86asm.SETAE:
		op = vm.OpJae
	case x86asm.SETBE:
		op = vm.OpJbe
	case x86asm.SETA:
		op = vm.OpJa
	}

	// Implementation: if COND then PUSH 1 else PUSH 0
	t.emit(op)
	patch := t.pos()
	t.emitU32(0)
	t.sPushImm32(0)
	t.emit(vm.OpJmp)
	jumpPatch := t.pos()
	t.emitU32(0)
	binary.LittleEndian.PutUint32(t.code[patch:], uint32(t.pos()))
	t.sPushImm32(1)
	binary.LittleEndian.PutUint32(t.code[jumpPatch:], uint32(t.pos()))
	t.emit(vm.OpSVstore, t.reg(dst))
	return nil
}

func (t *Translator) trFpAlu(inst x86asm.Inst) error {
	dst, ok := inst.Args[0].(x86asm.Reg)
	if !ok {
		return fmt.Errorf("unsupported FP ALU dest")
	}
	src := inst.Args[1]

	var op byte
	isDouble := false
	switch inst.Op {
	case x86asm.ADDSS:
		op = vm.OpSFAdd
	case x86asm.ADDSD:
		op = vm.OpSFAdd
		isDouble = true
	case x86asm.SUBSS:
		op = vm.OpSFSub
	case x86asm.SUBSD:
		op = vm.OpSFSub
		isDouble = true
	case x86asm.MULSS:
		op = vm.OpSFMul
	case x86asm.MULSD:
		op = vm.OpSFMul
		isDouble = true
	case x86asm.DIVSS:
		op = vm.OpSFDiv
	case x86asm.DIVSD:
		op = vm.OpSFDiv
		isDouble = true
	}

	fpType := byte(0) // single
	if isDouble {
		fpType = 1
	}

	if sReg, ok := src.(x86asm.Reg); ok {
		t.emit(op, t.reg(dst), t.reg(dst), t.reg(sReg), fpType)
		return nil
	}
	if mem, ok := src.(x86asm.Mem); ok {
		t.emitMemAddr(mem, inst, 0)    // address on stack
		t.emit(vm.OpSVLd, 31, byte(2)) // Load 32-bit to V31 (temp)
		if isDouble {
			t.code[len(t.code)-1] = 3
		} // Change to 64-bit if needed
		t.emit(op, t.reg(dst), t.reg(dst), 31, fpType)
		return nil
	}
	return fmt.Errorf("unsupported FP ALU src")
}

func (t *Translator) trFpMov(inst x86asm.Inst, offset int) error {
	dst, src := inst.Args[0], inst.Args[1]
	isDouble := (inst.Op == x86asm.MOVSD || inst.Op == x86asm.MOVAPD || inst.Op == x86asm.MOVUPD)
	is128 := (inst.Op == x86asm.MOVAPS || inst.Op == x86asm.MOVAPD || inst.Op == x86asm.MOVUPS || inst.Op == x86asm.MOVUPD || inst.Op == x86asm.MOVDQU || inst.Op == x86asm.MOVDQA)
	fpType := byte(0)
	if isDouble {
		fpType = 1
	}

	if dReg, ok1 := dst.(x86asm.Reg); ok1 {
		if sReg, ok2 := src.(x86asm.Reg); ok2 {
			t.emit(vm.OpSFMov, t.reg(dReg), t.reg(sReg), fpType)
			return nil
		}
		if mem, ok2 := src.(x86asm.Mem); ok2 {
			t.emitMemAddr(mem, inst, offset)
			sizeType := byte(2)
			if isDouble {
				sizeType = 3
			}
			if is128 {
				sizeType = 4
			}
			t.emit(vm.OpSVLd, t.reg(dReg), sizeType)
			return nil
		}
	}
	if dMem, ok1 := dst.(x86asm.Mem); ok1 {
		if sReg, ok2 := src.(x86asm.Reg); ok2 {
			t.emitMemAddr(dMem, inst, offset)
			sizeType := byte(2)
			if isDouble {
				sizeType = 3
			}
			if is128 {
				sizeType = 4
			}
			t.emit(vm.OpSVSt, t.reg(sReg), sizeType)
			return nil
		}
	}
	return fmt.Errorf("unsupported FP MOV")
}

func (t *Translator) trFpCmp(inst x86asm.Inst) error {
	n, m := inst.Args[0].(x86asm.Reg), inst.Args[1]
	isDouble := (inst.Op == x86asm.UCOMISD)
	fpType := byte(0)
	if isDouble {
		fpType = 1
	}

	if mReg, ok := m.(x86asm.Reg); ok {
		t.emit(vm.OpSFCmp, t.reg(n), t.reg(mReg), fpType)
		return nil
	}
	return fmt.Errorf("unsupported FP CMP")
}

func (t *Translator) trFpCvt(inst x86asm.Inst) error {
	dst, src := inst.Args[0], inst.Args[1]

	// CVTSI2SS/SD xmm, reg/mem
	if inst.Op == x86asm.CVTSI2SS || inst.Op == x86asm.CVTSI2SD {
		dReg := dst.(x86asm.Reg)
		isDouble := (inst.Op == x86asm.CVTSI2SD)
		fpType := byte(0)
		if isDouble {
			fpType = 1
		}

		if sReg, ok := src.(x86asm.Reg); ok {
			// type: bit0=fp_type, bit1=sf, bit2=unsigned
			t.emit(vm.OpSFCvtIF, t.reg(dReg), t.reg(sReg), fpType|(1<<1))
			return nil
		}
	}

	// CVTTSS2SI/CVTTSD2SI reg, xmm/mem
	if inst.Op == x86asm.CVTTSS2SI || inst.Op == x86asm.CVTTSD2SI {
		dReg := dst.(x86asm.Reg)
		isDouble := (inst.Op == x86asm.CVTTSD2SI)
		fpType := byte(0)
		if isDouble {
			fpType = 1
		}

		if sReg, ok := src.(x86asm.Reg); ok {
			t.emit(vm.OpSFCvtFI, t.reg(dReg), t.reg(sReg), fpType|(1<<1))
			return nil
		}
	}

	return fmt.Errorf("unsupported FP CVT")
}

func (t *Translator) trFpBitwise(inst x86asm.Inst) error {
	dst, src := inst.Args[0].(x86asm.Reg), inst.Args[1]
	if sReg, ok := src.(x86asm.Reg); ok {
		// Map bitwise XMM to VM stack ALU but for V regs
		// A simple way is: S_VLOAD_V(src), S_VLOAD_V(dst), S_OP, S_VSTORE_V(dst)
		var op byte
		switch inst.Op {
		case x86asm.XORPS, x86asm.XORPD:
			op = vm.OpSXor
		case x86asm.ANDPS, x86asm.ANDPD:
			op = vm.OpSAnd
		case x86asm.ORPS, x86asm.ORPD:
			op = vm.OpSOr
		}
		t.emit(vm.OpSVloadV, t.reg(sReg))
		t.emit(vm.OpSVloadV, t.reg(dst))
		t.emit(op)
		t.emit(vm.OpSVstoreV, t.reg(dst))
		return nil
	}
	return fmt.Errorf("unsupported FP bitwise")
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
		t.sPushImm64(uint64(imm))
		t.emit(vm.OpSVload, t.reg(a))
		t.emit(vm.OpSCmp)
		return nil
	}
	return fmt.Errorf("unsupported CMP src")
}

func (t *Translator) trCall(inst x86asm.Inst, offset int) error {
	if rel, ok := inst.Args[0].(x86asm.Rel); ok {
		target := int(int64(offset) + int64(inst.Len) + int64(rel))
		t.emit(vm.OpCallNative)
		t.emitU64(uint64(t.funcAddr + uint64(target)))
		return nil
	}
	if imm, ok := inst.Args[0].(x86asm.Imm); ok {
		target := int(int64(offset) + int64(inst.Len) + int64(imm))
		t.emit(vm.OpCallNative)
		t.emitU64(uint64(t.funcAddr + uint64(target)))
		return nil
	}
	return fmt.Errorf("unsupported CALL")
}

func (t *Translator) trJmp(inst x86asm.Inst, offset int) error {
	var target int
	if rel, ok := inst.Args[0].(x86asm.Rel); ok {
		target = int(int64(offset) + int64(inst.Len) + int64(rel))
	} else if imm, ok := inst.Args[0].(x86asm.Imm); ok {
		target = int(int64(offset) + int64(inst.Len) + int64(imm))
	} else {
		return fmt.Errorf("unsupported JMP")
	}

	t.emit(vm.OpJmp)
	t.fixups = append(t.fixups, branchFixup{vmOffset: t.pos(), x86Target: target})
	t.emitU32(0)
	return nil
}

func (t *Translator) trJcc(inst x86asm.Inst, offset int) error {
	var op byte
	switch inst.Op {
	case x86asm.JE:
		op = vm.OpJe
	case x86asm.JNE:
		op = vm.OpJne
	case x86asm.JL:
		op = vm.OpJl
	case x86asm.JGE:
		op = vm.OpJge
	case x86asm.JLE:
		op = vm.OpJle
	case x86asm.JG:
		op = vm.OpJgt
	case x86asm.JB:
		op = vm.OpJb
	case x86asm.JAE:
		op = vm.OpJae
	case x86asm.JBE:
		op = vm.OpJbe
	case x86asm.JA:
		op = vm.OpJa
	default:
		return fmt.Errorf("unsupported Jcc")
	}

	var target int
	if rel, ok := inst.Args[0].(x86asm.Rel); ok {
		target = int(int64(offset) + int64(inst.Len) + int64(rel))
	} else if imm, ok := inst.Args[0].(x86asm.Imm); ok {
		target = int(int64(offset) + int64(inst.Len) + int64(imm))
	} else {
		return fmt.Errorf("unsupported Jcc arg")
	}

	t.emit(op)
	t.fixups = append(t.fixups, branchFixup{vmOffset: t.pos(), x86Target: target})
	t.emitU32(0)
	return nil
}

func (t *Translator) emitMemAddr(mem x86asm.Mem, inst x86asm.Inst, instOffset int) {
	t.emit(vm.OpSPushImm64)
	disp := mem.Disp
	immOffset := t.pos()
	if mem.Base == x86asm.RIP {
		nextRIP := t.funcAddr + uint64(instOffset) + uint64(inst.Len)
		disp += int64(nextRIP)
		t.emitU64(uint64(disp))
		t.addReloc(immOffset, uint64(disp), false)
	} else {
		t.emitU64(uint64(disp))
		if disp > 0x10000 && mem.Base == 0 && mem.Index == 0 {
			t.addReloc(immOffset, uint64(disp), false)
		}
	}
	if mem.Index != 0 {
		t.emit(vm.OpSVload, t.reg(mem.Index))
		if mem.Scale > 1 {
			t.sPushImm32(uint32(mem.Scale))
			t.emit(vm.OpSMul)
		}
		t.emit(vm.OpSAdd)
	}
	if mem.Base != 0 && mem.Base != x86asm.RIP {
		t.emit(vm.OpSVload, t.reg(mem.Base))
		t.emit(vm.OpSAdd)
	}
}

func (t *Translator) addReloc(bcOffset int, targetAddr uint64, isInternal bool) {
	t.relocations = append(t.relocations, vm.Relocation{BcOffset: bcOffset, TargetAddr: targetAddr, IsInternal: isInternal})
}

func (t *Translator) identifyBasicBlocks(insts []vm.Instruction) map[int]bool {
	starts := make(map[int]bool)
	if len(insts) == 0 {
		return starts
	}
	starts[insts[0].Offset] = true
	for i, inst := range insts {
		xInst, err := x86asm.Decode(inst.RawBytes, 64)
		if err != nil {
			continue
		}
		isBr := false
		var targets []int
		if xInst.Op == x86asm.JMP || xInst.Op == x86asm.CALL {
			isBr = true
			if imm, ok := xInst.Args[0].(x86asm.Imm); ok {
				targets = append(targets, inst.Offset+xInst.Len+int(imm))
			}
		} else if xInst.Op == x86asm.RET {
			isBr = true
		} else if name := xInst.Op.String(); len(name) > 0 && name[0] == 'J' {
			isBr = true
			if imm, ok := xInst.Args[0].(x86asm.Imm); ok {
				targets = append(targets, inst.Offset+xInst.Len+int(imm))
			}
		}
		if isBr {
			for _, target := range targets {
				if target >= 0 && target <= t.funcSize {
					starts[target] = true
				}
			}
			if i+1 < len(insts) {
				starts[insts[i+1].Offset] = true
			}
		}
	}
	return starts
}

func (t *Translator) translateCFF(insts []vm.Instruction) (*vm.TranslateResult, error) {
	if len(insts) == 0 {
		return t.finishTranslate()
	}
	starts := t.identifyBasicBlocks(insts)
	for addr := range starts {
		t.bbStates[addr] = uint32(rand.Int31())
	}
	t.sPushImm32(t.bbStates[insts[0].Offset])
	t.dispPos = t.pos()
	for addr, state := range t.bbStates {
		t.emit(vm.OpSDup)
		t.sPushImm32(state)
		t.emit(vm.OpSCmp)
		t.emit(vm.OpJe)
		t.fixups = append(t.fixups, branchFixup{vmOffset: t.pos(), x86Target: addr})
		t.emitU32(0)
	}
	t.emit(vm.OpHalt)
	for i, inst := range insts {
		if starts[inst.Offset] {
			t.emit(vm.OpSDrop)
		}
		t.labels[inst.Offset] = t.pos()
		xInst, err := x86asm.Decode(inst.RawBytes, 64)
		if err != nil {
			t.unsupported = append(t.unsupported, fmt.Sprintf("offset 0x%X: decode error", inst.Offset))
			t.emit(vm.OpHalt)
			continue
		}
		isBranch := false
		if name := xInst.Op.String(); len(name) > 0 && name[0] == 'J' {
			isBranch = true
		} else if xInst.Op == x86asm.CALL || xInst.Op == x86asm.RET {
			isBranch = true
		}
		if isBranch {
			t.translateBranchCFF(xInst, inst.Offset)
		} else {
			if err := t.translateInst(xInst, inst.Offset); err != nil {
				t.unsupported = append(t.unsupported, fmt.Sprintf("offset 0x%X: %v", inst.Offset, err))
				t.emit(vm.OpHalt)
			}
		}
		nextIdx := i + 1
		if nextIdx < len(insts) {
			nextAddr := insts[nextIdx].Offset
			if starts[nextAddr] && !isBranch {
				t.sPushImm32(t.bbStates[nextAddr])
				t.emit(vm.OpJmp)
				t.emitU32(uint32(t.dispPos))
			}
		}
	}
	return t.finishTranslate()
}

func (t *Translator) translateBranchCFF(inst x86asm.Inst, offset int) {
	if inst.Op == x86asm.RET {
		t.emit(vm.OpRet, t.regMap[X86_RAX])
		return
	}
	if inst.Op == x86asm.CALL {
		t.trCall(inst, offset)
		return
	}
	if inst.Op == x86asm.JMP {
		if imm, ok := inst.Args[0].(x86asm.Imm); ok {
			target := int(int64(offset) + int64(inst.Len) + int64(imm))
			t.sPushImm32(t.bbStates[target])
			t.emit(vm.OpJmp)
			t.emitU32(uint32(t.dispPos))
		} else {
			t.emit(vm.OpHalt)
		}
		return
	}
	imm, ok := inst.Args[0].(x86asm.Imm)
	if !ok {
		t.emit(vm.OpHalt)
		return
	}
	target, fallthroughTarget := int(int64(offset)+int64(inst.Len)+int64(imm)), int(int64(offset)+int64(inst.Len))
	var op byte
	switch inst.Op {
	case x86asm.JE:
		op = vm.OpJe
	case x86asm.JNE:
		op = vm.OpJne
	case x86asm.JL:
		op = vm.OpJl
	case x86asm.JGE:
		op = vm.OpJge
	case x86asm.JLE:
		op = vm.OpJle
	case x86asm.JG:
		op = vm.OpJgt
	case x86asm.JB:
		op = vm.OpJb
	case x86asm.JAE:
		op = vm.OpJae
	case x86asm.JBE:
		op = vm.OpJbe
	case x86asm.JA:
		op = vm.OpJa
	default:
		t.emit(vm.OpHalt)
		return
	}
	t.emit(op)
	targetPatch := t.pos()
	t.emitU32(0)
	t.sPushImm32(t.bbStates[fallthroughTarget])
	t.emit(vm.OpJmp)
	t.emitU32(uint32(t.dispPos))
	binary.LittleEndian.PutUint32(t.code[targetPatch:], uint32(t.pos()))
	t.sPushImm32(t.bbStates[target])
	t.emit(vm.OpJmp)
	t.emitU32(uint32(t.dispPos))
}

func (t *Translator) emitStackMBA(sOp byte, pushX func(), pushY func()) bool {
	if !t.mba {
		return false
	}
	return t.emitRecursiveMBA(sOp, pushX, pushY, 0)
}

func (t *Translator) emitRecursiveMBA(sOp byte, pushX func(), pushY func(), depth int) bool {
	// Limit recursion depth to avoid bytecode explosion
	maxDepth := t.maxMBADepth
	if depth >= maxDepth {
		return false
	}

	// Probability of applying MBA at this level
	chance := 70 // 70% chance to expand
	if depth > 0 {
		chance = 40 // lower chance for nested expansions
	}
	if rand.Intn(100) > chance {
		return false
	}

	// Helper to emit sub-operations recursively
	emitSub := func(op byte, px func(), py func()) {
		if !t.emitRecursiveMBA(op, px, py, depth+1) {
			px()
			py()
			t.emit(op)
		}
	}

	switch sOp {
	case vm.OpSAdd:
		r := rand.Intn(3)
		switch r {
		case 0: // (x ^ y) + 2 * (x & y)
			emitSub(vm.OpSXor, pushX, pushY)
			px2 := func() { pushX(); pushY(); t.emit(vm.OpSAnd) }
			py2 := func() { t.sPushImm32(1) }
			emitSub(vm.OpSShl, px2, py2)
			t.emit(vm.OpSAdd) // Note: top-level add
		case 1: // (x | y) + (x & y)
			px := func() { pushX(); pushY(); t.emit(vm.OpSOr) }
			py := func() { pushX(); pushY(); t.emit(vm.OpSAnd) }
			emitSub(vm.OpSAdd, px, py)
		case 2: // 2 * (x | y) - (x ^ y)
			px := func() {
				px2 := func() { pushX(); pushY(); t.emit(vm.OpSOr) }
				py2 := func() { t.sPushImm32(1) }
				emitSub(vm.OpSShl, px2, py2)
			}
			py := func() { pushX(); pushY(); t.emit(vm.OpSXor) }
			emitSub(vm.OpSSub, px, py)
		}
	case vm.OpSSub:
		r := rand.Intn(2)
		switch r {
		case 0: // (x ^ y) - 2 * (~x & y)
			emitSub(vm.OpSXor, pushX, pushY)
			px2 := func() {
				px3 := func() { pushX(); t.emit(vm.OpSNot) }
				emitSub(vm.OpSAnd, px3, pushY)
			}
			py2 := func() { t.sPushImm32(1) }
			emitSub(vm.OpSShl, px2, py2)
			t.emit(vm.OpSSub)
		case 1: // (x & ~y) - (~x & y)
			px := func() {
				px2 := func() { pushY(); t.emit(vm.OpSNot) }
				emitSub(vm.OpSAnd, pushX, px2)
			}
			py := func() {
				px2 := func() { pushX(); t.emit(vm.OpSNot) }
				emitSub(vm.OpSAnd, px2, pushY)
			}
			emitSub(vm.OpSSub, px, py)
		}
		return true

	case vm.OpSXor:
		r := rand.Intn(2)
		switch r {
		case 0: // (x | y) - (x & y)
			px := func() { pushX(); pushY(); t.emit(vm.OpSOr) }
			py := func() { pushX(); pushY(); t.emit(vm.OpSAnd) }
			emitSub(vm.OpSSub, px, py)
		case 1: // (x & ~y) | (~x & y)
			px := func() {
				px2 := func() { pushY(); t.emit(vm.OpSNot) }
				emitSub(vm.OpSAnd, pushX, px2)
			}
			py := func() {
				px2 := func() { pushX(); t.emit(vm.OpSNot) }
				emitSub(vm.OpSAnd, px2, pushY)
			}
			emitSub(vm.OpSOr, px, py)
		}
		return true

	case vm.OpSAnd:
		r := rand.Intn(2)
		switch r {
		case 0: // (x | y) - (x ^ y)
			px := func() { pushX(); pushY(); t.emit(vm.OpSOr) }
			py := func() { pushX(); pushY(); t.emit(vm.OpSXor) }
			emitSub(vm.OpSSub, px, py)
		case 1: // ~(~x | ~y)
			px := func() {
				px2 := func() { pushX(); t.emit(vm.OpSNot) }
				py2 := func() { pushY(); t.emit(vm.OpSNot) }
				emitSub(vm.OpSOr, px2, py2)
			}
			px()
			t.emit(vm.OpSNot)
		}
		return true

	case vm.OpSOr:
		r := rand.Intn(2)
		switch r {
		case 0: // (x ^ y) + (x & y)
			px := func() { pushX(); pushY(); t.emit(vm.OpSXor) }
			py := func() { pushX(); pushY(); t.emit(vm.OpSAnd) }
			emitSub(vm.OpSAdd, px, py)
		case 1: // (x & ~y) + y
			px := func() {
				px2 := func() { pushY(); t.emit(vm.OpSNot) }
				emitSub(vm.OpSAnd, pushX, px2)
			}
			emitSub(vm.OpSAdd, px, pushY)
		}
		return true
	}

	return false
}
