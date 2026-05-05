package arm64

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math/rand"

	"github.com/vmpacker/pkg/vm"
)

func crc32_calc(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// ============================================================
// ARM64 → VM bytecode translator
//
// Translates decoded ARM64 instructions into custom VM bytecode.
// Returns error for unsupported instructions (never silently skips).
//
// Register mapping:
//   ARM64 X0-X15 → VM R0-R15 (direct mapping)
//   ARM64 X16-X28 → unsupported (trap)
//   ARM64 X29(FP) → not translated within function
//   ARM64 X30(LR) → special handling
//   ARM64 XZR/SP  → context-dependent
//
// Module files:
//   tr_alu.go       — arithmetic/logic/move instructions
//   tr_bitfield.go  — bitfield operations (UBFM/SBFM/EXTR)
//   tr_loadstore.go — load/store (LDR/STR/STP/LDP)
//   tr_branch.go    — branch/conditional select (B/BL/CBZ/CSEL)
//   tr_special.go   — special instructions (ADRP/ADR)
// ============================================================

// TranslateResult - translation result
type TranslateResult struct {
	Bytecode    []byte   // generated VM bytecode (includes trailer)
	CodeLen     int      // pure bytecode length (excludes trailer, used for opcode encryption range)
	Unsupported []string // list of unsupported instructions
	TotalInsts  int      // total instruction count
	TransInsts  int      // translated instruction count
	Relocations []vm.Relocation
}

// DebugEntry - debug mapping for a single instruction
type DebugEntry struct {
	ARM64Offset int    // ARM64 instruction offset within function
	ARM64Asm    string // ARM64 disassembly text
	ARM64Raw    uint32 // ARM64 raw encoding
	VMStart     int    // Translated VM bytecode start position
	VMEnd       int    // Translated VM bytecode end position
}

// StringRef defines an encrypted string to be decrypted at runtime
type StringRef struct {
	Addr uint64
	Len  uint32
	Key  uint32
}

// Translator ARM64 → VM translator
type Translator struct {
	code        []byte          // output buffer
	labels      map[int]int     // ARM64 offset → VM bytecode position mapping
	fixups      []branchFixup   // pending branch targets to patch
	relocations []vm.Relocation // runtime relocations (ASLR)
	funcSize    int             // original function size in bytes
	funcAddr    uint64          // original function start address
	unsupported []string
	decoder     *Decoder       // decoder reference (for name lookup)
	debug       bool           // debug mode
	debugLog    []DebugEntry   // debug mapping log
	cff         bool           // Control Flow Flattening enabled
	bbStates    map[int]uint32 // ARM64 offset -> State ID
	bbLabels    map[int]int    // ARM64 offset -> VM offset (start of block)
	dispPos     int            // VM offset of dispatcher
	mba         bool           // Mixed Boolean-Arithmetic obfuscation
	regMap      [64]byte       // Virtual register shuffling map (arch -> phys)
	stringRefs  map[uint64]StringRef
}

type branchFixup struct {
	vmOffset    int  // VM bytecode position to patch
	arm64Target int  // target ARM64 offset
	isRelToFunc bool // relative to function start
}

// NewTranslator creates translator
func NewTranslator(funcAddr uint64, funcSize int) *Translator {
	t := &Translator{
		code:        make([]byte, 0, funcSize*4),
		labels:      make(map[int]int),
		relocations: make([]vm.Relocation, 0),
		funcAddr:    funcAddr,
		funcSize:    funcSize,
		decoder:     NewDecoder(),
		bbStates:    make(map[int]uint32),
		bbLabels:    make(map[int]int),
	}

	// Initialize register map with random permutation (0..63)
	for i := 0; i < 64; i++ {
		t.regMap[i] = byte(i)
	}
	rand.Shuffle(64, func(i, j int) {
		t.regMap[i], t.regMap[j] = t.regMap[j], t.regMap[i]
	})

	return t
}

// SetCFF enables Control Flow Flattening
func (t *Translator) SetCFF(enabled bool) {
	t.cff = enabled
}

// SetMBA enables Mixed Boolean-Arithmetic obfuscation
func (t *Translator) SetMBA(enabled bool) {
	t.mba = enabled
}

// SetStringRefs sets the map of addresses to encrypted strings
func (t *Translator) SetStringRefs(refs map[uint64]StringRef) {
	t.stringRefs = refs
}

// identifyBasicBlocks scans instructions for branch targets to find block boundaries
func (t *Translator) identifyBasicBlocks(instructions []vm.Instruction) map[int]bool {
	starts := make(map[int]bool)
	if len(instructions) == 0 {
		return starts
	}
	starts[instructions[0].Offset] = true

	for i, inst := range instructions {
		op := Op(inst.Op)
		isBr := false
		var targets []int

		// Determine if instruction ends a basic block or defines a new target
		switch op {
		case B, BL:
			isBr = true
			targets = append(targets, inst.Offset+int(inst.Imm))
		case B_COND, CBZ, CBNZ, TBZ, TBNZ:
			isBr = true
			targets = append(targets, inst.Offset+int(inst.Imm))
			// Conditional branches create a fallthrough start
		case RET, BR, BLR:
			isBr = true
		}

		if isBr {
			for _, target := range targets {
				if target >= 0 && target <= t.funcSize {
					starts[target] = true
				}
			}
			// Following instruction is also a BB start
			if i+1 < len(instructions) {
				starts[instructions[i+1].Offset] = true
			}
		}
	}
	return starts
}

// SetDebug enables debug mode
func (t *Translator) SetDebug(on bool) {
	t.debug = on
}

// DebugLog returns debug mapping log
func (t *Translator) DebugLog() []DebugEntry {
	return t.debugLog
}

// emit appends bytes
func (t *Translator) emit(b ...byte) {
	t.code = append(t.code, b...)
}

// emitU32 appends 32-bit little-endian
func (t *Translator) emitU32(v uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	t.code = append(t.code, b...)
}

// emitU64 appends 64-bit little-endian
func (t *Translator) emitU64(v uint64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	t.code = append(t.code, b...)
}

// addReloc records a runtime relocation
func (t *Translator) addReloc(bcOffset int, targetAddr uint64, isInternal bool) {
	t.relocations = append(t.relocations, vm.Relocation{
		BcOffset:   bcOffset,
		TargetAddr: targetAddr,
		IsInternal: isInternal,
	})
}

// pos returns current bytecode position
func (t *Translator) pos() int {
	return len(t.code)
}

// trunc32 truncates to 32-bit (W register): Rd &= 0xFFFFFFFF
func (t *Translator) trunc32(rd byte) {
	t.emit(vm.OpAndImm, rd, rd)
	t.emitU32(0xFFFFFFFF)
}

// sext32 sign-extends lower 32 bits of Rd to 64-bit: Rd = (i64)(i32)Rd
// Implementation: LSL 32 → ASR 32
func (t *Translator) sext32(rd byte) {
	t.emit(vm.OpShlImm, rd, rd)
	t.emitU32(32)
	t.emit(vm.OpAsrImm, rd, rd)
	t.emitU32(32)
}

func (t *Translator) emitCFFJump(targetAddr int) {
	stateID := t.bbStates[targetAddr]
	t.emit(vm.OpMovImm32, 62) // R62 = State Register
	t.emitU32(stateID)
	t.emit(vm.OpJmp)
	t.emitU32(uint32(t.dispPos))
}

func (t *Translator) emitCFFCondBranch(vmOp byte, targetAddr int, nextAddr int) {
	takenID := t.bbStates[targetAddr]
	notTakenID := t.bbStates[nextAddr]

	// 1. Conditional jump to taken handler
	t.emit(vmOp)
	fixPosTaken := t.pos()
	t.emitU32(0)

	// 2. Not Taken case: update state and jump to dispatcher
	t.emit(vm.OpMovImm32, 62)
	t.emitU32(notTakenID)
	t.emit(vm.OpJmp)
	t.emitU32(uint32(t.dispPos))

	// 3. Taken case (stub): update state and jump to dispatcher
	targetPos := t.pos()
	binary.LittleEndian.PutUint32(t.code[fixPosTaken:], uint32(targetPos))
	t.emit(vm.OpMovImm32, 62)
	t.emitU32(takenID)
	t.emit(vm.OpJmp)
	t.emitU32(uint32(t.dispPos))
}

// insertJunkCode occasionally inserts an unconditional jump over garbage bytes
// This implements control flow obfuscation / opaque predicates in the bytecode.
func (t *Translator) insertJunkCode() {
	// 25% chance to insert junk
	if rand.Intn(4) != 0 {
		return
	}

	t.emit(vm.OpJmp)
	fixPos := t.pos()
	t.emitU32(0) // placeholder for jump target

	// Emit 1 to 12 bytes of garbage (confuses linear disassemblers)
	// Only use bytes where vm.InstructionSize returns 0 so the forward
	// parser in reverseInstructions treats each as a 1-byte instruction,
	// keeping instruction boundaries aligned.
	junkLen := rand.Intn(12) + 1
	for j := 0; j < junkLen; j++ {
		var b byte
		for {
			b = byte(rand.Intn(256))
			if vm.InstructionSize(b) == 0 {
				break
			}
		}
		t.emit(b)
	}

	// Patch the jump to point past the garbage
	targetPos := t.pos()
	binary.LittleEndian.PutUint32(t.code[fixPos:], uint32(targetPos))
}

// mapReg maps ARM64 register → VM register
func (t *Translator) mapReg(arm64Reg int) (byte, error) {
	if arm64Reg == vm.REG_XZR {
		return 63, nil // XZR → R63 (Permanent Zero)
	}
	// SIMD registers V0-V31 are encoded as REG_V_BASE + 0..31
	if arm64Reg >= vm.REG_V_BASE && arm64Reg < vm.REG_V_BASE+32 {
		return byte(arm64Reg - vm.REG_V_BASE), nil
	}
	if arm64Reg < 0 || arm64Reg > 31 {
		return 0, fmt.Errorf("register X%d out of VM range", arm64Reg)
	}
	return t.regMap[arm64Reg], nil
}

// EmitStringDecryption emits VM instructions to decrypt an array of strings at runtime
func (t *Translator) EmitStringDecryption(refs []StringRef) {
	for _, r := range refs {
		// Emit S_PUSH_IMM64 with string address
		t.emit(vm.OpSPushImm64)
		t.emitU64(r.Addr)
		// Record relocation for this immediate (will be adjusted at runtime by RTLR)
		t.addReloc(t.pos()-8, r.Addr, false)

		t.sPushImm32(r.Len)
		t.sPushImm32(r.Key)
		t.emit(vm.OpSDecryptStr)
	}
}

// Translate translates entire function
func (t *Translator) Translate(instructions []vm.Instruction) (*TranslateResult, error) {
	if t.cff {
		return t.translateCFF(instructions)
	}

	result := &TranslateResult{TotalInsts: len(instructions)}
	skip := 0
	for i := 0; i < len(instructions); i++ {
		if skip > 0 {
			t.labels[instructions[i].Offset] = t.pos()
			skip--
			result.TransInsts++
			continue
		}

		t.insertJunkCode()
		t.labels[instructions[i].Offset] = t.pos()

		vmStartPos := t.pos()
		var err error
		skip, err = t.translateOne(instructions, i)

		// debug: record mapping
		if t.debug {
			inst := instructions[i]
			entry := DebugEntry{
				ARM64Offset: inst.Offset,
				ARM64Asm:    OpName(Op(inst.Op)),
				ARM64Raw:    inst.Raw,
				VMStart:     vmStartPos,
				VMEnd:       t.pos(),
			}
			t.debugLog = append(t.debugLog, entry)
			// record subsequent skipped instructions
			for s := 1; s <= skip && i+s < len(instructions); s++ {
				skipped := instructions[i+s]
				t.debugLog = append(t.debugLog, DebugEntry{
					ARM64Offset: skipped.Offset,
					ARM64Asm:    OpName(Op(skipped.Op)) + " (merged)",
					ARM64Raw:    skipped.Raw,
					VMStart:     vmStartPos,
					VMEnd:       t.pos(),
				})
			}
		}

		if err != nil {
			t.unsupported = append(t.unsupported, fmt.Sprintf(
				"offset 0x%04X: %s (raw=0x%08X) - %v",
				instructions[i].Offset, OpName(Op(instructions[i].Op)), instructions[i].Raw, err))
			t.emit(vm.OpHalt)
		} else {
			result.TransInsts++
		}
	}

	return t.finishTranslate(result)
}

func (t *Translator) translateCFF(instructions []vm.Instruction) (*TranslateResult, error) {
	result := &TranslateResult{TotalInsts: len(instructions)}

	if len(instructions) == 0 {
		return t.finishTranslate(result)
	}

	// 1. Identify BB starts
	starts := t.identifyBasicBlocks(instructions)

	// 2. Assign random state IDs
	for addr := range starts {
		t.bbStates[addr] = uint32(rand.Int31())
	}

	// 3. Emit Prologue: Set initial state
	firstAddr := instructions[0].Offset
	t.emit(vm.OpMovImm32, 62) // R62 = State Register
	t.emitU32(t.bbStates[firstAddr])
	// Jump to dispatcher
	t.emit(vm.OpJmp)
	fixDisp := t.pos()
	t.emitU32(0)

	// 4. Emit Dispatcher
	t.dispPos = t.pos()
	binary.LittleEndian.PutUint32(t.code[fixDisp:], uint32(t.dispPos))

	for addr, stateID := range t.bbStates {
		t.emit(vm.OpCmpImm, 62)
		t.emitU32(stateID)
		t.emit(vm.OpJe)
		fixPos := t.pos()
		t.emitU32(0)
		t.fixups = append(t.fixups, branchFixup{vmOffset: fixPos, arm64Target: addr})
	}
	t.emit(vm.OpHalt) // Should not be reached

	// 5. Translate each block
	skip := 0
	for i := 0; i < len(instructions); i++ {
		addr := instructions[i].Offset
		t.labels[addr] = t.pos()

		if skip > 0 {
			skip--
			result.TransInsts++
			continue
		}

		// If this is a BB start, we might want to insert junk here too
		if starts[addr] {
			t.insertJunkCode()
			t.labels[addr] = t.pos() // Update label after junk
		}

		var err error
		skip, err = t.translateOne(instructions, i)

		if err != nil {
			t.unsupported = append(t.unsupported, fmt.Sprintf(
				"offset 0x%04X: %s (raw=0x%08X) - %v",
				addr, OpName(Op(instructions[i].Op)), instructions[i].Raw, err))
			t.emit(vm.OpHalt)
		} else {
			result.TransInsts++
		}

		// 6. End of block fallthrough
		nextIdx := i + skip + 1
		if nextIdx < len(instructions) {
			nextAddr := instructions[nextIdx].Offset
			if starts[nextAddr] {
				// This block ended without a branch, but next is a new BB start.
				// We MUST update state and jump to dispatcher.
				t.emit(vm.OpMovImm32, 62)
				t.emitU32(t.bbStates[nextAddr])
				t.emit(vm.OpJmp)
				t.emitU32(uint32(t.dispPos))
			}
		}
	}

	return t.finishTranslate(result)
}

func (t *Translator) finishTranslate(result *TranslateResult) (*TranslateResult, error) {
	t.labels[t.funcSize] = t.pos()
	t.emit(vm.OpHalt)

	for _, fix := range t.fixups {
		target, ok := t.labels[fix.arm64Target]
		if !ok {
			return nil, fmt.Errorf("branch target ARM64 offset 0x%X not found in VM positions", fix.arm64Target)
		}
		binary.LittleEndian.PutUint32(t.code[fix.vmOffset:], uint32(target))
	}

	// record pure bytecode length (before trailer)
	result.CodeLen = t.pos()

	// ---- append CRC section ----
	bcCrc := crc32_calc(t.code[:result.CodeLen])
	t.emitU64(0)          // stub_va placeholder
	t.emitU32(0)          // stub_size placeholder
	t.emitU32(0)          // stub_crc placeholder
	t.emitU32(bcCrc)      // bc_crc
	t.emitU32(0x43524332) // CRC_MAGIC

	// ---- append trailer ----
	t.emit(t.regMap[:]...)
	t.emit(vm.GlobalOpMap[:]...)
	mapCount := uint32(len(t.labels))
	for arm64Off, vmOff := range t.labels {
		t.emitU32(uint32(arm64Off))
		t.emitU32(uint32(vmOff))
	}
	t.emit(0)    // reverse placeholder
	t.emitU32(0) // oc_key placeholder
	t.emitU32(mapCount)
	t.emitU64(t.funcAddr)
	t.emitU32(uint32(t.funcSize))

	result.Bytecode = t.code
	result.Unsupported = t.unsupported
	result.Relocations = t.relocations
	return result, nil
}

// translateOne translates single instruction, returns number of subsequent instructions to skip
func (t *Translator) translateOne(instructions []vm.Instruction, idx int) (int, error) {
	inst := instructions[idx]
	op := Op(inst.Op)

	switch op {
	case NOP:
		t.emit(vm.OpNop)
		return 0, nil

	// ========== Data processing (immediate) -- stack mode ==========

	case ADD_IMM:
		return 0, t.trStackAluImm(inst, vm.OpSAdd)
	case SUB_IMM:
		return 0, t.trStackAluImm(inst, vm.OpSSub)
	case ADDS_IMM, SUBS_IMM:
		if inst.Rd == vm.REG_XZR {
			// CMN/CMP Xn, #imm -- stack mode
			rn, err := t.mapReg(inst.Rn)
			if err != nil {
				return 0, err
			}
			if op == ADDS_IMM {
				// CMN: flags from Xn + imm
				px := func() { t.pushRegOrZero(inst.Rn, rn) }
				py := func() { t.sPushImm(uint64(inst.Imm)) }
				if !t.emitStackMBA(vm.OpSAdd, px, py) {
					px()
					py()
					t.emit(vm.OpSAdd)
				}
				t.sPushImm32(0)
				t.emit(vm.OpSCmp)
				t.sDrop() // discard sum
			} else {
				// CMP: flags from Xn - imm
				t.pushRegOrZero(inst.Rn, rn)
				t.sPushImm(uint64(inst.Imm))
				t.emit(vm.OpSCmp)
			}
			return 0, nil
		}
		if op == ADDS_IMM {
			return 0, t.trStackAluImmFlags(inst, vm.OpSAdd, true)
		}
		return 0, t.trStackAluImmFlags(inst, vm.OpSSub, true)

	case AND_IMM:
		return 0, t.trStackAluImm(inst, vm.OpSAnd)
	case ANDS_IMM:
		if inst.Rd == vm.REG_XZR {
			// TST Xn, #imm -- stack mode
			rn, err := t.mapReg(inst.Rn)
			if err != nil {
				return 0, err
			}
			t.pushRegOrZero(inst.Rn, rn)
			t.sPushImm(uint64(inst.Imm))
			t.emit(vm.OpSAnd)
			t.sPushImm32(0)
			t.emit(vm.OpSCmp)
			t.sDrop() // discard AND result
			return 0, nil
		}
		return 0, t.trStackAluImmFlags(inst, vm.OpSAnd, true)
	case ORR_IMM:
		return 0, t.trStackAluImm(inst, vm.OpSOr)
	case EOR_IMM:
		return 0, t.trStackAluImm(inst, vm.OpSXor)

	case MOVZ:
		return 0, t.trStackMov(inst)
	case MOVK:
		return 0, t.trStackMovK(inst)
	case MOVN:
		return 0, t.trStackMovN(inst)

	// ========== Data processing (register) ==========

	case ADD_REG:
		return 0, t.trStackAluReg(inst, vm.OpSAdd)
	case SUB_REG:
		return 0, t.trStackAluReg(inst, vm.OpSSub)
	case AND_REG:
		return 0, t.trStackAluReg(inst, vm.OpSAnd)
	case ORR_REG:
		if inst.Rn == vm.REG_XZR {
			// MOV alias: ORR Xd, XZR, Xm → stack mode
			return 0, t.trStackMovReg(vm.Instruction{Op: inst.Op, Rd: inst.Rd, Rn: inst.Rm, SF: inst.SF})
		}
		return 0, t.trStackAluReg(inst, vm.OpSOr)
	case EOR_REG:
		return 0, t.trStackAluReg(inst, vm.OpSXor)
	case EON:
		return 0, t.trStackEON(inst) // stack mode
	case MVN:
		// MVN Xd, Xm[, shift] — stack mode
		rd, err := t.mapReg(inst.Rd)
		if err != nil {
			return 0, err
		}
		rm, err := t.mapReg(inst.Rm)
		if err != nil {
			return 0, err
		}
		t.sVload(rm)
		if inst.Shift != 0 {
			t.emitShiftOnStack(0, uint32(inst.Shift), inst.SF) // LSL
		}
		t.emit(vm.OpSNot)
		if !inst.SF {
			t.emit(vm.OpSTrunc32)
		}
		t.sVstore(rd)
		return 0, nil
	case MUL:
		return 0, t.trStackAluReg(inst, vm.OpSMul)
	case LSL_REG:
		return 0, t.trStackAluReg(inst, vm.OpSShl)
	case LSR_REG:
		return 0, t.trStackAluReg(inst, vm.OpSShr)
	case ASR_REG:
		return 0, t.trStackAluReg(inst, vm.OpSAsr)
	case ROR_REG:
		return 0, t.trStackAluReg(inst, vm.OpSRor)

	case ADDS_REG, SUBS_REG:
		if inst.Rd == vm.REG_XZR {
			// CMN/CMP Xn, Xm -- stack mode
			rn, err := t.mapReg(inst.Rn)
			if err != nil {
				return 0, err
			}
			rm, err := t.mapReg(inst.Rm)
			if err != nil {
				return 0, err
			}
			if op == ADDS_REG {
				// CMN: VLOAD(rn) VLOAD(rm) S_ADD PUSH(0) S_CMP DROP
				px := func() { t.pushRegOrZero(inst.Rn, rn) }
				py := func() { t.pushRegOrZero(inst.Rm, rm) }
				if !t.emitStackMBA(vm.OpSAdd, px, py) {
					px()
					py()
					t.emit(vm.OpSAdd)
				}
				t.sPushImm32(0)
				t.emit(vm.OpSCmp)
				t.sDrop()
			} else {
				// CMP: VLOAD(rn) VLOAD(rm) S_CMP
				t.pushRegOrZero(inst.Rn, rn)
				t.pushRegOrZero(inst.Rm, rm)
				t.emit(vm.OpSCmp)
			}
			return 0, nil
		}
		if op == ADDS_REG {
			return 0, t.trStackAluRegFlags(inst, vm.OpSAdd, true)
		}
		return 0, t.trStackAluRegFlags(inst, vm.OpSSub, true)

	case ANDS_REG:
		if inst.Rd == vm.REG_XZR {
			// TST Xn, Xm -- stack mode
			rn, err := t.mapReg(inst.Rn)
			if err != nil {
				return 0, err
			}
			rm, err := t.mapReg(inst.Rm)
			if err != nil {
				return 0, err
			}
			px := func() { t.pushRegOrZero(inst.Rn, rn) }
			py := func() { t.pushRegOrZero(inst.Rm, rm) }
			if !t.emitStackMBA(vm.OpSAnd, px, py) {
				px()
				py()
				t.emit(vm.OpSAnd)
			}
			t.sPushImm32(0)
			t.emit(vm.OpSCmp)
			t.sDrop()
			return 0, nil
		}
		return 0, t.trStackAluRegFlags(inst, vm.OpSAnd, true)

	case BIC:
		return 0, t.trStackBitLogicalNot(inst, vm.OpSAnd, false)
	case BICS:
		if inst.Rd == vm.REG_XZR {
			return 0, t.trStackBitLogicalNot(inst, vm.OpSAnd, true)
		}
		return 0, t.trStackBitLogicalNot(inst, vm.OpSAnd, true)
	case ORN:
		return 0, t.trStackBitLogicalNot(inst, vm.OpSOr, false)

	// ========== FP / SIMD ==========
	case FADD, FSUB, FMUL, FDIV, FMOV, FCMP, FNEG, FABS, FSQRT,
		FMAX, FMIN, FCVTZS, FCVTZU, SCVTF, UCVTF, FCVT:
		return t.translateFP(inst)

	// ========== Bitfield operations ==========

	case UBFM:
		return 0, t.trStackUBFM(inst)
	case SBFM:
		return 0, t.trSBFM(inst)
	case BFM:
		return 0, t.trStackBFM(inst)

	// ========== Load/store ==========

	case LDR_IMM, LDRB_IMM, LDRH_IMM, LDRSB_IMM, LDRSH_IMM, LDRSW_IMM:
		return 0, t.trStackLoad(inst)
	case LDR_LIT:
		return 0, t.trStackLdrLiteral(inst)
	case STR_IMM, STRB_IMM, STRH_IMM:
		return 0, t.trStackStore(inst)

	case STP:
		return 0, t.trStackSTP(inst)
	case LDP:
		return 0, t.trStackLDP(inst)

	// ========== Branch ==========

	case B:
		return 0, t.trBranch(inst)
	case B_COND:
		return 0, t.trBranchCond(inst)
	case CBZ:
		return 0, t.trStackCBZ(inst, true)
	case CBNZ:
		return 0, t.trStackCBZ(inst, false)
	case BL:
		return 0, t.trBL(inst)
	case BLR:
		return 0, t.trBLR(inst)
	case BR:
		return 0, t.trBR(inst)
	case RET:
		t.emit(vm.OpRet, 0)
		return 0, nil

	// ========== Conditional select ==========
	case CSEL:
		return 0, t.trStackCSEL(inst)
	case CSINC:
		return 0, t.trStackCSEL(inst)
	case CSINV:
		return 0, t.trStackCSEL(inst)
	case CSNEG:
		return 0, t.trStackCSEL(inst)
	case MADD:
		return 0, t.trStackMADD(inst, false)
	case MSUB:
		return 0, t.trStackMADD(inst, true)
	case SMADDL:
		return 0, t.trStackSMADDL(inst, false)
	case SMSUBL:
		return 0, t.trStackSMADDL(inst, true)
	case UMADDL:
		return 0, t.trStackUMADDL(inst, false)
	case UMSUBL:
		return 0, t.trStackUMADDL(inst, true)
	case UMULH:
		return 0, t.trStackUnary(inst, vm.OpSUmulh) // UMULH is binary but doesn't set flags

	// ========== Extended register add/subtract (T4) -- stack mode ==========
	case ADD_EXT:
		return 0, t.trStackAddSubExt(inst, vm.OpSAdd, false)
	case SUB_EXT:
		return 0, t.trStackAddSubExt(inst, vm.OpSSub, false)
	case ADDS_EXT:
		return 0, t.trStackAddSubExt(inst, vm.OpSAdd, true)
	case SUBS_EXT:
		return 0, t.trStackAddSubExt(inst, vm.OpSSub, true)

	// ========== TBZ/TBNZ (T5) ==========
	case TBZ:
		return 0, t.trTBZ(inst, true)
	case TBNZ:
		return 0, t.trTBZ(inst, false)

	// ========== CCMP/CCMN (T6/T7) ==========
	case CCMP_REG:
		return 0, t.trCCMP(inst, false, false)
	case CCMP_IMM:
		return 0, t.trCCMP(inst, false, true)
	case CCMN_REG:
		return 0, t.trCCMP(inst, true, false)
	case CCMN_IMM:
		return 0, t.trCCMP(inst, true, true)

	// ========== SVC (T8) ==========
	case SVC:
		return 0, t.trSVC(inst)

	// ========== UDIV/SDIV ==========
	case UDIV:
		return 0, t.trStackAluReg(inst, vm.OpSUdiv)
	case SDIV:
		return 0, t.trStackAluReg(inst, vm.OpSSdiv)

	// ========== MRS ==========
	case MRS:
		return 0, t.trMRS(inst) // system register keeps original routing

	// ========== SMULH/CLZ/CLS/RBIT/REV — stack mode ==========
	case SMULH:
		return 0, t.trStackAluReg(inst, vm.OpSSmulh)
	case CLZ:
		return 0, t.trStackUnary(inst, vm.OpSClz)
	case CLS:
		return 0, t.trStackUnary(inst, vm.OpSCls)
	case RBIT:
		return 0, t.trStackUnary(inst, vm.OpSRbit)
	case REV:
		return 0, t.trStackUnary(inst, vm.OpSRev)
	case REV16:
		return 0, t.trStackUnary(inst, vm.OpSRev16)
	case REV32:
		return 0, t.trStackUnary(inst, vm.OpSRev32)

	// ========== ADC/ADCS/SBC/SBCS — stack mode ==========
	case ADC:
		return 0, t.trStackAluReg(inst, vm.OpSAdc)
	case ADCS:
		return 0, t.trStackAluRegFlags(inst, vm.OpSAdc, true)
	case SBC:
		return 0, t.trStackAluReg(inst, vm.OpSSbc)
	case SBCS:
		return 0, t.trStackAluRegFlags(inst, vm.OpSSbc, true)

	// ========== Register offset load/store — stack mode ==========
	case LDR_REG, LDRB_REG, LDRH_REG:
		return 0, t.trStackLoadReg(inst)
	case LDRSB_REG, LDRSH_REG, LDRSW_REG:
		return 0, t.trStackLoadRegSigned(inst)
	case STR_REG, STRB_REG, STRH_REG:
		return 0, t.trStackStoreReg(inst)

	// ========== ADRP ==========
	case ADRP:
		return t.trADRP(instructions, idx)
	case ADR:
		return t.trADR(inst)

	// ========== SIMD LD1/ST1 ==========
	case LD1_16B:
		rn, err := t.mapReg(inst.Rn)
		if err != nil {
			return 0, err
		}
		t.emit(vm.OpVld16, rn)
		t.code = append(t.code, byte(inst.Imm))
		return 0, nil
	case ST1_16B:
		rn, err := t.mapReg(inst.Rn)
		if err != nil {
			return 0, err
		}
		t.emit(vm.OpVst16, rn)
		t.code = append(t.code, byte(inst.Imm))
		return 0, nil

	// ========== Bitfield extract ==========
	case EXTR:
		return 0, t.trStackEXTR(inst)

	// ========== NOP-ified instructions (Batch 4/6/7) ==========
	case DMB, DSB, ISB, WFE, WFI, YIELD_ARM, CLREX, MSR_WRITE, PRFM:
		t.emit(vm.OpNop)
		return 0, nil
	case HLT, BRK:
		t.emit(vm.OpHalt)
		return 0, nil

	// ========== Acquire/Release (Batch 5) ==========
	case LDAR, LDAXR:
		return 0, t.trLdar(inst)
	case STLR:
		return 0, t.trStlr(inst)
	case STLXR:
		return 0, t.trStlxr(inst)

	// ========== LDPSW (Batch 8) ==========
	case LDPSW:
		return 0, t.trStackLdpsw(inst)

	// ========== Atomic LSE (Batch 8) ==========
	case LDADD:
		return 0, t.trStackLdadd(inst)
	case CAS:
		return 0, t.trStackCas(inst)

	default:
		return 0, fmt.Errorf("unsupported instruction type")
	}
}
