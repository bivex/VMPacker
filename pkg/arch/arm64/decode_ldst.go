package arm64

import "github.com/vmpacker/pkg/vm"

// ============================================================
// 加载/存储 模式表
//
// 覆盖: LDP/STP, LDR/STR(imm unsigned/pre/post), LDRB/STRB,
//       LDRH/STRH, LDR/STR(reg), LDRSB/LDRSH/LDRSW,
//       LD1/ST1(SIMD)
// ============================================================

var ldstPatterns = []InstrPattern{
	// ================================================================
	// LDP/STP (load/store pair)
	// 编码: opc:101:V:cat:L:imm7:Rt2:Rn:Rt
	//   opc=x0 → 32-bit, opc=1x → 64-bit  (sf=bit31)
	//   V=0 integer
	//   cat: 001=post, 010=signed-offset, 011=pre
	//   L: 0=store, 1=load
	// 匹配: bits[28:27]=01, bit26=0 → pair integer
	// ================================================================
	{
		Name: "STP", Mask: 0x1C400000, Value: 0x08000000, Op: STP,
		Fields: []FieldDef{
			{Name: "sf", Hi: 31, Lo: 31},
			{Name: "wb", Hi: 25, Lo: 23},
			{Name: "imm7", Hi: 21, Lo: 15, Signed: true},
			{Name: "Rm", Hi: 14, Lo: 10}, // Rt2
			fRn, fRd,                     // Rn, Rt
		},
		Post: postPair,
	},
		// LDP integer (V=0, L=1): same as STP but bit22=1 (load)
		{
			Name: "LDP_INT", Mask: 0x1C400000, Value: 0x08400000, Op: LDP,
			Fields: []FieldDef{
				{Name: "sf", Hi: 31, Lo: 31},
				{Name: "wb", Hi: 25, Lo: 23},
				{Name: "imm7", Hi: 21, Lo: 15, Signed: true},
				{Name: "Rm", Hi: 14, Lo: 10}, // Rt2
				fRn, fRd,                     // Rn, Rt
			},
			Post: postPair,
		},
	// ================================================================
	// LDP/STP (SIMD/FP)
	// 编码: opc:101:V:cat:L:imm7:Rt2:Rn:Rt
	//   V=1 (bit 26)
	//   bits[29:27] = 101
	//   Mask: 0x3E400000, Value: 0x2C400000 for LDP, 0x2C000000 for STP
	// ================================================================
	{
		Name: "STP_SIMD", Mask: 0x3E400000, Value: 0x2C000000, Op: STP,
		Fields: []FieldDef{{Name: "opc", Hi: 31, Lo: 30}, {Name: "wb", Hi: 25, Lo: 23}, {Name: "imm7", Hi: 21, Lo: 15, Signed: true}, {Name: "Rm", Hi: 14, Lo: 10}, fRn, fRd},
		Post:   postPairSIMD,
	},
	{
		Name: "LDP_SIMD", Mask: 0x3E400000, Value: 0x2C400000, Op: LDP,
		Fields: []FieldDef{{Name: "opc", Hi: 31, Lo: 30}, {Name: "wb", Hi: 25, Lo: 23}, {Name: "imm7", Hi: 21, Lo: 15, Signed: true}, {Name: "Rm", Hi: 14, Lo: 10}, fRn, fRd},
		Post:   postPairSIMD,
	},


	// ================================================================
	// LDPSW (load pair signed word): opc=01, V=0, L=1
	// 编码: 01:101:0:0:cat:1:imm7:Rt2:Rn:Rt
	// cat: 001=post, 010=signed-offset, 011=pre
	// Loads two 32-bit words, sign-extends them to 64-bit
	// ================================================================
	{
		Name: "LDPSW", Mask: 0xFC400000, Value: 0x68400000, Op: LDPSW,
		Fields: []FieldDef{
			{Name: "wb", Hi: 25, Lo: 23},
			{Name: "imm7", Hi: 21, Lo: 15, Signed: true},
			{Name: "Rm", Hi: 14, Lo: 10},
			fRn, fRd,
		},
		Post: postLdpsw,
	},

	// ================================================================
	// SIMD load/store (LD1/ST1 multiple structures)
	// 编码: 0:Q:001100:L:000000:opcode:00:Rn:Rt
	// ================================================================
	{
		Name: "LD1_16B", Mask: 0xBFFF0000, Value: 0x0C400000, Op: LD1_16B,
		Fields: []FieldDef{fRn, fRd, {Name: "opcode", Hi: 15, Lo: 12}},
		Post:   postSimdMulti,
	},
	{
		Name: "ST1_16B", Mask: 0xBFFF0000, Value: 0x0C000000, Op: ST1_16B,
		Fields: []FieldDef{fRn, fRd, {Name: "opcode", Hi: 15, Lo: 12}},
		Post:   postSimdMulti,
	},

	// ================================================================
	// Load/Store register (register offset)
	// 编码: size:111:V:00:opc:1:Rm:option:S:10:Rn:Rt
	// ================================================================
	{
		Name: "LDRB_REG", Mask: 0xFFE00C00, Value: 0x38600800, Op: LDRB_REG,
		Fields: []FieldDef{fRm16, fRn, fRd},
	},
	{
		Name: "STRB_REG", Mask: 0xFFE00C00, Value: 0x38200800, Op: STRB_REG,
		Fields: []FieldDef{fRm16, fRn, fRd},
	},
	{
		Name: "LDR_REG_32", Mask: 0xFFE00C00, Value: 0xB8600800, Op: LDR_REG,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post:   func(_ map[string]int64, inst *vm.Instruction) { inst.SF = false },
	},
	{
		Name: "STR_REG_32", Mask: 0xFFE00C00, Value: 0xB8200800, Op: STR_REG,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post:   func(_ map[string]int64, inst *vm.Instruction) { inst.SF = false },
	},
	{
		Name: "LDR_REG_64", Mask: 0xFFE00C00, Value: 0xF8600800, Op: LDR_REG,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post:   func(_ map[string]int64, inst *vm.Instruction) { inst.SF = true },
	},
	{
		Name: "STR_REG_64", Mask: 0xFFE00C00, Value: 0xF8200800, Op: STR_REG,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post:   func(_ map[string]int64, inst *vm.Instruction) { inst.SF = true },
	},

	// ---- Register offset: LDRH/STRH ----
	// LDRH (reg): size=01,V=0,opc=01 → 0x78600800
	{
		Name: "LDRH_REG", Mask: 0xFFE00C00, Value: 0x78600800, Op: LDRH_REG,
		Fields: []FieldDef{fRm16, fRn, fRd},
	},
	// STRH (reg): size=01,V=0,opc=00 → 0x78200800
	{
		Name: "STRH_REG", Mask: 0xFFE00C00, Value: 0x78200800, Op: STRH_REG,
		Fields: []FieldDef{fRm16, fRn, fRd},
	},
	// LDRSB (reg): size=00,V=0,opc=10 → 0x38A00800 (64-bit dest)
	{
		Name: "LDRSB_REG", Mask: 0xFFE00C00, Value: 0x38A00800, Op: LDRSB_REG,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post:   func(_ map[string]int64, inst *vm.Instruction) { inst.SF = true },
	},
	// LDRSH (reg): size=01,V=0,opc=10 → 0x78A00800
	{
		Name: "LDRSH_REG", Mask: 0xFFE00C00, Value: 0x78A00800, Op: LDRSH_REG,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post:   func(_ map[string]int64, inst *vm.Instruction) { inst.SF = true },
	},
	// LDRSW (reg): size=10,V=0,opc=10 → 0xB8A00800
	{
		Name: "LDRSW_REG", Mask: 0xFFE00C00, Value: 0xB8A00800, Op: LDRSW_REG,
		Fields: []FieldDef{fRm16, fRn, fRd},
		Post:   func(_ map[string]int64, inst *vm.Instruction) { inst.SF = true },
	},

	// ================================================================
	// Load/Store register (unscaled immediate) LDUR/STUR
	// 编码: size:111:V:00:opc:0:imm9:00:Rn:Rt  (bits[11:10]=00)
	// 复用 LDR_IMM/STR_IMM Op，offset 不缩放
	// ================================================================
	// LDUR 64-bit
	{
		Name: "LDUR_64", Mask: 0xFFE00C00, Value: 0xF8400000, Op: LDR_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, fRn, fRd},
		Post:   postUnscaled(true),
	},
	// STUR 64-bit
	{
		Name: "STUR_64", Mask: 0xFFE00C00, Value: 0xF8000000, Op: STR_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, fRn, fRd},
		Post:   postUnscaled(true),
	},
	// LDUR 32-bit
	{
		Name: "LDUR_32", Mask: 0xFFE00C00, Value: 0xB8400000, Op: LDR_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, fRn, fRd},
		Post:   postUnscaled(false),
	},
	// STUR 32-bit
	{
		Name: "STUR_32", Mask: 0xFFE00C00, Value: 0xB8000000, Op: STR_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, fRn, fRd},
		Post:   postUnscaled(false),
	},
	// STURB (byte unscaled)
	{
		Name: "STURB", Mask: 0xFFE00C00, Value: 0x38000000, Op: STRB_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, fRn, fRd},
		Post:   postUnscaled(false),
	},
	// LDURB (byte unscaled)
	{
		Name: "LDURB", Mask: 0xFFE00C00, Value: 0x38400000, Op: LDRB_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, fRn, fRd},
		Post:   postUnscaled(false),
	},
	// LDURH (halfword unscaled): size=01,V=0,opc=01 → 0x78400000
	{
		Name: "LDURH", Mask: 0xFFE00C00, Value: 0x78400000, Op: LDRH_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, fRn, fRd},
		Post:   postUnscaled(false),
	},
	// STURH (halfword unscaled): size=01,V=0,opc=00 → 0x78000000
	{
		Name: "STURH", Mask: 0xFFE00C00, Value: 0x78000000, Op: STRH_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, fRn, fRd},
		Post:   postUnscaled(false),
	},
	// LDURSB (signed byte unscaled): size=00,V=0,opc=10 → 0x38800000
	{
		Name: "LDURSB", Mask: 0xFFE00C00, Value: 0x38800000, Op: LDRSB_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, fRn, fRd},
		Post:   postUnscaled(true),
	},
	// LDURSH (signed halfword unscaled): size=01,V=0,opc=10 → 0x78800000
	{
		Name: "LDURSH", Mask: 0xFFE00C00, Value: 0x78800000, Op: LDRSH_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, fRn, fRd},
		Post:   postUnscaled(true),
	},
	// LDURSW (signed word unscaled): size=10,V=0,opc=10 → 0xB8800000
	{
		Name: "LDURSW", Mask: 0xFFE00C00, Value: 0xB8800000, Op: LDRSW_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, fRn, fRd},
		Post:   postUnscaled(true),
	},

	// ================================================================
	// Load/Store register (immediate pre/post-index)
	// 编码: size:111:V:00:opc:0:imm9:wb:Rn:Rt
	//   wb=01 → post-index, wb=11 → pre-index
	// ================================================================
	// STR 32-bit post
	{
		Name: "STR_IMM32_POST", Mask: 0xFFE00400, Value: 0xB8000400, Op: STR_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, {Name: "wb", Hi: 11, Lo: 10}, fRn, fRd},
		Post:   postLdrStrPrePost(false),
	},
	// STR 64-bit post
	{
		Name: "STR_IMM64_POST", Mask: 0xFFE00400, Value: 0xF8000400, Op: STR_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, {Name: "wb", Hi: 11, Lo: 10}, fRn, fRd},
		Post:   postLdrStrPrePost(true),
	},
	// LDR 32-bit post
	{
		Name: "LDR_IMM32_POST", Mask: 0xFFE00400, Value: 0xB8400400, Op: LDR_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, {Name: "wb", Hi: 11, Lo: 10}, fRn, fRd},
		Post:   postLdrStrPrePost(false),
	},
	// LDR 64-bit post
	{
		Name: "LDR_IMM64_POST", Mask: 0xFFE00400, Value: 0xF8400400, Op: LDR_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, {Name: "wb", Hi: 11, Lo: 10}, fRn, fRd},
		Post:   postLdrStrPrePost(true),
	},
	// STRB pre/post
	{
		Name: "STRB_IMM_PP", Mask: 0xFFE00400, Value: 0x38000400, Op: STRB_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, {Name: "wb", Hi: 11, Lo: 10}, fRn, fRd},
		Post:   postLdrStrPrePostXZR,
	},
	// LDRB pre/post
	{
		Name: "LDRB_IMM_PP", Mask: 0xFFE00400, Value: 0x38400400, Op: LDRB_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, {Name: "wb", Hi: 11, Lo: 10}, fRn, fRd},
		Post:   postLdrStrPrePostXZR,
	},
	// STRH pre/post
	{
		Name: "STRH_IMM_PP", Mask: 0xFFE00400, Value: 0x78000400, Op: STRH_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, {Name: "wb", Hi: 11, Lo: 10}, fRn, fRd},
		Post:   postLdrStrPrePostXZR,
	},
	// LDRH pre/post
	{
		Name: "LDRH_IMM_PP", Mask: 0xFFE00400, Value: 0x78400400, Op: LDRH_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, {Name: "wb", Hi: 11, Lo: 10}, fRn, fRd},
		Post:   postLdrStrPrePostXZR,
	},
	// LDRSB pre/post: size=00,V=0,opc=10,wb → 0x38800400
	{
		Name: "LDRSB_IMM_PP", Mask: 0xFFE00400, Value: 0x38800400, Op: LDRSB_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, {Name: "wb", Hi: 11, Lo: 10}, fRn, fRd},
		Post:   postLdrStrPrePostXZR,
	},
	// LDRSH pre/post: size=01,V=0,opc=10,wb → 0x78800400
	{
		Name: "LDRSH_IMM_PP", Mask: 0xFFE00400, Value: 0x78800400, Op: LDRSH_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, {Name: "wb", Hi: 11, Lo: 10}, fRn, fRd},
		Post:   postLdrStrPrePostXZR,
	},
	// LDRSW pre/post: size=10,V=0,opc=10,wb → 0xB8800400
	{
		Name: "LDRSW_IMM_PP", Mask: 0xFFE00400, Value: 0xB8800400, Op: LDRSW_IMM,
		Fields: []FieldDef{{Name: "imm9", Hi: 20, Lo: 12, Signed: true}, {Name: "wb", Hi: 11, Lo: 10}, fRn, fRd},
		Post:   postLdrStrPrePostXZR,
	},

	// ================================================================
	// Load/Store register (unsigned offset imm12)
	// 编码: size:111:V:01:opc:imm12:Rn:Rt
	// ================================================================
	// LDR 64-bit unsigned
	{
		Name: "LDR_UIMM64", Mask: 0xFFC00000, Value: 0xF9400000, Op: LDR_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(8, true),
	},
	// STR 64-bit unsigned
	{
		Name: "STR_UIMM64", Mask: 0xFFC00000, Value: 0xF9000000, Op: STR_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(8, true),
	},
	// LDR 32-bit unsigned
	{
		Name: "LDR_UIMM32", Mask: 0xFFC00000, Value: 0xB9400000, Op: LDR_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(4, false),
	},
	// STR 32-bit unsigned
	{
		Name: "STR_UIMM32", Mask: 0xFFC00000, Value: 0xB9000000, Op: STR_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(4, false),
	},
	// LDRB unsigned
	{
		Name: "LDRB_UIMM", Mask: 0xFFC00000, Value: 0x39400000, Op: LDRB_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(1, false),
	},
	// STRB unsigned
	{
		Name: "STRB_UIMM", Mask: 0xFFC00000, Value: 0x39000000, Op: STRB_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(1, false),
	},
	// LDRH unsigned
	{
		Name: "LDRH_UIMM", Mask: 0xFFC00000, Value: 0x79400000, Op: LDRH_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(2, false),
	},
	// STRH unsigned
	{
		Name: "STRH_UIMM", Mask: 0xFFC00000, Value: 0x79000000, Op: STRH_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(2, false),
	},
	// LDRSW unsigned (size=10, opc=10)
	{
		Name: "LDRSW_UIMM", Mask: 0xFFC00000, Value: 0xB9800000, Op: LDRSW_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(4, true),
	},
	// LDRSB unsigned (size=00, opc=10) → 64-bit dest
	{
		Name: "LDRSB_UIMM64", Mask: 0xFFC00000, Value: 0x39800000, Op: LDRSB_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(1, true),
	},
	// LDRSB unsigned (size=00, opc=11) → 32-bit dest
	{
		Name: "LDRSB_UIMM32", Mask: 0xFFC00000, Value: 0x39C00000, Op: LDRSB_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(1, false),
	},
	// LDRSH unsigned (size=01, opc=10)
	{
		Name: "LDRSH_UIMM", Mask: 0xFFC00000, Value: 0x79800000, Op: LDRSH_IMM,
		Fields: []FieldDef{{Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postUnsigned(2, true),
	},

	// ================================================================
	// LDR/STR (SIMD/FP) - Unsigned immediate
	// 编码: size:111:V:01:opc:imm12:Rn:Rt
	//   V=1, bits[29:27]=111, bits[25:24]=01
	//   Mask: 0x3BC00000 (Exclude size bits [31:30] and opc bits [23:22] from fixed match)
	//   Value: 0x39400000
	// ================================================================
	{
		Name: "LDR_SIMD_UIMM", Mask: 0x3BC00000, Value: 0x39400000, Op: LDR_IMM,
		Fields: []FieldDef{{Name: "size", Hi: 31, Lo: 30}, {Name: "opc", Hi: 23, Lo: 22}, {Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postLdrStrSIMD(true),
	},
	{
		Name: "STR_SIMD_UIMM", Mask: 0x3BC00000, Value: 0x39000000, Op: STR_IMM,
		Fields: []FieldDef{{Name: "size", Hi: 31, Lo: 30}, {Name: "opc", Hi: 23, Lo: 22}, {Name: "imm12", Hi: 21, Lo: 10}, fRn, fRd},
		Post:   postLdrStrSIMD(false),
	},

	// ================================================================
	// Load register (literal / PC-relative)
	// 编码: opc:011:V:00:imm19:Rt
	//   opc=00,V=0 → LDR Wt   (32-bit)
	//   opc=01,V=0 → LDR Xt   (64-bit)
	//   opc=10,V=0 → LDRSW Xt (32→64 sign-extend)
	// PC-relative offset = sign_extend(imm19) * 4
	// ================================================================
	// LDR Xt, [PC+imm] (64-bit)
	{
		Name: "LDR_LIT_64", Mask: 0xFF000000, Value: 0x58000000, Op: LDR_LIT,
		Fields: []FieldDef{{Name: "imm19", Hi: 23, Lo: 5, Signed: true}, fRd},
		Post:   postLdrLiteral(true, false),
	},
	// LDR Wt, [PC+imm] (32-bit)
	{
		Name: "LDR_LIT_32", Mask: 0xFF000000, Value: 0x18000000, Op: LDR_LIT,
		Fields: []FieldDef{{Name: "imm19", Hi: 23, Lo: 5, Signed: true}, fRd},
		Post:   postLdrLiteral(false, false),
	},
	// LDRSW Xt, [PC+imm] (32-bit sign-extended to 64)
	{
		Name: "LDRSW_LIT", Mask: 0xFF000000, Value: 0x98000000, Op: LDR_LIT,
		Fields: []FieldDef{{Name: "imm19", Hi: 23, Lo: 5, Signed: true}, fRd},
		Post:   postLdrLiteral(true, true),
	},

	// ================================================================
	// PRFM (immediate, unsigned offset)
	// 编码: 11:111:0:01:10:imm12:Rn:Rt(prfop)
	// 与 LDR(imm) 64-bit unsigned 共享 op0 分组，opc=10 区分
	// VMP 中 NOP 化：预取不影响语义
	// ================================================================
	{
		Name: "PRFM_IMM", Mask: 0xFFC00000, Value: 0xF9800000, Op: PRFM,
		Fields: []FieldDef{fRn},
	},

	// ================================================================
	// Load-Acquire / Store-Release / Load-Acquire Exclusive / Store-Release Exclusive
	// 编码: size:001000:o2:L:o1:Rs:o0:Rt2:Rn:Rt
	// ================================================================
	// LDAR: size:001000:1:1:0:11111:1:11111:Rn:Rt
	{
		Name: "LDAR", Mask: 0x3FFFFC00, Value: 0x08DFFC00, Op: LDAR,
		Fields: []FieldDef{{Name: "size", Hi: 31, Lo: 30}, fRn, fRd},
		Post:   postAcqRel,
	},
	// STLR: size:001000:1:0:0:11111:1:11111:Rn:Rt
	{
		Name: "STLR", Mask: 0x3FFFFC00, Value: 0x089FFC00, Op: STLR,
		Fields: []FieldDef{{Name: "size", Hi: 31, Lo: 30}, fRn, fRd},
		Post:   postAcqRel,
	},
	// LDAXR: size:001000:0:1:0:11111:1:11111:Rn:Rt
	{
		Name: "LDAXR", Mask: 0x3FFFFC00, Value: 0x085FFC00, Op: LDAXR,
		Fields: []FieldDef{{Name: "size", Hi: 31, Lo: 30}, fRn, fRd},
		Post:   postAcqRel,
	},
	// STLXR: size:001000:0:0:0:Rs:1:11111:Rn:Rt
	{
		Name: "STLXR", Mask: 0x3FE0FC00, Value: 0x0800FC00, Op: STLXR,
		Fields: []FieldDef{{Name: "size", Hi: 31, Lo: 30}, fRm16, fRn, fRd},
		Post:   postAcqRel,
	},

	// ================================================================
	// LDADD (atomic add, LSE / ARMv8.1)
	// 编码: size:111:0:00:A:R:1:Rs:0:000:00:Rn:Rt
	//   size: 10=32-bit, 11=64-bit (bit[31]=1)
	// VMP 单线程简化: old = [Rn]; [Rn] = old + Rs; Rt = old
	// ================================================================
	{
		Name: "LDADD", Mask: 0x3F20FC00, Value: 0x38200000, Op: LDADD,
		Fields: []FieldDef{{Name: "size", Hi: 31, Lo: 30}, fRm16, fRn, fRd},
		Post:   postLdadd,
	},

	// ================================================================
	// CAS (compare-and-swap, LSE / ARMv8.1)
	// 编码: size:001000:1:L:1:Rs:o0:11111:Rn:Rt
	//   size: 10=32-bit, 11=64-bit (bit[31]=1)
	//   Rs = compare value register (also destination for loaded value)
	//   Rt = new value register
	// VMP 单线程简化: old = [Rn]; if old==Rs then [Rn]=Rt; Rs = old
	// ================================================================
	{
		Name: "CAS", Mask: 0x3FA07C00, Value: 0x08A07C00, Op: CAS,
		Fields: []FieldDef{{Name: "size", Hi: 31, Lo: 30}, fRm16, fRn, fRd},
		Post:   postCas,
	},
}

// postPairSIMD LDP/STP SIMD 后处理
func postPairSIMD(f map[string]int64, inst *vm.Instruction) {
	wb := f["wb"]
	if wb != 1 && wb != 2 && wb != 3 {
		inst.Op = int(UNSUPPORTED)
		return
	}
	inst.WB = int(wb)

	opc := f["opc"]
	scale := 8
	szType := 0
	switch opc {
	case 0: // 32-bit (S)
		scale = 4
		szType = 2
		inst.SF = false
	case 1: // 64-bit (D)
		scale = 8
		szType = 3
		inst.SF = true
	case 2: // 128-bit (Q)
		scale = 16
		szType = 4
		inst.SF = true
	default:
		inst.Op = int(UNSUPPORTED)
		return
	}

	inst.Imm = f["imm7"] * int64(scale)
	inst.Shift = szType // szType stored in Shift for translator
	// Mark as SIMD registers
	inst.Rd += vm.REG_V_BASE
	inst.Rm += vm.REG_V_BASE
}

// postLdrStrSIMD LDR/STR SIMD 后处理
func postLdrStrSIMD(isLoad bool) PostFunc {
	return func(f map[string]int64, inst *vm.Instruction) {
		sz := f["size"]
		opc := f["opc"]
		// Combine size:opc to get real size
		// Manual: [size][opc]
		// 00 00/01 -> 8b (B)
		// 01 00/01 -> 16b (H)
		// 10 00/01 -> 32b (S)
		// 11 00/01 -> 64b (D)
		// 00 10/11 -> 128b (Q)
		typeScale := 1
		szType := 0
		switch {
		case sz == 0 && (opc == 0 || opc == 1):
			typeScale = 1
			szType = 0
		case sz == 1 && (opc == 0 || opc == 1):
			typeScale = 2
			szType = 1
		case sz == 2 && (opc == 0 || opc == 1):
			typeScale = 4
			szType = 2
		case sz == 3 && (opc == 0 || opc == 1):
			typeScale = 8
			szType = 3
		case sz == 0 && (opc == 2 || opc == 3):
			typeScale = 16
			szType = 4
		default:
			inst.Op = int(UNSUPPORTED)
			return
		}
		inst.Imm = f["imm12"] * int64(typeScale)
		inst.Shift = szType
		inst.Rd += vm.REG_V_BASE
		inst.SF = (typeScale >= 8)
	}
}

// postAcqRel Acquire/Release load/store 后处理
// size[31:30]: 00=1B, 01=2B, 10=4B, 11=8B → inst.Shift = access bytes
func postAcqRel(f map[string]int64, inst *vm.Instruction) {
	sz := f["size"]
	inst.Shift = 1 << int(sz) // 1,2,4,8
	inst.SF = (sz == 3)       // 64-bit if size=11
	xzrReplace(&inst.Rd)
}

// ---- Post 处理函数 ----

// postPair LDP/STP 后处理：验证寻址模式 + offset 缩放
func postPair(f map[string]int64, inst *vm.Instruction) {
	wb := f["wb"]
	if wb != 1 && wb != 2 && wb != 3 {
		inst.Op = int(UNSUPPORTED)
		return
	}
	inst.WB = int(wb)
	// sf=bit31: 1→64-bit (X regs), 0→32-bit (W regs)
	inst.SF = (f["sf"] != 0)
	if inst.SF {
		inst.Imm = f["imm7"] * 8
	} else {
		inst.Imm = f["imm7"] * 4
	}
	// Rt/Rt2 中 reg31 = XZR (STP存零/LDP丢弃), 不是SP
	xzrReplace(&inst.Rd)
	xzrReplace(&inst.Rm)
}

// postSimdMulti SIMD 多结构体 load/store
func postSimdMulti(f map[string]int64, inst *vm.Instruction) {
	switch f["opcode"] {
	case 0b0111:
		inst.Imm = 16
	case 0b1010:
		inst.Imm = 32
	case 0b0110:
		inst.Imm = 48
	case 0b0010:
		inst.Imm = 64
	default:
		inst.Op = int(UNSUPPORTED)
	}
}

// postLdrStrPrePost LDR/STR pre/post index
func postLdrStrPrePost(is64 bool) PostFunc {
	return func(f map[string]int64, inst *vm.Instruction) {
		wb := f["wb"]
		if wb != 1 && wb != 3 {
			inst.Op = int(UNSUPPORTED)
			return
		}
		inst.Imm = f["imm9"]
		inst.WB = int(wb)
		inst.SF = is64
		xzrReplace(&inst.Rd)
	}
}

// postLdrStrPrePostXZR LDRB/STRB/LDRH/STRH pre/post index
func postLdrStrPrePostXZR(f map[string]int64, inst *vm.Instruction) {
	wb := f["wb"]
	if wb != 1 && wb != 3 {
		inst.Op = int(UNSUPPORTED)
		return
	}
	inst.Imm = f["imm9"]
	inst.WB = int(wb)
	xzrReplace(&inst.Rd)
}

// postUnscaled LDUR/STUR 无缩放偏移后处理: imm9 直接使用，不缩放
func postUnscaled(sf bool) PostFunc {
	return func(f map[string]int64, inst *vm.Instruction) {
		inst.Imm = f["imm9"]
		inst.SF = sf
		xzrReplace(&inst.Rd)
	}
}

// postUnsigned unsigned offset 后处理
func postUnsigned(scale int64, sf bool) PostFunc {
	return func(f map[string]int64, inst *vm.Instruction) {
		inst.Imm = f["imm12"] * scale
		inst.SF = sf
		xzrReplace(&inst.Rd)
	}
}

// postLdrLiteral LDR literal 后处理: imm19*4 = PC-relative offset
// WB=4 标记 LDRSW (sign-extend) 变体
func postLdrLiteral(sf bool, signExtend bool) PostFunc {
	return func(f map[string]int64, inst *vm.Instruction) {
		inst.Imm = f["imm19"] * 4 // PC-relative byte offset
		inst.SF = sf
		inst.Rn = -1 // 无 base register (PC-relative)
		if signExtend {
			inst.WB = 4 // 标记 LDRSW
		}
		xzrReplace(&inst.Rd)
	}
}

// postLdpsw LDPSW 后处理: 类似 postPair 但 opc=01 固定 32-bit load
// offset 按 4 字节(32-bit word) 缩放，结果 sign-extend 到 64-bit
func postLdpsw(f map[string]int64, inst *vm.Instruction) {
	wb := f["wb"]
	if wb != 1 && wb != 2 && wb != 3 {
		inst.Op = int(UNSUPPORTED)
		return
	}
	inst.WB = int(wb)
	inst.SF = false          // 32-bit loads
	inst.Imm = f["imm7"] * 4 // scale by 4 (word)
	xzrReplace(&inst.Rd)
	xzrReplace(&inst.Rm)
}

// postLdadd LDADD 后处理: 根据 size 设置 access width
func postLdadd(f map[string]int64, inst *vm.Instruction) {
	sz := f["size"]
	inst.Shift = 1 << int(sz) // 1,2,4,8
	inst.SF = (sz == 3)
	xzrReplace(&inst.Rd)
	xzrReplace(&inst.Rm)
}

// postCas CAS 后处理: 根据 size 设置 access width
// Rm=Rs(compare reg, also written), Rd=Rt(new value reg)
func postCas(f map[string]int64, inst *vm.Instruction) {
	sz := f["size"]
	inst.Shift = 1 << int(sz) // 1,2,4,8
	inst.SF = (sz == 3)
	xzrReplace(&inst.Rd)
	xzrReplace(&inst.Rm)
}
