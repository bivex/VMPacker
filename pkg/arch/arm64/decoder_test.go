package arm64

import (
	"testing"

	"github.com/vmpacker/pkg/vm"
)

// Golden tests using real objdump disassembly instruction encodings

func TestDecode_STP_64bit(t *testing.T) {
	d := NewDecoder()

	// stp x29, x30, [sp, #-16]! → 0xa9bf7bfd (pre-index, 64-bit)
	inst := d.Decode(0xa9bf7bfd, 0)
	expect(t, "STP pre-index Op", int(STP), inst.Op)
	expect(t, "STP pre-index SF", true, inst.SF)
	expect(t, "STP pre-index Rd(Rt)", 29, inst.Rd)
	expect(t, "STP pre-index Rm(Rt2)", 30, inst.Rm)
	expect(t, "STP pre-index Rn", 31, inst.Rn)
	expect(t, "STP pre-index Imm", int64(-16), inst.Imm)
	expect(t, "STP pre-index WB", 3, inst.WB) // pre-index

	// stp x20, x21, [sp, #24] → 0xa901d7f4 (signed offset, 64-bit)
	inst = d.Decode(0xa901d7f4, 0)
	expect(t, "STP signed Op", int(STP), inst.Op)
	expect(t, "STP signed SF", true, inst.SF)
	expect(t, "STP signed Imm", int64(24), inst.Imm)
	expect(t, "STP signed Rd", 20, inst.Rd)
	expect(t, "STP signed Rm", 21, inst.Rm)
}

func TestDecode_LDP_64bit(t *testing.T) {
	d := NewDecoder()

	// ldp x29, x30, [sp], #16 → 0xa8c17bfd (post-index, 64-bit)
	inst := d.Decode(0xa8c17bfd, 0)
	expect(t, "LDP post Op", int(LDP), inst.Op)
	expect(t, "LDP post SF", true, inst.SF)
	expect(t, "LDP post Imm", int64(16), inst.Imm)
	expect(t, "LDP post WB", 1, inst.WB) // post-index

	// ldp x29, x30, [sp], #64 → 0xa8c47bfd
	inst = d.Decode(0xa8c47bfd, 0)
	expect(t, "LDP post64 Imm", int64(64), inst.Imm)

	// ldp x20, x21, [sp, #24] → 0xa941d7f4 (signed offset)
	inst = d.Decode(0xa941d7f4, 0)
	expect(t, "LDP signed Imm", int64(24), inst.Imm)
	expect(t, "LDP signed WB", 2, inst.WB)
}

func TestDecode_STP_64bit_BigFrame(t *testing.T) {
	d := NewDecoder()
	// stp x29, x30, [sp, #-64]! → 0xa9bc7bfd
	inst := d.Decode(0xa9bc7bfd, 0)
	expect(t, "STP big Op", int(STP), inst.Op)
	expect(t, "STP big SF", true, inst.SF)
	expect(t, "STP big Imm", int64(-64), inst.Imm)
}

func TestDecode_ADD_IMM(t *testing.T) {
	d := NewDecoder()

	// add x16, x16, #0xff8 → 0x913fe210
	inst := d.Decode(0x913fe210, 0)
	expect(t, "ADD_IMM Op", int(ADD_IMM), inst.Op)
	expect(t, "ADD_IMM SF", true, inst.SF)
	expect(t, "ADD_IMM Imm", int64(0xff8), inst.Imm)
	expect(t, "ADD_IMM Rd", 16, inst.Rd)
	expect(t, "ADD_IMM Rn", 16, inst.Rn)

	// add w1, w1, #0x1 → 0x11000421
	inst = d.Decode(0x11000421, 0)
	expect(t, "ADD_IMM 32bit SF", false, inst.SF)
	expect(t, "ADD_IMM 32bit Imm", int64(1), inst.Imm)
}

func TestDecode_SUB_REG(t *testing.T) {
	d := NewDecoder()

	// sub x1, x1, x0 → 0xcb000021
	inst := d.Decode(0xcb000021, 0)
	expect(t, "SUB_REG Op", int(SUB_REG), inst.Op)
	expect(t, "SUB_REG SF", true, inst.SF)
	expect(t, "SUB_REG Rd", 1, inst.Rd)
	expect(t, "SUB_REG Rn", 1, inst.Rn)
	expect(t, "SUB_REG Rm", 0, inst.Rm)
}

func TestDecode_SUBS_REG_CMP(t *testing.T) {
	d := NewDecoder()

	// cmp x1, x0 → 0xeb00003f (SUBS with Rd=XZR)
	inst := d.Decode(0xeb00003f, 0)
	expect(t, "CMP Op", int(SUBS_REG), inst.Op)
	expect(t, "CMP Rd", vm.REG_XZR, inst.Rd)
	expect(t, "CMP Rn", 1, inst.Rn)
	expect(t, "CMP Rm", 0, inst.Rm)
}

func TestDecode_SUBS_IMM_CMP(t *testing.T) {
	d := NewDecoder()

	// cmp w1, #0x8 → 0x7100203f
	inst := d.Decode(0x7100203f, 0)
	expect(t, "CMP_IMM Op", int(SUBS_IMM), inst.Op)
	expect(t, "CMP_IMM Rd", vm.REG_XZR, inst.Rd)
	expect(t, "CMP_IMM Imm", int64(8), inst.Imm)
}

func TestDecode_MOVZ(t *testing.T) {
	d := NewDecoder()

	// mov x29, #0x0 → 0xd280001d (MOVZ)
	inst := d.Decode(0xd280001d, 0)
	expect(t, "MOVZ Op", int(MOVZ), inst.Op)
	expect(t, "MOVZ SF", true, inst.SF)
	expect(t, "MOVZ Rd", 29, inst.Rd)
	expect(t, "MOVZ Imm", int64(0), inst.Imm)

	// mov w0, #0x1 → 0x52800020
	inst = d.Decode(0x52800020, 0)
	expect(t, "MOVZ 32bit SF", false, inst.SF)
	expect(t, "MOVZ 32bit Imm", int64(1), inst.Imm)

	// mov x4, #0x5a → 0xd2800b44
	inst = d.Decode(0xd2800b44, 0)
	expect(t, "MOVZ x4 Imm", int64(0x5a), inst.Imm)
}

func TestDecode_ORR_REG_MOV(t *testing.T) {
	d := NewDecoder()

	// mov x5, x0 → 0xaa0003e5 (ORR x5, XZR, x0)
	inst := d.Decode(0xaa0003e5, 0)
	expect(t, "MOV_REG Op", int(ORR_REG), inst.Op)
	expect(t, "MOV_REG Rd", 5, inst.Rd)
	expect(t, "MOV_REG Rn", vm.REG_XZR, inst.Rn) // XZR
	expect(t, "MOV_REG Rm", 0, inst.Rm)
}

func TestDecode_LDR_STR_unsigned(t *testing.T) {
	d := NewDecoder()

	// ldr x17, [x16, #4088] → 0xf947fe11
	inst := d.Decode(0xf947fe11, 0)
	expect(t, "LDR64 uoff Op", int(LDR_IMM), inst.Op)
	expect(t, "LDR64 uoff SF", true, inst.SF)
	expect(t, "LDR64 uoff Imm", int64(4088), inst.Imm)
	expect(t, "LDR64 uoff Rn", 16, inst.Rn)

	// str x19, [sp, #16] → 0xf9000bf3
	inst = d.Decode(0xf9000bf3, 0)
	expect(t, "STR64 uoff Op", int(STR_IMM), inst.Op)
	expect(t, "STR64 uoff Imm", int64(16), inst.Imm)
	expect(t, "STR64 uoff SF", true, inst.SF)

	// ldrb w0, [x19, #56] → 0x3940e260
	inst = d.Decode(0x3940e260, 0)
	expect(t, "LDRB uoff Op", int(LDRB_IMM), inst.Op)
	expect(t, "LDRB uoff Imm", int64(56), inst.Imm)

	// strb w0, [x19, #56] → 0x3900e260
	inst = d.Decode(0x3900e260, 0)
	expect(t, "STRB uoff Op", int(STRB_IMM), inst.Op)
	expect(t, "STRB uoff Imm", int64(56), inst.Imm)
}

func TestDecode_LDR_STR_prepost(t *testing.T) {
	d := NewDecoder()

	// ldrb w3, [x2, #1]! → 0x38401c43 (pre-index byte)
	inst := d.Decode(0x38401c43, 0)
	expect(t, "LDRB pre Op", int(LDRB_IMM), inst.Op)
	expect(t, "LDRB pre Imm", int64(1), inst.Imm)
	expect(t, "LDRB pre WB", 3, inst.WB) // pre-index

	// ldrb w3, [x1], #1 → 0x38401423 (post-index byte)
	inst = d.Decode(0x38401423, 0)
	expect(t, "LDRB post Op", int(LDRB_IMM), inst.Op)
	expect(t, "LDRB post Imm", int64(1), inst.Imm)
	expect(t, "LDRB post WB", 1, inst.WB) // post-index
}

func TestDecode_LDR_REG(t *testing.T) {
	d := NewDecoder()

	// ldr x3, [x21, x19, lsl #3] → 0xf8737aa3
	inst := d.Decode(0xf8737aa3, 0)
	expect(t, "LDR_REG Op", int(LDR_REG), inst.Op)
	expect(t, "LDR_REG SF", true, inst.SF)
	expect(t, "LDR_REG Rd", 3, inst.Rd)
	expect(t, "LDR_REG Rn", 21, inst.Rn)
	expect(t, "LDR_REG Rm", 19, inst.Rm)
}

func TestDecode_ADRP(t *testing.T) {
	d := NewDecoder()

	// adrp x16, 410000 → 0x90000090
	inst := d.Decode(0x90000090, 0)
	expect(t, "ADRP Op", int(ADRP), inst.Op)
	expect(t, "ADRP Rd", 16, inst.Rd)
}

func TestDecode_Branch(t *testing.T) {
	d := NewDecoder()

	// bl 4004f8 → 0x94000032
	inst := d.Decode(0x94000032, 0)
	expect(t, "BL Op", int(BL), inst.Op)
	expect(t, "BL Imm", int64(0x32*4), inst.Imm)

	// b 400470 → 0x17ffffdb
	inst = d.Decode(0x17ffffdb, 0)
	expect(t, "B Op", int(B), inst.Op)

	// br x17 → 0xd61f0220
	inst = d.Decode(0xd61f0220, 0)
	expect(t, "BR Op", int(BR), inst.Op)
	expect(t, "BR Rn", 17, inst.Rn)

	// blr x3 → 0xd63f0060
	inst = d.Decode(0xd63f0060, 0)
	expect(t, "BLR Op", int(BLR), inst.Op)
	expect(t, "BLR Rn", 3, inst.Rn)

	// ret → 0xd65f03c0
	inst = d.Decode(0xd65f03c0, 0)
	expect(t, "RET Op", int(RET), inst.Op)
	expect(t, "RET Rn", 30, inst.Rn)
}

func TestDecode_Bcond(t *testing.T) {
	d := NewDecoder()

	// b.eq 400534 → 0x54000080
	inst := d.Decode(0x54000080, 0)
	expect(t, "B.EQ Op", int(B_COND), inst.Op)
	expect(t, "B.EQ Cond", COND_EQ, inst.Cond)

	// b.ne 4005fc → 0x54ffff41
	inst = d.Decode(0x54ffff41, 0)
	expect(t, "B.NE Op", int(B_COND), inst.Op)
	expect(t, "B.NE Cond", COND_NE, inst.Cond)

	// b.le 4006a0 → 0x540001cd
	inst = d.Decode(0x540001cd, 0)
	expect(t, "B.LE Op", int(B_COND), inst.Op)
	expect(t, "B.LE Cond", COND_LE, inst.Cond)
}

func TestDecode_CBZ_CBNZ(t *testing.T) {
	d := NewDecoder()

	// cbz x0, 400508 → 0xb4000040
	inst := d.Decode(0xb4000040, 0)
	expect(t, "CBZ Op", int(CBZ), inst.Op)
	expect(t, "CBZ SF", true, inst.SF)
	expect(t, "CBZ Rd", 0, inst.Rd)

	// cbnz w0, 4005a4 → 0x35000080
	inst = d.Decode(0x35000080, 0)
	expect(t, "CBNZ Op", int(CBNZ), inst.Op)
	expect(t, "CBNZ SF", false, inst.SF)
}

func TestDecode_EOR_REG(t *testing.T) {
	d := NewDecoder()

	// eor x2, x0, x4 → 0xca040002
	inst := d.Decode(0xca040002, 0)
	expect(t, "EOR_REG Op", int(EOR_REG), inst.Op)
	expect(t, "EOR_REG Rd", 2, inst.Rd)
	expect(t, "EOR_REG Rn", 0, inst.Rn)
	expect(t, "EOR_REG Rm", 4, inst.Rm)
}

func TestDecode_ASR(t *testing.T) {
	d := NewDecoder()

	// asr x1, x1, #3 → 0x9343fc21 (SBFM alias)
	inst := d.Decode(0x9343fc21, 0)
	expect(t, "SBFM/ASR Op", int(SBFM), inst.Op)
	expect(t, "SBFM/ASR SF", true, inst.SF)
	expect(t, "SBFM/ASR Rd", 1, inst.Rd)
	expect(t, "SBFM/ASR Rn", 1, inst.Rn)
}

func TestDecode_LSL_UBFM(t *testing.T) {
	d := NewDecoder()

	// lsl x0, x2, #5 → 0xd37be840 (UBFM alias)
	inst := d.Decode(0xd37be840, 0)
	expect(t, "UBFM/LSL Op", int(UBFM), inst.Op)
	expect(t, "UBFM/LSL SF", true, inst.SF)
}

func TestDecode_CSINC_CSET(t *testing.T) {
	d := NewDecoder()

	// cset w2, eq → 0x1a9f17e2 (CSINC w2, WZR, WZR, ne)
	inst := d.Decode(0x1a9f17e2, 0)
	expect(t, "CSINC Op", int(CSINC), inst.Op)
	expect(t, "CSINC Rd", 2, inst.Rd)
	expect(t, "CSINC Rn", vm.REG_XZR, inst.Rn)  // WZR
	expect(t, "CSINC Rm", vm.REG_XZR, inst.Rm)  // WZR
	expect(t, "CSINC Cond", COND_NE, inst.Cond) // inverted condition
}

func TestDecode_NOP(t *testing.T) {
	d := NewDecoder()
	inst := d.Decode(0xd503201f, 0)
	expect(t, "NOP Op", int(NOP), inst.Op)
}

func TestDecode_ADD_REG(t *testing.T) {
	d := NewDecoder()

	// add x0, x0, x2 → 0x8b020000
	inst := d.Decode(0x8b020000, 0)
	expect(t, "ADD_REG Op", int(ADD_REG), inst.Op)
	expect(t, "ADD_REG SF", true, inst.SF)
	expect(t, "ADD_REG Rd", 0, inst.Rd)
	expect(t, "ADD_REG Rn", 0, inst.Rn)
	expect(t, "ADD_REG Rm", 2, inst.Rm)
}

func TestDecode_LDUR_64(t *testing.T) {
	d := NewDecoder()

	// ldur x1, [x14, #-8] → 0xF85F81C1
	// size=11, V=0, opc=01, imm9=-8(0x1F8), bits[11:10]=00, Rn=14, Rt=1
	inst := d.Decode(0xF85F81C1, 0)
	expect(t, "LDUR_64 Op", int(LDR_IMM), inst.Op)
	expect(t, "LDUR_64 SF", true, inst.SF)
	expect(t, "LDUR_64 Rn", 14, inst.Rn)
	expect(t, "LDUR_64 Rd", 1, inst.Rd)
	expect(t, "LDUR_64 Imm", int64(-8), inst.Imm)
	expect(t, "LDUR_64 WB", 0, inst.WB) // no writeback
}

func TestDecode_STUR_64(t *testing.T) {
	d := NewDecoder()

	// stur x0, [x1, #-16] → 0xF81F0020
	// size=11, V=0, opc=00, imm9=-16(0x1F0), Rn=1, Rt=0
	inst := d.Decode(0xF81F0020, 0)
	expect(t, "STUR_64 Op", int(STR_IMM), inst.Op)
	expect(t, "STUR_64 SF", true, inst.SF)
	expect(t, "STUR_64 Rn", 1, inst.Rn)
	expect(t, "STUR_64 Rd", 0, inst.Rd)
	expect(t, "STUR_64 Imm", int64(-16), inst.Imm)
	expect(t, "STUR_64 WB", 0, inst.WB)
}

func TestDecode_LDUR_32(t *testing.T) {
	d := NewDecoder()

	// ldur w2, [x3, #5] → 0xB8405062
	// size=10, V=0, opc=01, imm9=5, Rn=3, Rt=2
	inst := d.Decode(0xB8405062, 0)
	expect(t, "LDUR_32 Op", int(LDR_IMM), inst.Op)
	expect(t, "LDUR_32 SF", false, inst.SF)
	expect(t, "LDUR_32 Rn", 3, inst.Rn)
	expect(t, "LDUR_32 Rd", 2, inst.Rd)
	expect(t, "LDUR_32 Imm", int64(5), inst.Imm)
}

func TestDecode_STUR_32(t *testing.T) {
	d := NewDecoder()

	// stur w4, [x5, #-4] → 0xB81FC0A4
	// size=10, V=0, opc=00, imm9=-4(0x1FC), Rn=5, Rt=4
	inst := d.Decode(0xB81FC0A4, 0)
	expect(t, "STUR_32 Op", int(STR_IMM), inst.Op)
	expect(t, "STUR_32 SF", false, inst.SF)
	expect(t, "STUR_32 Rn", 5, inst.Rn)
	expect(t, "STUR_32 Rd", 4, inst.Rd)
	expect(t, "STUR_32 Imm", int64(-4), inst.Imm)
}

// ---- Helpers ----

func expect[T comparable](t *testing.T, name string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("%s: want %v, got %v", name, want, got)
	}
}
