package arm32

import "github.com/vmpacker/pkg/vm"

// FieldDef describes a bit field in an instruction word
type FieldDef struct {
	Name   string
	Hi     int  // high bit (inclusive)
	Lo     int  // low bit (inclusive)
	Signed bool
}

// PostFunc optional post-processing callback
type PostFunc func(fields map[string]int64, inst *vm.Instruction)

// InstrPattern defines a mask/value pattern for instruction matching
type InstrPattern struct {
	Name   string
	Mask   uint32
	Value  uint32
	Op     Op
	Fields []FieldDef
	Post   PostFunc
}

// extractField extracts a single bit field from raw
func extractField(raw uint32, f FieldDef) int64 {
	width := f.Hi - f.Lo + 1
	mask := uint32((1 << width) - 1)
	val := (raw >> uint(f.Lo)) & mask
	if f.Signed {
		return SignExtend(val, width)
	}
	return int64(val)
}

// extractFields extracts all defined fields from raw
func extractFields(raw uint32, fields []FieldDef) map[string]int64 {
	result := make(map[string]int64, len(fields))
	for _, f := range fields {
		result[f.Name] = extractField(raw, f)
	}
	return result
}

// applyCommonFields maps common field names to vm.Instruction
func applyCommonFields(fields map[string]int64, inst *vm.Instruction) {
	if v, ok := fields["Rd"]; ok {
		inst.Rd = int(v)
	}
	if v, ok := fields["Rn"]; ok {
		inst.Rn = int(v)
	}
	if v, ok := fields["Rm"]; ok {
		inst.Rm = int(v)
	}
	if v, ok := fields["shift"]; ok {
		inst.Shift = int(v)
	}
	if v, ok := fields["shtype"]; ok {
		inst.ShiftType = int(v)
	}
}

// matchAndDecode finds the first matching pattern and decodes
func matchAndDecode(raw uint32, patterns []InstrPattern, inst *vm.Instruction) bool {
	for i := range patterns {
		p := &patterns[i]
		if raw&p.Mask == p.Value {
			inst.Op = int(p.Op)
			fields := extractFields(raw, p.Fields)
			applyCommonFields(fields, inst)
			if p.Post != nil {
				p.Post(fields, inst)
			}
			return true
		}
	}
	return false
}

// Common field definitions for ARM32
var (
	fRd   = FieldDef{Name: "Rd", Hi: 15, Lo: 12}
	fRn   = FieldDef{Name: "Rn", Hi: 19, Lo: 16}
	fRm   = FieldDef{Name: "Rm", Hi: 3, Lo: 0}
	fRs   = FieldDef{Name: "Rs", Hi: 11, Lo: 8}
	fRdHi = FieldDef{Name: "RdHi", Hi: 19, Lo: 16}
	fRdLo = FieldDef{Name: "RdLo", Hi: 15, Lo: 12}
)

// postRotImm resolves the ARM32 rotated immediate (operand2 in DP immediate)
func postRotImm(fields map[string]int64, inst *vm.Instruction) {
	imm8 := uint32(fields["imm8"]) & 0xFF
	rot := uint32(fields["rot"]) & 0xF
	inst.Imm = int64(decodeRotImm(imm8, rot))
}

// postShiftedReg resolves the barrel shifter operand2 for register mode.
// Sets inst.Shift and inst.ShiftType; if shift by register, inst.Imm = -1 as flag.
func postShiftedReg(fields map[string]int64, inst *vm.Instruction) {
	shtype := int(fields["shtype"])
	inst.ShiftType = shtype

	if rs, ok := fields["Rs"]; ok && fields["bit4"] == 1 {
		// Shift by register: Rs in bits[11:8], bit[4]=1
		inst.Imm = -1 // flag: shift amount is in register Rs
		inst.Shift = int(rs)
	} else {
		// Shift by immediate: shamt in bits[11:7]
		shamt := int(fields["shamt"])
		inst.Shift = shamt
	}
}

// postDPImm handles data processing immediate: decode rotated imm + set S flag
func postDPImm(fields map[string]int64, inst *vm.Instruction) {
	postRotImm(fields, inst)
}

// postDPReg handles data processing register: decode shifted reg + set S flag
func postDPReg(fields map[string]int64, inst *vm.Instruction) {
	postShiftedReg(fields, inst)
}

// postDPRegShift handles data processing register-shifted register
func postDPRegShift(fields map[string]int64, inst *vm.Instruction) {
	postShiftedReg(fields, inst)
}
