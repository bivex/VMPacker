package arm32

import (
	"encoding/binary"
	"fmt"

	"github.com/vmpacker/pkg/vm"
)

// TranslateResult holds the translation output
type TranslateResult struct {
	Bytecode    []byte
	CodeLen     int
	Unsupported []string
	TotalInsts  int
	TransInsts  int
}

// DebugEntry records a single instruction's debug mapping
type DebugEntry struct {
	ARM32Offset int
	ARM32Asm    string
	ARM32Raw    uint32
	VMStart     int
	VMEnd       int
}

// Translator ARM32 → VM bytecode translator
type Translator struct {
	code             []byte
	labels           map[int]int
	fixups           []branchFixup
	funcSize         int
	funcAddr         uint64
	unsupported      []string
	decoder          *Decoder
	debug            bool
	debugLog         []DebugEntry
	thumbMode        bool
	literalPoolStart int // index in instruction slice where literal pool begins (-1 = none)
	rawCode          []byte // raw function machine code (for PC-relative literal pool resolution)
}

type branchFixup struct {
	vmOffset    int
	arm32Target int
}

// NewTranslator creates a new ARM32 translator.
// rawCode is the raw function machine code for resolving PC-relative literal pool loads.
func NewTranslator(funcAddr uint64, funcSize int, rawCode ...[]byte) *Translator {
	t := &Translator{
		code:     make([]byte, 0, funcSize*4),
		labels:   make(map[int]int),
		funcAddr: funcAddr,
		funcSize: funcSize,
		decoder:  NewDecoder(),
	}
	if len(rawCode) > 0 {
		t.rawCode = rawCode[0]
	}
	return t
}

// NewThumbTranslator creates a Thumb mode translator.
// rawCode is the raw function machine code for resolving PC-relative literal pool loads.
func NewThumbTranslator(funcAddr uint64, funcSize int, rawCode ...[]byte) *Translator {
	t := &Translator{
		code:      make([]byte, 0, funcSize*4),
		labels:    make(map[int]int),
		funcAddr:  funcAddr,
		funcSize:  funcSize,
		decoder:   NewThumbDecoder(),
		thumbMode: true,
	}
	if len(rawCode) > 0 {
		t.rawCode = rawCode[0]
	}
	return t
}

// SetDebug enables debug mode
func (t *Translator) SetDebug(on bool) { t.debug = on }

// DebugLog returns the debug log
func (t *Translator) DebugLog() []DebugEntry { return t.debugLog }

func (t *Translator) emit(b ...byte) { t.code = append(t.code, b...) }

func (t *Translator) emitU32(v uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	t.code = append(t.code, b...)
}

func (t *Translator) emitU64(v uint64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	t.code = append(t.code, b...)
}

func (t *Translator) pos() int { return len(t.code) }

// mapReg maps ARM32 register to VM register.
// R0-R12 → VM R0-R12, R13(SP) → VM R13, R14(LR) → VM R14, R15(PC) → error
func (t *Translator) mapReg(arm32Reg int) (byte, error) {
	if arm32Reg < 0 || arm32Reg > 15 {
		return 0, fmt.Errorf("register R%d out of range", arm32Reg)
	}
	return byte(arm32Reg), nil
}

// mapReg3 maps three registers
func (t *Translator) mapReg3(inst vm.Instruction) (byte, byte, byte, error) {
	rd, err := t.mapReg(inst.Rd)
	if err != nil {
		return 0, 0, 0, err
	}
	rn, err := t.mapReg(inst.Rn)
	if err != nil {
		return 0, 0, 0, err
	}
	rm, err := t.mapReg(inst.Rm)
	if err != nil {
		return 0, 0, 0, err
	}
	return rd, rn, rm, nil
}

// --- Stack machine helpers ---

func (t *Translator) sVload(reg byte)  { t.emit(vm.OpSVload, reg) }
func (t *Translator) sVstore(reg byte) { t.emit(vm.OpSVstore, reg) }

func (t *Translator) sPushImm32(v uint32) {
	t.emit(vm.OpSPushImm32)
	t.emitU32(v)
}

func (t *Translator) sPushImm64(v uint64) {
	t.emit(vm.OpSPushImm64)
	t.emitU64(v)
}

func (t *Translator) sPushImm(v uint64) {
	if v <= 0xFFFFFFFF {
		t.sPushImm32(uint32(v))
	} else {
		t.sPushImm64(v)
	}
}

func (t *Translator) sDup()  { t.emit(vm.OpSDup) }
func (t *Translator) sSwap() { t.emit(vm.OpSSwap) }
func (t *Translator) sDrop() { t.emit(vm.OpSDrop) }

// emitTrunc32 truncates TOS to 32-bit (all ARM32 results are 32-bit)
func (t *Translator) emitTrunc32() {
	t.emit(vm.OpSTrunc32)
}

// emitCondWrapper wraps the body of a conditional instruction.
// If cond != AL, it emits: condJump(skip) [body] skip:
// Returns the position after the skip placeholder (for the caller to emit body after).
func (t *Translator) emitCondCheck(cond int) (skipFixPos int, needsFix bool) {
	if cond == COND_AL || cond < 0 {
		return 0, false
	}

	// Emit the INVERSE condition jump to skip the body
	var vmOp byte
	switch cond {
	case COND_EQ:
		vmOp = vm.OpJne // skip if NE
	case COND_NE:
		vmOp = vm.OpJe
	case COND_CS:
		vmOp = vm.OpJb // skip if CC
	case COND_CC:
		vmOp = vm.OpJae
	case COND_MI:
		vmOp = vm.OpJge // skip if PL
	case COND_PL:
		vmOp = vm.OpJl
	case COND_VS:
		vmOp = vm.OpJvc // skip if VC
	case COND_VC:
		vmOp = vm.OpJvs // skip if VS
	case COND_HI:
		vmOp = vm.OpJbe // skip if LS
	case COND_LS:
		vmOp = vm.OpJa
	case COND_GE:
		vmOp = vm.OpJl // skip if LT
	case COND_LT:
		vmOp = vm.OpJge
	case COND_GT:
		vmOp = vm.OpJle // skip if LE
	case COND_LE:
		vmOp = vm.OpJgt
	default:
		return 0, false
	}

	t.emit(vmOp)
	fixPos := t.pos()
	t.emitU32(0)
	return fixPos, true
}

// patchCondSkip patches the skip target after the body is emitted
func (t *Translator) patchCondSkip(fixPos int) {
	binary.LittleEndian.PutUint32(t.code[fixPos:], uint32(t.pos()))
}

// Translate translates an entire function's instructions
func (t *Translator) Translate(instructions []vm.Instruction) (*TranslateResult, error) {
	result := &TranslateResult{TotalInsts: len(instructions)}

	// Pre-scan: mark trailing literal pool data.
	// Strategy: find the last return instruction (BX LR / POP {PC} / LDR PC),
	// then everything after it is literal pool data.  Fall back to the old
	// consecutive-UNKNOWN scan when no return is found.
	t.literalPoolStart = -1
	lastRetIdx := -1
	for i, inst := range instructions {
		op := Op(inst.Op)
		switch op {
		case BX:
			if inst.Rm == 14 { // BX LR
				lastRetIdx = i
			}
		case LDR_IMM:
			if inst.Rd == 15 { // LDR PC, ... (POP {PC})
				lastRetIdx = i
			}
		case LDM:
			if uint16(inst.Imm)&(1<<15) != 0 { // reglist includes PC
				lastRetIdx = i
			}
		}
	}
	if lastRetIdx >= 0 && lastRetIdx < len(instructions)-1 {
		t.literalPoolStart = lastRetIdx + 1
	} else {
		// Fallback: consecutive UNSUPPORTED/UNKNOWN from the end
		for i := len(instructions) - 1; i >= 0; i-- {
			op := Op(instructions[i].Op)
			if op == UNSUPPORTED || op == UNKNOWN {
				t.literalPoolStart = i
			} else {
				break
			}
		}
	}

	for i := 0; i < len(instructions); i++ {
		inst := instructions[i]
		t.labels[inst.Offset] = t.pos()

		vmStartPos := t.pos()
		err := t.translateOne(instructions, i)

		if t.debug {
			entry := DebugEntry{
				ARM32Offset: inst.Offset,
				ARM32Asm:    OpName(Op(inst.Op)),
				ARM32Raw:    inst.Raw,
				VMStart:     vmStartPos,
				VMEnd:       t.pos(),
			}
			t.debugLog = append(t.debugLog, entry)
		}

		if err != nil {
			t.unsupported = append(t.unsupported, fmt.Sprintf(
				"offset 0x%04X: %s (raw=0x%08X) - %v",
				inst.Offset, OpName(Op(inst.Op)), inst.Raw, err))
			t.emit(vm.OpHalt)
		} else {
			result.TransInsts++
		}
	}

	t.labels[t.funcSize] = t.pos()
	t.emit(vm.OpHalt)

	// Patch branch targets
	for _, fix := range t.fixups {
		target, ok := t.labels[fix.arm32Target]
		if !ok {
			return nil, fmt.Errorf("branch target 0x%X not found in VM labels", fix.arm32Target)
		}
		binary.LittleEndian.PutUint32(t.code[fix.vmOffset:], uint32(target))
	}

	result.CodeLen = t.pos()

	// Trailer: addr map + reverse + oc_key + map_count + func_addr + func_size
	mapCount := uint32(len(t.labels))
	for arm32Off, vmOff := range t.labels {
		t.emitU32(uint32(arm32Off))
		t.emitU32(uint32(vmOff))
	}
	t.emit(0)    // reverse placeholder
	t.emitU32(0) // oc_key placeholder
	t.emitU32(mapCount)
	t.emitU64(t.funcAddr)
	t.emitU32(uint32(t.funcSize))

	result.Bytecode = t.code
	result.Unsupported = t.unsupported
	return result, nil
}

// translateOne translates a single instruction
func (t *Translator) translateOne(instructions []vm.Instruction, idx int) error {
	inst := instructions[idx]
	op := Op(inst.Op)

	switch op {
	case NOP:
		t.emit(vm.OpNop)
		return nil
	case IT:
		// IT blocks are handled by the decoder (condition propagation).
		// The IT instruction itself is a NOP in the VM.
		t.emit(vm.OpNop)
		return nil

	// ========== Data processing (immediate) ==========
	case ADD_IMM:
		return t.trCondAluImm(inst, vm.OpSAdd)
	case ADDS_IMM:
		return t.trCondAluImmFlags(inst, vm.OpSAdd)
	case SUB_IMM:
		return t.trCondAluImm(inst, vm.OpSSub)
	case SUBS_IMM:
		return t.trCondAluImmFlags(inst, vm.OpSSub)
	case RSB_IMM:
		return t.trCondRSBImm(inst, false)
	case RSBS_IMM:
		return t.trCondRSBImm(inst, true)
	case AND_IMM:
		return t.trCondAluImm(inst, vm.OpSAnd)
	case ANDS_IMM:
		return t.trCondAluImmFlags(inst, vm.OpSAnd)
	case ORR_IMM:
		return t.trCondAluImm(inst, vm.OpSOr)
	case ORRS_IMM:
		return t.trCondAluImmFlags(inst, vm.OpSOr)
	case EOR_IMM:
		return t.trCondAluImm(inst, vm.OpSXor)
	case EORS_IMM:
		return t.trCondAluImmFlags(inst, vm.OpSXor)
	case BIC_IMM:
		return t.trCondBICImm(inst, false)
	case BICS_IMM:
		return t.trCondBICImm(inst, true)
	case ADC_IMM:
		return t.trCondAluImm(inst, vm.OpSAdc)
	case ADCS_IMM:
		return t.trCondAluImmFlags(inst, vm.OpSAdc)
	case SBC_IMM:
		return t.trCondAluImm(inst, vm.OpSSbc)
	case SBCS_IMM:
		return t.trCondAluImmFlags(inst, vm.OpSSbc)
	case RSC_IMM:
		return t.trCondRSBImm(inst, false) // RSC = reverse sub with carry
	case RSCS_IMM:
		return t.trCondRSBImm(inst, true)

	// ========== MOV/MVN (immediate) ==========
	case MOV_IMM, MOVS_IMM:
		return t.trCondMovImm(inst, op == MOVS_IMM)
	case MVN_IMM, MVNS_IMM:
		return t.trCondMvnImm(inst, op == MVNS_IMM)
	case MOVW:
		return t.trCondMovW(inst)
	case MOVT:
		return t.trCondMovT(inst)

	// ========== Compare/Test (immediate) ==========
	case CMP_IMM:
		return t.trCondCmpImm(inst)
	case CMN_IMM:
		return t.trCondCmnImm(inst)
	case TST_IMM:
		return t.trCondTstImm(inst)
	case TEQ_IMM:
		return t.trCondTeqImm(inst)

	// ========== Data processing (register) ==========
	case ADD_REG:
		return t.trCondAluReg(inst, vm.OpSAdd)
	case ADDS_REG:
		return t.trCondAluRegFlags(inst, vm.OpSAdd)
	case SUB_REG:
		return t.trCondAluReg(inst, vm.OpSSub)
	case SUBS_REG:
		return t.trCondAluRegFlags(inst, vm.OpSSub)
	case RSB_REG:
		return t.trCondRSBReg(inst, false)
	case RSBS_REG:
		return t.trCondRSBReg(inst, true)
	case AND_REG:
		return t.trCondAluReg(inst, vm.OpSAnd)
	case ANDS_REG:
		return t.trCondAluRegFlags(inst, vm.OpSAnd)
	case ORR_REG:
		return t.trCondAluReg(inst, vm.OpSOr)
	case ORRS_REG:
		return t.trCondAluRegFlags(inst, vm.OpSOr)
	case EOR_REG:
		return t.trCondAluReg(inst, vm.OpSXor)
	case EORS_REG:
		return t.trCondAluRegFlags(inst, vm.OpSXor)
	case BIC_REG:
		return t.trCondBICReg(inst, false)
	case BICS_REG:
		return t.trCondBICReg(inst, true)
	case ADC_REG:
		return t.trCondAluReg(inst, vm.OpSAdc)
	case ADCS_REG:
		return t.trCondAluRegFlags(inst, vm.OpSAdc)
	case SBC_REG:
		return t.trCondAluReg(inst, vm.OpSSbc)
	case SBCS_REG:
		return t.trCondAluRegFlags(inst, vm.OpSSbc)
	case RSC_REG:
		return t.trCondRSBReg(inst, false)
	case RSCS_REG:
		return t.trCondRSBReg(inst, true)

	// ========== MOV/MVN (register) ==========
	case MOV_REG, MOVS_REG:
		return t.trCondMovReg(inst, op == MOVS_REG)
	case MVN_REG, MVNS_REG:
		return t.trCondMvnReg(inst, op == MVNS_REG)

	// ========== Compare/Test (register) ==========
	case CMP_REG:
		return t.trCondCmpReg(inst)
	case CMN_REG:
		return t.trCondCmnReg(inst)
	case TST_REG:
		return t.trCondTstReg(inst)
	case TEQ_REG:
		return t.trCondTeqReg(inst)

	// ========== Shift (register) ==========
	case LSL_REG:
		return t.trCondAluReg(inst, vm.OpSShl)
	case LSR_REG:
		return t.trCondAluReg(inst, vm.OpSShr)
	case ASR_REG:
		return t.trCondAluReg(inst, vm.OpSAsr)
	case ROR_REG:
		return t.trCondAluReg(inst, vm.OpSRor)

	// ========== Multiply ==========
	case MUL:
		return t.trCondMul(inst)
	case MLA:
		return t.trCondMLA(inst)
	case UMULL:
		return t.trCondLongMul(inst, false, false)
	case SMULL:
		return t.trCondLongMul(inst, true, false)
	case UMLAL:
		return t.trCondLongMul(inst, false, true)
	case SMLAL:
		return t.trCondLongMul(inst, true, true)
	case UDIV:
		return t.trCondAluReg(inst, vm.OpSUdiv)
	case SDIV:
		return t.trCondAluReg(inst, vm.OpSSdiv)

	// ========== Load/Store ==========
	case LDR_IMM, LDRB_IMM, LDRH_IMM, LDRSB_IMM, LDRSH_IMM:
		return t.trCondLoad(inst)
	case STR_IMM, STRB_IMM, STRH_IMM:
		return t.trCondStore(inst)
	case LDR_REG, LDRB_REG, LDRH_REG, LDRSB_REG, LDRSH_REG:
		return t.trCondLoadReg(inst)
	case STR_REG, STRB_REG, STRH_REG:
		return t.trCondStoreReg(inst)
	case LDRD_IMM:
		return t.trCondLoadDouble(inst)
	case STRD_IMM:
		return t.trCondStoreDouble(inst)

	// ========== Load/Store Multiple ==========
	case LDM:
		return t.trCondLDM(inst)
	case STM:
		return t.trCondSTM(inst)

	// ========== Branch ==========
	case B:
		return t.trCondBranch(inst)
	case BL:
		return t.trCondBL(inst)
	case BLX_IMM:
		return t.trCondBL(inst) // BLX(imm) same as BL: call to absolute PC-relative address
	case BX:
		return t.trCondBX(inst)
	case BLX_REG:
		return t.trCondBLX(inst)
	case CBZ:
		return t.trCBZCBNZ(inst, true)
	case CBNZ:
		return t.trCBZCBNZ(inst, false)

	// ========== System ==========
	case SVC:
		return t.trCondSVC(inst)
	case MRS:
		return t.trCondMRS(inst)
	case MSR:
		t.emit(vm.OpNop) // simplified
		return nil

	// ========== Bit manipulation ==========
	case CLZ:
		return t.trCondCLZ32(inst)
	case RBIT:
		return t.trCondUnary(inst, vm.OpSRbit)
	case REV:
		return t.trCondUnary(inst, vm.OpSRev32)
	case REV16:
		return t.trCondUnary(inst, vm.OpSRev16)

	// ========== PC-relative ==========
	case ADR:
		return t.trCondADR(inst)

	// ========== NOP barriers ==========
	case DMB, DSB, ISB:
		t.emit(vm.OpNop)
		return nil
	case BKPT:
		t.emit(vm.OpHalt)
		return nil

	case UNSUPPORTED, UNKNOWN:
		if t.literalPoolStart >= 0 && idx >= t.literalPoolStart {
			// Literal pool data at the end of the function — safe to skip
			t.emit(vm.OpNop)
			return nil
		}
		return fmt.Errorf("unrecognized instruction %s (raw=0x%08X)", OpName(op), inst.Raw)

	default:
		return fmt.Errorf("unsupported instruction type %s", OpName(op))
	}
}
