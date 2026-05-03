package arm64

import "github.com/vmpacker/pkg/vm"

// ============================================================
// ARM64 table-driven decoding engine
//
// FieldDef - describes a bit field's Hi/Lo position, eliminating manual shift ambiguity
// InstrPattern - Mask/Value match + Fields auto-extract + Post callback
// ============================================================

// FieldDef - bit field definition
type FieldDef struct {
	Name   string // field name: "sf", "Rd", "Rn", "imm7" ...
	Hi     int    // high bit (inclusive), e.g. bit31 → Hi=31
	Lo     int    // low bit (inclusive), e.g. bit0 → Lo=0
	Signed bool   // whether it has sign extension
}

// PostFunc - post-processing callback: handles logic that tables can't express (XZR replacement, offset scaling, etc.)
type PostFunc func(fields map[string]int64, inst *vm.Instruction)

// InstrPattern - instruction pattern definition
type InstrPattern struct {
	Name   string     // debug name, e.g. "ADD_IMM"
	Mask   uint32     // fixed bit mask
	Value  uint32     // expected fixed bit value
	Op     Op         // decoded instruction type
	Fields []FieldDef // bit field definitions
	Post   PostFunc   // optional post-processing
}

// ---- Bit field extraction ----

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

// extractFields extracts all bit fields from raw, returns name→value map
func extractFields(raw uint32, fields []FieldDef) map[string]int64 {
	result := make(map[string]int64, len(fields))
	for _, f := range fields {
		result[f.Name] = extractField(raw, f)
	}
	return result
}

// ---- Common field mapping ----

// applyCommonFields maps common field names to vm.Instruction
//
// Convention: Rd→inst.Rd, Rn→inst.Rn, Rm→inst.Rm, sf→inst.SF,
//       cond→inst.Cond, wb→inst.WB, shift→inst.Shift
//
// inst.Imm is set by each instruction's Post callback (imm width/scale varies)
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
	if v, ok := fields["sf"]; ok {
		inst.SF = v != 0
	}
	if v, ok := fields["cond"]; ok {
		inst.Cond = int(v)
	}
	if v, ok := fields["wb"]; ok {
		inst.WB = int(v)
	}
	if v, ok := fields["shift"]; ok {
		inst.Shift = int(v)
	}
}

// ---- Pattern matching ----

// matchAndDecode finds the first matching pattern in patterns, decodes and fills inst
// Returns whether a match was found
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

// ---- Helper functions ----

// xzrReplace ARM64 XZR marker: reg==31 → REG_XZR
func xzrReplace(reg *int) {
	if *reg == 31 {
		*reg = vm.REG_XZR
	}
}
