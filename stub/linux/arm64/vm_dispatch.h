/*
 * vm_dispatch.h — Indirect Dispatch Jump Table
 *
 * Enabled when VM_INDIRECT_DISPATCH macro is defined.
 * Uses an absolute function pointer array, filled on the stack at runtime,
 * to break IDA cross-references through indirect handler calls.
 *
 * Core mechanism:
 *   jump_table[opcode] = (vm_handler_fn)handler
 *   handler = jump_table[opcode]; handler(vm);
 *
 * Note: Jump table must be allocated on the stack — stub is an RX-only flat binary,
 * BSS/data sections are not writable.
 */
#ifndef VM_DISPATCH_H
#define VM_DISPATCH_H

#ifdef VM_INDIRECT_DISPATCH

#include "vm_opcodes.h"
#include "vm_sections.h"
#include "vm_types.h"

/* Handler function signature: receives vm_ctx_t*, returns instruction step size in bytes */
typedef u32 (*vm_handler_fn)(vm_ctx_t *vm);

/* HALT sentinel value: handler returns this value to indicate VM exit */
#define VM_STEP_HALT 0xFFFFFFFFu

/* RET sentinel value: handler returns this value to indicate RET instruction */
#define VM_STEP_RET 0xFFFFFFFEu

/* ================================================================
 * Handler Wrapper Functions
 *
 * Existing handlers are static, the compiler may inline them.
 * Wrapper functions use noinline to ensure independent function bodies are generated,
 * making the indirect call mechanism truly effective.
 * ================================================================ */

/* ---- System ---- */
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_nop(vm_ctx_t *vm) {
  return h_nop(vm);
}

__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_halt(vm_ctx_t *vm) {
  (void)vm;
  return VM_STEP_HALT;
}

__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_ret(vm_ctx_t *vm) {
  /* Return result from R[0] per AAPCS64 */
  return VM_STEP_RET;
}

__attribute__((noinline)) VM_SECTION_SYSTEM static u32
hw_unknown(vm_ctx_t *vm) {
  (void)vm;
  return VM_STEP_HALT;
}

 /* ---- MRS ---- */
 __attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_mrs(vm_ctx_t *vm) {
   return h_mrs(vm);
 }
 
 /* ---- NATIVE_EXEC ---- */
 __attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_native_exec(vm_ctx_t *vm) {
   return h_native_exec(vm);
 }
 
 /* ---- FP ALU ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fadd(vm_ctx_t *vm) {
  return h_fadd(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fsub(vm_ctx_t *vm) {
  return h_fsub(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fmul(vm_ctx_t *vm) {
  return h_fmul(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fdiv(vm_ctx_t *vm) {
  return h_fdiv(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fmov(vm_ctx_t *vm) {
  return h_fmov(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fcmp(vm_ctx_t *vm) {
  return h_fcmp(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fneg(vm_ctx_t *vm) {
  return h_fneg(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fabs(vm_ctx_t *vm) {
  return h_fabs(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fsqrt(vm_ctx_t *vm) {
  return h_fsqrt(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fcvt_if(vm_ctx_t *vm) {
  return h_fcvt_if(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fcvt_fi(vm_ctx_t *vm) {
  return h_fcvt_fi(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fmax(vm_ctx_t *vm) {
  return h_fmax(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fmin(vm_ctx_t *vm) {
  return h_fmin(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fcvt(vm_ctx_t *vm) {
  return h_fcvt(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fmov_rv(vm_ctx_t *vm) {
  return h_fmov_rv(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_fmov_vr(vm_ctx_t *vm) {
  return h_fmov_vr(vm);
}

/* ---- Stack Machine ---- */

__attribute__((noinline)) VM_SECTION_MEM static u32 hw_mov_imm(vm_ctx_t *vm) {
  return h_mov_imm(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_mov_imm32(vm_ctx_t *vm) {
  return h_mov_imm32(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_mov_reg(vm_ctx_t *vm) {
  return h_mov_reg(vm);
}

/* ---- Memory ---- */
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_load8(vm_ctx_t *vm) {
  return h_load8(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_load32(vm_ctx_t *vm) {
  return h_load32(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_load64(vm_ctx_t *vm) {
  return h_load64(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_store8(vm_ctx_t *vm) {
  return h_store8(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_store32(vm_ctx_t *vm) {
  return h_store32(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_store64(vm_ctx_t *vm) {
  return h_store64(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_load16(vm_ctx_t *vm) {
  return h_load16(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_store16(vm_ctx_t *vm) {
  return h_store16(vm);
}

/* ---- SIMD Memory ---- */
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_vld(vm_ctx_t *vm) {
  return h_s_vld(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_vst(vm_ctx_t *vm) {
  return h_s_vst(vm);
}

/* ---- ALU Three-Register ---- */

__attribute__((noinline)) VM_SECTION_ALU static u32 hw_add(vm_ctx_t *vm) {
  return h_add(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_sub(vm_ctx_t *vm) {
  return h_sub(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_mul(vm_ctx_t *vm) {
  return h_mul(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_xor(vm_ctx_t *vm) {
  return h_xor(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_and(vm_ctx_t *vm) {
  return h_and(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_or(vm_ctx_t *vm) {
  return h_or(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_shl(vm_ctx_t *vm) {
  return h_shl(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_shr(vm_ctx_t *vm) {
  return h_shr(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_asr(vm_ctx_t *vm) {
  return h_asr(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_not(vm_ctx_t *vm) {
  return h_not(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_ror(vm_ctx_t *vm) {
  return h_ror(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_umulh(vm_ctx_t *vm) {
  return h_umulh(vm);
}

/* ---- ALU Immediate ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_add_imm(vm_ctx_t *vm) {
  return h_add_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_sub_imm(vm_ctx_t *vm) {
  return h_sub_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_xor_imm(vm_ctx_t *vm) {
  return h_xor_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_and_imm(vm_ctx_t *vm) {
  return h_and_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_or_imm(vm_ctx_t *vm) {
  return h_or_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_mul_imm(vm_ctx_t *vm) {
  return h_mul_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_shl_imm(vm_ctx_t *vm) {
  return h_shl_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_shr_imm(vm_ctx_t *vm) {
  return h_shr_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_asr_imm(vm_ctx_t *vm) {
  return h_asr_imm(vm);
}

/* ---- Comparison ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_cmp(vm_ctx_t *vm) {
  return h_cmp(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_cmp_imm(vm_ctx_t *vm) {
  return h_cmp_imm(vm);
}

/* ---- Branch (returns 0 means pc has been set) ---- */
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jmp(vm_ctx_t *vm) {
  h_jmp(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_je(vm_ctx_t *vm) {
  h_je(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jne(vm_ctx_t *vm) {
  h_jne(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jl(vm_ctx_t *vm) {
  h_jl(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jge(vm_ctx_t *vm) {
  h_jge(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jgt(vm_ctx_t *vm) {
  h_jgt(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jle(vm_ctx_t *vm) {
  h_jle(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jb(vm_ctx_t *vm) {
  h_jb(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jae(vm_ctx_t *vm) {
  h_jae(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_jbe(vm_ctx_t *vm) {
  h_jbe(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_ja(vm_ctx_t *vm) {
  h_ja(vm);
  return 0;
}

/* ---- Stack Operations ---- */
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_push(vm_ctx_t *vm) {
  return h_push(vm);
}
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_pop(vm_ctx_t *vm) {
  return h_pop(vm);
}

/* ---- Native Call ---- */
__attribute__((noinline)) VM_SECTION_SYSTEM static u32
hw_call_nat(vm_ctx_t *vm) {
  return h_call_nat(vm);
}
__attribute__((noinline)) VM_SECTION_SYSTEM static u32
hw_call_reg(vm_ctx_t *vm) {
  return h_call_reg(vm);
}
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_br_reg(vm_ctx_t *vm) {
  return h_br_reg(vm);
}

/* ---- SIMD ---- */
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_vld16(vm_ctx_t *vm) {
  return h_vld16(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_vst16(vm_ctx_t *vm) {
  return h_vst16(vm);
}

/* ---- TBZ/TBNZ (branch, returns 0) ---- */
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_tbz(vm_ctx_t *vm) {
  h_tbz(vm);
  return 0;
}
__attribute__((noinline)) VM_SECTION_BRANCH static u32 hw_tbnz(vm_ctx_t *vm) {
  h_tbnz(vm);
  return 0;
}

/* ---- CCMP/CCMN ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_ccmp_reg(vm_ctx_t *vm) {
  return h_ccmp_reg(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_ccmp_imm(vm_ctx_t *vm) {
  return h_ccmp_imm(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_ccmn_reg(vm_ctx_t *vm) {
  return h_ccmn_reg(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_ccmn_imm(vm_ctx_t *vm) {
  return h_ccmn_imm(vm);
}

/* ---- SVC ---- */
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_svc(vm_ctx_t *vm) {
  return h_svc(vm);
}

/* ---- UDIV/SDIV ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_udiv(vm_ctx_t *vm) {
  return h_udiv(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_sdiv(vm_ctx_t *vm) {
  return h_sdiv(vm);
}

/* ---- SMULH/CLZ/CLS/RBIT/REV ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_smulh(vm_ctx_t *vm) {
  return h_smulh(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_clz(vm_ctx_t *vm) {
  return h_clz(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_cls(vm_ctx_t *vm) {
  return h_cls(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_rbit(vm_ctx_t *vm) {
  return h_rbit(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_rev(vm_ctx_t *vm) {
  return h_rev(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_rev16(vm_ctx_t *vm) {
  return h_rev16(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_rev32(vm_ctx_t *vm) {
  return h_rev32(vm);
}

/* ---- ADC/SBC ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_adc(vm_ctx_t *vm) {
  return h_adc(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_sbc(vm_ctx_t *vm) {
  return h_sbc(vm);
}

/* ================================================================
 * Stack Machine Handler Wrappers
 * ================================================================ */

/* ---- Stack Transfer ---- */
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_vload(vm_ctx_t *vm) {
  return h_s_vload(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_vstore(vm_ctx_t *vm) {
  return h_s_vstore(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_vload_v(vm_ctx_t *vm) {
  return h_s_vload_v(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_vstore_v(vm_ctx_t *vm) {
  return h_s_vstore_v(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_push_imm32(vm_ctx_t *vm) {
  return h_s_push_imm32(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32
hw_s_push_imm64(vm_ctx_t *vm) {
  return h_s_push_imm64(vm);
}

/* ---- Stack Control ---- */
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_s_dup(vm_ctx_t *vm) {
  return h_s_dup(vm);
}
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_s_swap(vm_ctx_t *vm) {
  return h_s_swap(vm);
}
__attribute__((noinline)) VM_SECTION_SYSTEM static u32 hw_s_drop(vm_ctx_t *vm) {
  return h_s_drop(vm);
}

/* ---- Stack ALU Binary ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_add(vm_ctx_t *vm) {
  return h_s_add(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_sub(vm_ctx_t *vm) {
  return h_s_sub(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_mul(vm_ctx_t *vm) {
  return h_s_mul(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_xor(vm_ctx_t *vm) {
  return h_s_xor(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_and(vm_ctx_t *vm) {
  return h_s_and(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_or(vm_ctx_t *vm) {
  return h_s_or(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_shl(vm_ctx_t *vm) {
  return h_s_shl(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_shr(vm_ctx_t *vm) {
  return h_s_shr(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_asr(vm_ctx_t *vm) {
  return h_s_asr(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_ror(vm_ctx_t *vm) {
  return h_s_ror(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_umulh(vm_ctx_t *vm) {
  return h_s_umulh(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_smulh(vm_ctx_t *vm) {
  return h_s_smulh(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_udiv(vm_ctx_t *vm) {
  return h_s_udiv(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_sdiv(vm_ctx_t *vm) {
  return h_s_sdiv(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_adc(vm_ctx_t *vm) {
  return h_s_adc(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_sbc(vm_ctx_t *vm) {
  return h_s_sbc(vm);
}

/* ---- Stack ALU Unary ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_not(vm_ctx_t *vm) {
  return h_s_not(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_neg(vm_ctx_t *vm) {
  return h_s_neg(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_clz(vm_ctx_t *vm) {
  return h_s_clz(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_cls(vm_ctx_t *vm) {
  return h_s_cls(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_rbit(vm_ctx_t *vm) {
  return h_s_rbit(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_rev(vm_ctx_t *vm) {
  return h_s_rev(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_rev16(vm_ctx_t *vm) {
  return h_s_rev16(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_rev32(vm_ctx_t *vm) {
  return h_s_rev32(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_trunc32(vm_ctx_t *vm) {
  return h_s_trunc32(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_sext32(vm_ctx_t *vm) {
  return h_s_sext32(vm);
}
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_load_slide(vm_ctx_t *vm) {
  return h_s_load_slide(vm);
}

/* ---- Stack Comparison ---- */
__attribute__((noinline)) VM_SECTION_ALU static u32 hw_s_cmp(vm_ctx_t *vm) {
  return h_s_cmp(vm);
}

/* ---- Stack Memory ---- */
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_ld8(vm_ctx_t *vm) {
  return h_s_ld8(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_ld16(vm_ctx_t *vm) {
  return h_s_ld16(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_ld32(vm_ctx_t *vm) {
  return h_s_ld32(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_ld64(vm_ctx_t *vm) {
  return h_s_ld64(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_st8(vm_ctx_t *vm) {
  return h_s_st8(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_st16(vm_ctx_t *vm) {
  return h_s_st16(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_st32(vm_ctx_t *vm) {
  return h_s_st32(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_st64(vm_ctx_t *vm) {
  return h_s_st64(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_s_decrypt_str(vm_ctx_t *vm) {
  return h_s_decrypt_str(vm);
}
__attribute__((noinline)) VM_SECTION_MEM static u32 hw_snprintf(vm_ctx_t *vm) {
  return h_snprintf(vm);
}

/* ================================================================
 * Jump table runtime initialization (absolute function pointers)
 *
 * Use a loop to fill defaults to avoid GCC range initializers
 * generating implicit memset/memcpy calls under -nostdlib.
 *
 * tbl: vm_handler_fn[256] array allocated on the caller's stack
 * ================================================================ */
__attribute__((noinline)) static void vm_init_jump_table(vm_handler_fn *tbl) {
  for (int i = 0; i < OP_ID_COUNT; i++)
    tbl[i] = hw_unknown;

  /* System */
  tbl[OP_ID_NOP] = hw_nop;
  tbl[OP_ID_HALT] = hw_halt;
  tbl[OP_ID_RET] = hw_ret;

  /* Data Movement */
  tbl[OP_ID_MOVIMM] = hw_mov_imm;
  tbl[OP_ID_MOVIMM32] = hw_mov_imm32;
  tbl[OP_ID_MOVREG] = hw_mov_reg;

  /* Memory */
  tbl[OP_ID_LOAD8] = hw_load8;
  tbl[OP_ID_LOAD32] = hw_load32;
  tbl[OP_ID_LOAD64] = hw_load64;
  tbl[OP_ID_STORE8] = hw_store8;
  tbl[OP_ID_STORE32] = hw_store32;
  tbl[OP_ID_STORE64] = hw_store64;
  tbl[OP_ID_LOAD16] = hw_load16;
  tbl[OP_ID_STORE16] = hw_store16;

  /* ALU Three-Register */
  tbl[OP_ID_ADD] = hw_add;
  tbl[OP_ID_SUB] = hw_sub;
  tbl[OP_ID_MUL] = hw_mul;
  tbl[OP_ID_XOR] = hw_xor;
  tbl[OP_ID_AND] = hw_and;
  tbl[OP_ID_OR] = hw_or;
  tbl[OP_ID_SHL] = hw_shl;
  tbl[OP_ID_SHR] = hw_shr;
  tbl[OP_ID_ASR] = hw_asr;
  tbl[OP_ID_NOT] = hw_not;
  tbl[OP_ID_ROR] = hw_ror;
  tbl[OP_ID_UMULH] = hw_umulh;

  /* ALU Immediate */
  tbl[OP_ID_ADDIMM] = hw_add_imm;
  tbl[OP_ID_SUBIMM] = hw_sub_imm;
  tbl[OP_ID_XORIMM] = hw_xor_imm;
  tbl[OP_ID_ANDIMM] = hw_and_imm;
  tbl[OP_ID_ORIMM] = hw_or_imm;
  tbl[OP_ID_MULIMM] = hw_mul_imm;
  tbl[OP_ID_SHLIMM] = hw_shl_imm;
  tbl[OP_ID_SHRIMM] = hw_shr_imm;
  tbl[OP_ID_ASRIMM] = hw_asr_imm;

  /* Comparison */
  tbl[OP_ID_CMP] = hw_cmp;
  tbl[OP_ID_CMPIMM] = hw_cmp_imm;

  /* Branch */
  tbl[OP_ID_JMP] = hw_jmp;
  tbl[OP_ID_JE] = hw_je;
  tbl[OP_ID_JNE] = hw_jne;
  tbl[OP_ID_JL] = hw_jl;
  tbl[OP_ID_JGE] = hw_jge;
  tbl[OP_ID_JGT] = hw_jgt;
  tbl[OP_ID_JLE] = hw_jle;
  tbl[OP_ID_JB] = hw_jb;
  tbl[OP_ID_JAE] = hw_jae;
  tbl[OP_ID_JBE] = hw_jbe;
  tbl[OP_ID_JA] = hw_ja;

  /* Stack Operations */
  tbl[OP_ID_PUSH] = hw_push;
  tbl[OP_ID_POP] = hw_pop;

  /* Native Call */
  tbl[OP_ID_CALLNATIVE] = hw_call_nat;
  tbl[OP_ID_CALLREG] = hw_call_reg;
  tbl[OP_ID_BRREG] = hw_br_reg;

  /* SIMD */
  tbl[OP_ID_VLD16] = hw_vld16;
  tbl[OP_ID_VST16] = hw_vst16;

  /* TBZ/TBNZ */
  tbl[OP_ID_TBZ] = hw_tbz;
  tbl[OP_ID_TBNZ] = hw_tbnz;

  /* CCMP/CCMN */
  tbl[OP_ID_CCMPREG] = hw_ccmp_reg;
  tbl[OP_ID_CCMPIMM] = hw_ccmp_imm;
  tbl[OP_ID_CCMNREG] = hw_ccmn_reg;
  tbl[OP_ID_CCMNIMM] = hw_ccmn_imm;

  /* SVC */
  tbl[OP_ID_SVC] = hw_svc;

  /* UDIV/SDIV */
  tbl[OP_ID_UDIV] = hw_udiv;
  tbl[OP_ID_SDIV] = hw_sdiv;

 /* MRS */
 tbl[OP_ID_MRS] = hw_mrs;
 
 /* NATIVE_EXEC */
 tbl[OP_ID_SNATIVEEXEC] = hw_native_exec;
 
 /* SMULH/CLZ/CLS/RBIT/REV */
  tbl[OP_ID_SMULH] = hw_smulh;
  tbl[OP_ID_CLZ] = hw_clz;
  tbl[OP_ID_CLS] = hw_cls;
  tbl[OP_ID_RBIT] = hw_rbit;
  tbl[OP_ID_REV] = hw_rev;
  tbl[OP_ID_REV16] = hw_rev16;
  tbl[OP_ID_REV32] = hw_rev32;

  /* ADC/SBC */
  tbl[OP_ID_ADC] = hw_adc;
  tbl[OP_ID_SBC] = hw_sbc;

  /* ---- Stack Machine Opcodes ---- */
  tbl[OP_ID_SVLOAD] = hw_s_vload;
  tbl[OP_ID_SVSTORE] = hw_s_vstore;
  tbl[OP_ID_SVLDV] = hw_s_vload_v;
  tbl[OP_ID_SVSTV] = hw_s_vstore_v;
  tbl[OP_ID_SPUSHIMM32] = hw_s_push_imm32;
  tbl[OP_ID_SPUSHIMM64] = hw_s_push_imm64;
  tbl[OP_ID_SDUP] = hw_s_dup;
  tbl[OP_ID_SSWAP] = hw_s_swap;
  tbl[OP_ID_SDROP] = hw_s_drop;
  tbl[OP_ID_SADD] = hw_s_add;
  tbl[OP_ID_SSUB] = hw_s_sub;
  tbl[OP_ID_SMUL] = hw_s_mul;
  tbl[OP_ID_SXOR] = hw_s_xor;
  tbl[OP_ID_SAND] = hw_s_and;
  tbl[OP_ID_SOR] = hw_s_or;
  tbl[OP_ID_SSHL] = hw_s_shl;
  tbl[OP_ID_SSHR] = hw_s_shr;
  tbl[OP_ID_SASR] = hw_s_asr;
  tbl[OP_ID_SROR] = hw_s_ror;
  tbl[OP_ID_SUMULH] = hw_s_umulh;
  tbl[OP_ID_SSMULH] = hw_s_smulh;
  tbl[OP_ID_SUDIV] = hw_s_udiv;
  tbl[OP_ID_SSDIV] = hw_s_sdiv;
  tbl[OP_ID_SADC] = hw_s_adc;
  tbl[OP_ID_SSBC] = hw_s_sbc;
  tbl[OP_ID_SNOT] = hw_s_not;
  tbl[OP_ID_SNEG] = hw_s_neg;
  tbl[OP_ID_SCLZ] = hw_s_clz;
  tbl[OP_ID_SCLS] = hw_s_cls;
  tbl[OP_ID_SRBIT] = hw_s_rbit;
  tbl[OP_ID_SREV] = hw_s_rev;
  tbl[OP_ID_SREV16] = hw_s_rev16;
  tbl[OP_ID_SREV32] = hw_s_rev32;
  tbl[OP_ID_STRUNC32] = hw_s_trunc32;
  tbl[OP_ID_SSEXT32] = hw_s_sext32;
  tbl[OP_ID_SLOADSLIDE] = hw_s_load_slide;
  tbl[OP_ID_SCMP] = hw_s_cmp;
  tbl[OP_ID_SLD8] = hw_s_ld8;
  tbl[OP_ID_SLD16] = hw_s_ld16;
  tbl[OP_ID_SLD32] = hw_s_ld32;
  tbl[OP_ID_SLD64] = hw_s_ld64;
  tbl[OP_ID_SST8] = hw_s_st8;
  tbl[OP_ID_SST16] = hw_s_st16;
  tbl[OP_ID_SST32] = hw_s_st32;
  tbl[OP_ID_SST64] = hw_s_st64;

  /* ---- SIMD Memory Access ---- */
  tbl[OP_ID_SVLD] = hw_s_vld;
  tbl[OP_ID_SVST] = hw_s_vst;

  /* ---- FP ALU ---- */
  tbl[OP_ID_SFADD] = hw_fadd;
  tbl[OP_ID_SFSUB] = hw_fsub;
  tbl[OP_ID_SFMUL] = hw_fmul;
  tbl[OP_ID_SFDIV] = hw_fdiv;
  tbl[OP_ID_SFMOV] = hw_fmov;
  tbl[OP_ID_SFCMP] = hw_fcmp;
  tbl[OP_ID_SFNEG] = hw_fneg;
  tbl[OP_ID_SFABS] = hw_fabs;
  tbl[OP_ID_SFSQRT] = hw_fsqrt;
  tbl[OP_ID_SFMAX] = hw_fmax;
  tbl[OP_ID_SFMIN] = hw_fmin;
  tbl[OP_ID_SFCVTIF] = hw_fcvt_if;
  tbl[OP_ID_SFCVTFI] = hw_fcvt_fi;
  tbl[OP_ID_SFMOVRV] = hw_fmov_rv;
  tbl[OP_ID_SFMOVVR] = hw_fmov_vr;
  tbl[OP_ID_SFCVT] = hw_fcvt;
  tbl[OP_ID_SDECRYPTSTR] = hw_s_decrypt_str;
  tbl[OP_ID_SNPRINTF] = hw_snprintf;
}

#endif /* VM_INDIRECT_DISPATCH */
#endif /* VM_DISPATCH_H */
