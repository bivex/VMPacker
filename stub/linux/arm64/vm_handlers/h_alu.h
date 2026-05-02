/*
 * h_alu.h — 算术/逻辑指令 handler
 *
 * 分两类:
 *   三寄存器: [4B: op | d | n | m]    ADD/SUB/MUL/EOR/AND/ORR/LSL/LSR/ASR/ROR
 *   寄存器+立即数: [7B: op | d | n | imm32]
 *   双寄存器: [3B: op | d | n]         MVN
 */
#ifndef H_ALU_H
#define H_ALU_H

#include "../vm_decode.h"
#include "../vm_types.h"
#include "../vm_umulh.h"

/* ========== 三寄存器 ALU (4B) ========== */

/* ADD Xd, Xn, Xm */
static inline u32 h_add(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  vm->R[d & 63] = vm->R[a & 63] + vm->R[b & 63];
  return 4;
}

/* SUB Xd, Xn, Xm */
static inline u32 h_sub(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  vm->R[d & 63] = vm->R[a & 63] - vm->R[b & 63];
  return 4;
}

/* MUL Xd, Xn, Xm */
static inline u32 h_mul(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  vm->R[d & 63] = vm->R[a & 63] * vm->R[b & 63];
  return 4;
}

/* EOR Xd, Xn, Xm */
static inline u32 h_xor(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  vm->R[d & 63] = vm->R[a & 63] ^ vm->R[b & 63];
  return 4;
}

/* AND Xd, Xn, Xm */
static inline u32 h_and(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  vm->R[d & 63] = vm->R[a & 63] & vm->R[b & 63];
  return 4;
}

/* ORR Xd, Xn, Xm */
static inline u32 h_or(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  vm->R[d & 63] = vm->R[a & 63] | vm->R[b & 63];
  return 4;
}

/* LSL Xd, Xn, Xm */
static inline u32 h_shl(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  vm->R[d & 63] = vm->R[a & 63] << (vm->R[b & 63] & 63);
  return 4;
}

/* LSR Xd, Xn, Xm */
static inline u32 h_shr(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  vm->R[d & 63] = vm->R[a & 63] >> (vm->R[b & 63] & 63);
  return 4;
}

/* ASR Xd, Xn, Xm (算术右移, 保留符号位) */
static inline u32 h_asr(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  vm->R[d & 63] = (u64)((i64)vm->R[a & 63] >> (vm->R[b & 63] & 63));
  return 4;
}

/* MVN Xd, Xn (按位取反) */
static inline u32 h_not(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  vm->R[d & 63] = ~vm->R[n & 63];
  return 3;
}

/* ROR Xd, Xn, Xm (循环右移) */
static inline u32 h_ror(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  u64 v = vm->R[a & 63];
  u32 n = (u32)(vm->R[b & 63] & 63);
  if (n == 0) {
    vm->R[d & 63] = v;
  } else {
    vm->R[d & 63] = (v >> n) | (v << (64 - n));
  }
  return 4;
}

/* UMULH Xd, Xn, Xm (无符号高 64 位乘法) */
static inline u32 h_umulh(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  vm->R[d & 63] = umulh64(vm->R[a & 63], vm->R[b & 63]);
  return 4;
}

/* ========== 寄存器 + 立即数 ALU (7B) ========== */

/* ADD Xd, Xn, #imm32 */
static inline u32 h_add_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  vm->R[d & 63] = vm->R[n & 63] + (u64)imm;
  return 7;
}

/* SUB Xd, Xn, #imm32 */
static inline u32 h_sub_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  vm->R[d & 63] = vm->R[n & 63] - (u64)imm;
  return 7;
}

/* EOR Xd, Xn, #imm32 */
static inline u32 h_xor_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  vm->R[d & 63] = vm->R[n & 63] ^ (u64)imm;
  return 7;
}

/* AND Xd, Xn, #imm32 */
static inline u32 h_and_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  vm->R[d & 63] = vm->R[n & 63] & (u64)imm;
  return 7;
}

/* ORR Xd, Xn, #imm32 */
static inline u32 h_or_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  vm->R[d & 63] = vm->R[n & 63] | (u64)imm;
  return 7;
}

/* MUL Xd, Xn, #imm32 */
static inline u32 h_mul_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  vm->R[d & 63] = vm->R[n & 63] * (u64)imm;
  return 7;
}

/* LSL Xd, Xn, #imm32 */
static inline u32 h_shl_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  vm->R[d & 63] = vm->R[n & 63] << (imm & 63);
  return 7;
}

/* LSR Xd, Xn, #imm32 */
static inline u32 h_shr_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  vm->R[d & 63] = vm->R[n & 63] >> (imm & 63);
  return 7;
}

/* ASR Xd, Xn, #imm32 */
static inline u32 h_asr_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u32 imm = rd32(&vm->bc[vm->pc + 3]);
  vm->R[d & 63] = (u64)((i64)vm->R[n & 63] >> (imm & 63));
  return 7;
}

/* UDIV Xd, Xn, Xm (无符号除法, 除零返回0 — ARM64 规范) */
static inline u32 h_udiv(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  u64 divisor = vm->R[b & 63];
  vm->R[d & 63] = (divisor == 0) ? 0 : UDIV64(vm->R[a & 63], divisor);
  return 4;
}

/* SDIV Xd, Xn, Xm (有符号除法, 除零返回0 — ARM64 规范) */
static inline u32 h_sdiv(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  i64 divisor = (i64)vm->R[b & 63];
  vm->R[d & 63] = (divisor == 0) ? 0 : SDIV64(vm->R[a & 63], vm->R[b & 63]);
  return 4;
}

/* SMULH Xd, Xn, Xm (有符号高 64 位乘法) */
static inline u32 h_smulh(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], a = vm->bc[vm->pc + 2], b = vm->bc[vm->pc + 3];
  vm->R[d & 63] = smulh64(vm->R[a & 63], vm->R[b & 63]);
  return 4;
}

/* CLZ Xd, Xn (前导零计数) */
static inline u32 h_clz(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u64 v = vm->R[n & 63];
  vm->R[d & 63] = (v == 0) ? 64 : (u64)__builtin_clzll(v);
  return 3;
}

/* CLS Xd, Xn (前导符号位计数) */
static inline u32 h_cls(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u64 v = vm->R[n & 63];
  if (v == 0 || v == ~(u64)0) {
    vm->R[d & 63] = 63;
  } else {
    /* CLS = CLZ(x ^ (x << 1)) - 1, or equivalently CLZ(x XOR (x ASR 1)) */
    u64 x = v ^ (v >> 1);
    vm->R[d & 63] = (u64)__builtin_clzll(x) - 1;
  }
  return 3;
}

/* RBIT Xd, Xn (位反转) */
static inline u32 h_rbit(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u64 v = vm->R[n & 63];
  u64 r = 0;
  for (int i = 0; i < 64; i++) {
    r = (r << 1) | (v & 1);
    v >>= 1;
  }
  vm->R[d & 63] = r;
  return 3;
}

/* REV Xd, Xn (字节序反转, 64-bit) */
static inline u32 h_rev(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u64 v = vm->R[n & 63];
  vm->R[d & 63] = __builtin_bswap64(v);
  return 3;
}

/* REV16 Xd, Xn (半字级字节反转) */
static inline u32 h_rev16(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u64 v = vm->R[n & 63];
  u64 r = 0;
  for (int i = 0; i < 4; i++) {
    u16 hw = (u16)(v >> (i * 16));
    hw = (u16)((hw >> 8) | (hw << 8));
    r |= (u64)hw << (i * 16);
  }
  vm->R[d & 63] = r;
  return 3;
}

/* REV32 Xd, Xn (字级字节反转, 64-bit only) */
static inline u32 h_rev32(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  u64 v = vm->R[n & 63];
  u32 lo = __builtin_bswap32((u32)v);
  u32 hi = __builtin_bswap32((u32)(v >> 32));
  vm->R[d & 63] = ((u64)hi << 32) | (u64)lo;
  return 3;
}

/* ADC Xd, Xn, Xm (带进位加法) */
static inline u32 h_adc(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], m = vm->bc[vm->pc + 3];
  u64 carry = (vm->FL & FL_CARRY) ? 1 : 0;
  vm->R[d & 63] = vm->R[n & 63] + vm->R[m & 63] + carry;
  return 4;
}

/* SBC Xd, Xn, Xm (带借位减法) */
static inline u32 h_sbc(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], m = vm->bc[vm->pc + 3];
  u64 carry = (vm->FL & FL_CARRY) ? 1 : 0;
  vm->R[d & 63] = vm->R[n & 63] - vm->R[m & 63] - (1 - carry);
  return 4;
}

#endif /* H_ALU_H */
