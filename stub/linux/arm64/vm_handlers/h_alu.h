#ifndef H_ALU_H
#define H_ALU_H

#include "../vm_decode.h"
#include "../vm_types.h"
#include "../vm_umulh.h"

/* ---- ALU Three-Register ---- */

static inline __attribute__((always_inline)) u32 h_add(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, a) + VMP_REG_GET(vm, b));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_sub(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, a) - VMP_REG_GET(vm, b));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_mul(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, a) * VMP_REG_GET(vm, b));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_xor(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, a) ^ VMP_REG_GET(vm, b));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_and(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, a) & VMP_REG_GET(vm, b));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_or(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, a) | VMP_REG_GET(vm, b));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_shl(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, a) << (VMP_REG_GET(vm, b) & 63));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_shr(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, a) >> (VMP_REG_GET(vm, b) & 63));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_asr(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  VMP_REG_SET(vm, d, (u64)((i64)VMP_REG_GET(vm, a) >> (VMP_REG_GET(vm, b) & 63)));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_not(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2];
  VMP_REG_SET(vm, d, ~VMP_REG_GET(vm, a));
  return 3;
}

static inline __attribute__((always_inline)) u32 h_ror(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  u64 val = VMP_REG_GET(vm, a);
  u32 shift = (u32)(VMP_REG_GET(vm, b) & 63);
  if (shift == 0) VMP_REG_SET(vm, d, val);
  else VMP_REG_SET(vm, d, (val >> shift) | (val << (64 - shift)));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_umulh(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  VMP_REG_SET(vm, d, umulh64(VMP_REG_GET(vm, a), VMP_REG_GET(vm, b)));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_smulh(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  VMP_REG_SET(vm, d, smulh64(VMP_REG_GET(vm, a), VMP_REG_GET(vm, b)));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_udiv(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  u64 val_b = VMP_REG_GET(vm, b);
  VMP_REG_SET(vm, d, (val_b == 0) ? 0 : (VMP_REG_GET(vm, a) / val_b));
  return 4;
}

static inline __attribute__((always_inline)) u32 h_sdiv(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  i64 val_a = (i64)VMP_REG_GET(vm, a);
  i64 val_b = (i64)VMP_REG_GET(vm, b);
  VMP_REG_SET(vm, d, (u64)((val_b == 0) ? 0 : (val_a / val_b)));
  return 4;
}

/* ---- ALU Immediate ---- */

static inline __attribute__((always_inline)) u32 h_add_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, n) + imm);
  return 7;
}

static inline __attribute__((always_inline)) u32 h_sub_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, n) - imm);
  return 7;
}

static inline __attribute__((always_inline)) u32 h_xor_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, n) ^ imm);
  return 7;
}

static inline __attribute__((always_inline)) u32 h_and_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, n) & imm);
  return 7;
}

static inline __attribute__((always_inline)) u32 h_or_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, n) | imm);
  return 7;
}

static inline __attribute__((always_inline)) u32 h_mul_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, n) * imm);
  return 7;
}

static inline __attribute__((always_inline)) u32 h_shl_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, n) << (imm & 63));
  return 7;
}

static inline __attribute__((always_inline)) u32 h_shr_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, n) >> (imm & 63));
  return 7;
}

static inline __attribute__((always_inline)) u32 h_asr_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  VMP_REG_SET(vm, d, (u64)((i64)VMP_REG_GET(vm, n) >> (imm & 63)));
  return 7;
}

/* ---- Bit manipulation ---- */

static inline __attribute__((always_inline)) u32 h_clz(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u64 val = VMP_REG_GET(vm, n);
  VMP_REG_SET(vm, d, val ? (u64)__builtin_clzll(val) : 64);
  return 3;
}

static inline __attribute__((always_inline)) u32 h_cls(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  i64 val = (i64)VMP_REG_GET(vm, n);
  if (val < 0) val = ~val;
  VMP_REG_SET(vm, d, val ? (u64)__builtin_clzll(val) - 1 : 63);
  return 3;
}

static inline __attribute__((always_inline)) u32 h_rbit(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u64 val = VMP_REG_GET(vm, n), res = 0;
  for (int i = 0; i < 64; i++) if ((val >> i) & 1) res |= (1ULL << (63 - i));
  VMP_REG_SET(vm, d, res);
  return 3;
}

static inline __attribute__((always_inline)) u32 h_rev(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  VMP_REG_SET(vm, d, __builtin_bswap64(VMP_REG_GET(vm, n)));
  return 3;
}

static inline __attribute__((always_inline)) u32 h_rev16(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u64 val = VMP_REG_GET(vm, n);
  u64 res = ((val & 0xFF00FF00FF00FF00ULL) >> 8) | ((val & 0x00FF00FF00FF00FFULL) << 8);
  VMP_REG_SET(vm, d, res);
  return 3;
}

static inline __attribute__((always_inline)) u32 h_rev32(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u64 val = VMP_REG_GET(vm, n);
  u64 res = ((val & 0xFFFF0000FFFF0000ULL) >> 16) | ((val & 0x0000FFFF0000FFFFULL) << 16);
  VMP_REG_SET(vm, d, res);
  return 3;
}

/* ---- Carry ALU ---- */

static inline __attribute__((always_inline)) u32 h_adc(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  u64 carry = (vm->FL & FL_CARRY) ? 1 : 0;
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, a) + VMP_REG_GET(vm, b) + carry);
  return 4;
}

static inline __attribute__((always_inline)) u32 h_sbc(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  u64 carry = (vm->FL & FL_CARRY) ? 1 : 0;
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, a) - VMP_REG_GET(vm, b) - (1 - carry));
  return 4;
}

/* ---- Conditional Comparison ---- */

static inline __attribute__((always_inline)) u32 h_ccmp_reg(vm_ctx_t *vm) {
  /* [op | n | m | nzcv_cond] */
  u8 n = vm->bc[vm->pc + 1], m = vm->bc[vm->pc + 2], nzcv_cond = vm->bc[vm->pc + 3];
  (void)n; (void)m; (void)nzcv_cond; /* Simplified implementation */
  return 4;
}

static inline __attribute__((always_inline)) u32 h_ccmp_imm(vm_ctx_t *vm) {
  u8 n = vm->bc[vm->pc + 1], imm = vm->bc[vm->pc + 2], nzcv_cond = vm->bc[vm->pc + 3];
  (void)n; (void)imm; (void)nzcv_cond;
  return 4;
}

static inline __attribute__((always_inline)) u32 h_ccmn_reg(vm_ctx_t *vm) {
  u8 n = vm->bc[vm->pc + 1], m = vm->bc[vm->pc + 2], nzcv_cond = vm->bc[vm->pc + 3];
  (void)n; (void)m; (void)nzcv_cond;
  return 4;
}

static inline __attribute__((always_inline)) u32 h_ccmn_imm(vm_ctx_t *vm) {
  u8 n = vm->bc[vm->pc + 1], imm = vm->bc[vm->pc + 2], nzcv_cond = vm->bc[vm->pc + 3];
  (void)n; (void)imm; (void)nzcv_cond;
  return 4;
}

#endif /* H_ALU_H */
