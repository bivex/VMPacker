package arm32

import (
	"fmt"

	"github.com/vmpacker/pkg/vm"
)

// Op - ARM32 instruction opcodes
type Op int

const (
	UNKNOWN Op = iota

	// Data processing (register/immediate)
	ADD_IMM
	ADD_REG
	SUB_IMM
	SUB_REG
	RSB_IMM
	RSB_REG
	AND_IMM
	AND_REG
	ORR_IMM
	ORR_REG
	EOR_IMM
	EOR_REG
	BIC_IMM
	BIC_REG
	MOV_IMM
	MOV_REG
	MVN_IMM
	MVN_REG
	ADC_IMM
	ADC_REG
	SBC_IMM
	SBC_REG
	RSC_IMM
	RSC_REG

	// Data processing with S flag
	ADDS_IMM
	ADDS_REG
	SUBS_IMM
	SUBS_REG
	RSBS_IMM
	RSBS_REG
	ANDS_IMM
	ANDS_REG
	ORRS_IMM
	ORRS_REG
	EORS_IMM
	EORS_REG
	BICS_IMM
	BICS_REG
	MOVS_IMM
	MOVS_REG
	MVNS_IMM
	MVNS_REG
	ADCS_IMM
	ADCS_REG
	SBCS_IMM
	SBCS_REG
	RSCS_IMM
	RSCS_REG

	// Compare/test (always set flags, no Rd)
	CMP_IMM
	CMP_REG
	CMN_IMM
	CMN_REG
	TST_IMM
	TST_REG
	TEQ_IMM
	TEQ_REG

	// ARMv7 wide immediate moves
	MOVW // MOVW Rd, #imm16
	MOVT // MOVT Rd, #imm16

	// Multiply
	MUL
	MLA
	UMULL
	SMULL
	UMLAL
	SMLAL

	// Divide (ARMv7-A with UDIV/SDIV)
	UDIV
	SDIV

	// Shift instructions (register)
	LSL_REG
	LSR_REG
	ASR_REG
	ROR_REG

	// Load/store word/byte (immediate offset)
	LDR_IMM
	STR_IMM
	LDRB_IMM
	STRB_IMM

	// Load/store word/byte (register offset)
	LDR_REG
	STR_REG
	LDRB_REG
	STRB_REG

	// Load/store halfword/signed (immediate)
	LDRH_IMM
	STRH_IMM
	LDRSB_IMM
	LDRSH_IMM

	// Load/store halfword/signed (register)
	LDRH_REG
	STRH_REG
	LDRSB_REG
	LDRSH_REG

	// Load/store multiple
	LDM
	STM

	// Load/store double (ARMv5TE+)
	LDRD_IMM
	STRD_IMM

	// Branch
	B
	BL
	BX
	BLX_REG
	BLX_IMM

	// System
	SVC
	NOP
	MRS
	MSR

	// Bit manipulation
	CLZ
	RBIT
	REV
	REV16

	// Barriers (ARMv7)
	DMB
	DSB
	ISB

	// Debug
	BKPT

	// PC-relative address
	ADR

	// Thumb-specific opcodes (shared Op space)
	IT   // Thumb IT block
	CBZ  // Compare and Branch if Zero
	CBNZ // Compare and Branch if Not Zero

	UNSUPPORTED
)

// ARM32 condition codes (bits[31:28])
const (
	COND_EQ = 0x0 // Equal (Z=1)
	COND_NE = 0x1 // Not equal (Z=0)
	COND_CS = 0x2 // Carry set / unsigned higher or same
	COND_CC = 0x3 // Carry clear / unsigned lower
	COND_MI = 0x4 // Negative
	COND_PL = 0x5 // Positive or zero
	COND_VS = 0x6 // Overflow
	COND_VC = 0x7 // No overflow
	COND_HI = 0x8 // Unsigned higher
	COND_LS = 0x9 // Unsigned lower or same
	COND_GE = 0xA // Signed greater or equal
	COND_LT = 0xB // Signed less than
	COND_GT = 0xC // Signed greater than
	COND_LE = 0xD // Signed less or equal
	COND_AL = 0xE // Always
)

// Decoder - ARM32 decoder implementing vm.Decoder interface
// Supports both ARM (A32) and Thumb/Thumb-2 (T32) modes.
type Decoder struct {
	thumbMode bool // true = Thumb/Thumb-2 mode
	itState   itBlockState
}

// itBlockState tracks Thumb IT block state
type itBlockState struct {
	mask     byte // IT mask (remaining conditions)
	baseCond int  // base condition code
	count    int  // instructions remaining in IT block
}

// NewDecoder creates an ARM32 A32 decoder
func NewDecoder() *Decoder {
	return &Decoder{thumbMode: false}
}

// NewThumbDecoder creates a Thumb/Thumb-2 decoder
func NewThumbDecoder() *Decoder {
	return &Decoder{thumbMode: true}
}

// IsThumbMode returns whether the decoder is in Thumb mode
func (d *Decoder) IsThumbMode() bool {
	return d.thumbMode
}

// Decode decodes a single ARM32 or Thumb instruction.
// For ARM mode: raw is a 32-bit ARM instruction.
// For Thumb mode: raw is either 16-bit (zero-extended) or 32-bit Thumb-2.
func (d *Decoder) Decode(raw uint32, offset int) vm.Instruction {
	if d.thumbMode {
		return d.decodeThumb(raw, offset)
	}
	return d.decodeARM(raw, offset)
}

// decodeARM decodes a 32-bit ARM (A32) instruction
func (d *Decoder) decodeARM(raw uint32, offset int) vm.Instruction {
	inst := vm.Instruction{Raw: raw, Op: int(UNKNOWN), Offset: offset, Rd: -1, Rn: -1, Rm: -1}

	cond := int((raw >> 28) & 0xF)
	inst.Cond = cond

	// Unconditional instructions (cond=0xF)
	if cond == 0xF {
		matchAndDecode(raw, unconditionalPatterns, &inst)
		if inst.Op == int(UNKNOWN) {
			inst.Op = int(UNSUPPORTED)
		}
		return inst
	}

	// NOP: MOV R0, R0 with cond=AL
	if raw == 0xE1A00000 {
		inst.Op = int(NOP)
		inst.Cond = COND_AL
		return inst
	}
	// ARMv6K+ NOP hint
	if raw == 0xE320F000 {
		inst.Op = int(NOP)
		inst.Cond = COND_AL
		return inst
	}

	op1 := (raw >> 25) & 0x7

	var matched bool
	switch op1 {
	case 0b000:
		// Data processing (register) / Multiply / Misc
		matched = matchAndDecode(raw, dpRegPatterns, &inst)
	case 0b001:
		// Data processing (immediate) / Move immediate
		matched = matchAndDecode(raw, dpImmPatterns, &inst)
	case 0b010:
		// Load/store (immediate offset)
		matched = matchAndDecode(raw, ldstImmPatterns, &inst)
	case 0b011:
		if raw&0x10 == 0 {
			// Load/store (register offset): bit[4]=0
			matched = matchAndDecode(raw, ldstRegPatterns, &inst)
		} else {
			// Media instructions: bit[4]=1 (UXTB, UXTH, BFI, etc.)
			matched = matchAndDecode(raw, mediaPatterns, &inst)
		}
	case 0b100:
		// Load/store multiple
		matched = matchAndDecode(raw, ldstMultiPatterns, &inst)
	case 0b101:
		// Branch
		matched = matchAndDecode(raw, branchPatterns, &inst)
	case 0b111:
		// SVC / coprocessor
		matched = matchAndDecode(raw, svcPatterns, &inst)
	}

	if !matched {
		// Try extra halfword/signed load/store (op1=000 but special encoding)
		if op1 == 0b000 {
			matched = matchAndDecode(raw, ldstHalfPatterns, &inst)
		}
	}

	if !matched {
		inst.Op = int(UNSUPPORTED)
	}

	return inst
}

// DecodeThumbPair decodes a Thumb instruction from two halfwords.
// Returns the decoded instruction and the instruction size in bytes (2 or 4).
func (d *Decoder) DecodeThumbPair(hw1, hw2 uint16, offset int) (vm.Instruction, int) {
	if IsThumb32(hw1) {
		raw := (uint32(hw1) << 16) | uint32(hw2)
		inst := d.decodeThumb(raw, offset)
		return inst, 4
	}
	inst := d.decodeThumb(uint32(hw1), offset)
	return inst, 2
}

// IsThumb32 returns true if the first halfword indicates a 32-bit Thumb-2 instruction
func IsThumb32(hw uint16) bool {
	top5 := hw >> 11
	return top5 == 0x1D || top5 == 0x1E || top5 == 0x1F
}

// InstName returns the instruction name
func (d *Decoder) InstName(op int) string {
	return OpName(Op(op))
}

// SignExtend performs sign extension
func SignExtend(val uint32, bits int) int64 {
	sign := uint32(1) << (bits - 1)
	mask := sign - 1
	if val&sign != 0 {
		return int64(int32(val | ^mask))
	}
	return int64(val & mask)
}

// decodeRotImm decodes ARM32 rotated immediate: imm8 ROR (rot*2)
func decodeRotImm(imm8, rot uint32) uint32 {
	shift := (rot * 2) & 31
	val := imm8
	if shift != 0 {
		val = (imm8 >> shift) | (imm8 << (32 - shift))
	}
	return val
}

// OpName returns the string name for an ARM32 opcode
func OpName(op Op) string {
	names := map[Op]string{
		ADD_IMM: "ADD(imm)", ADD_REG: "ADD(reg)",
		SUB_IMM: "SUB(imm)", SUB_REG: "SUB(reg)",
		RSB_IMM: "RSB(imm)", RSB_REG: "RSB(reg)",
		AND_IMM: "AND(imm)", AND_REG: "AND(reg)",
		ORR_IMM: "ORR(imm)", ORR_REG: "ORR(reg)",
		EOR_IMM: "EOR(imm)", EOR_REG: "EOR(reg)",
		BIC_IMM: "BIC(imm)", BIC_REG: "BIC(reg)",
		MOV_IMM: "MOV(imm)", MOV_REG: "MOV(reg)",
		MVN_IMM: "MVN(imm)", MVN_REG: "MVN(reg)",
		ADC_IMM: "ADC(imm)", ADC_REG: "ADC(reg)",
		SBC_IMM: "SBC(imm)", SBC_REG: "SBC(reg)",
		RSC_IMM: "RSC(imm)", RSC_REG: "RSC(reg)",
		ADDS_IMM: "ADDS(imm)", ADDS_REG: "ADDS(reg)",
		SUBS_IMM: "SUBS(imm)", SUBS_REG: "SUBS(reg)",
		RSBS_IMM: "RSBS(imm)", RSBS_REG: "RSBS(reg)",
		ANDS_IMM: "ANDS(imm)", ANDS_REG: "ANDS(reg)",
		ORRS_IMM: "ORRS(imm)", ORRS_REG: "ORRS(reg)",
		EORS_IMM: "EORS(imm)", EORS_REG: "EORS(reg)",
		BICS_IMM: "BICS(imm)", BICS_REG: "BICS(reg)",
		MOVS_IMM: "MOVS(imm)", MOVS_REG: "MOVS(reg)",
		MVNS_IMM: "MVNS(imm)", MVNS_REG: "MVNS(reg)",
		ADCS_IMM: "ADCS(imm)", ADCS_REG: "ADCS(reg)",
		SBCS_IMM: "SBCS(imm)", SBCS_REG: "SBCS(reg)",
		RSCS_IMM: "RSCS(imm)", RSCS_REG: "RSCS(reg)",
		CMP_IMM: "CMP(imm)", CMP_REG: "CMP(reg)",
		CMN_IMM: "CMN(imm)", CMN_REG: "CMN(reg)",
		TST_IMM: "TST(imm)", TST_REG: "TST(reg)",
		TEQ_IMM: "TEQ(imm)", TEQ_REG: "TEQ(reg)",
		MOVW: "MOVW", MOVT: "MOVT",
		MUL: "MUL", MLA: "MLA",
		UMULL: "UMULL", SMULL: "SMULL",
		UMLAL: "UMLAL", SMLAL: "SMLAL",
		UDIV: "UDIV", SDIV: "SDIV",
		LSL_REG: "LSL(reg)", LSR_REG: "LSR(reg)",
		ASR_REG: "ASR(reg)", ROR_REG: "ROR(reg)",
		LDR_IMM: "LDR(imm)", STR_IMM: "STR(imm)",
		LDRB_IMM: "LDRB(imm)", STRB_IMM: "STRB(imm)",
		LDR_REG: "LDR(reg)", STR_REG: "STR(reg)",
		LDRB_REG: "LDRB(reg)", STRB_REG: "STRB(reg)",
		LDRH_IMM: "LDRH(imm)", STRH_IMM: "STRH(imm)",
		LDRSB_IMM: "LDRSB(imm)", LDRSH_IMM: "LDRSH(imm)",
		LDRH_REG: "LDRH(reg)", STRH_REG: "STRH(reg)",
		LDRSB_REG: "LDRSB(reg)", LDRSH_REG: "LDRSH(reg)",
		LDRD_IMM: "LDRD(imm)", STRD_IMM: "STRD(imm)",
		LDM: "LDM", STM: "STM",
		B: "B", BL: "BL", BX: "BX", BLX_REG: "BLX", BLX_IMM: "BLX(imm)",
		SVC: "SVC", NOP: "NOP", MRS: "MRS", MSR: "MSR",
		CLZ: "CLZ", RBIT: "RBIT", REV: "REV", REV16: "REV16",
		DMB: "DMB", DSB: "DSB", ISB: "ISB",
		BKPT: "BKPT", ADR: "ADR", IT: "IT",
		CBZ: "CBZ", CBNZ: "CBNZ",
	}
	if n, ok := names[op]; ok {
		return n
	}
	return fmt.Sprintf("UNKNOWN(0x%X)", int(op))
}
