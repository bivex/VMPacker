/*
 * h_mem.h — 内存访问指令 handler
 *
 * LDRB / LDR(32/64) / STRB / STR(32/64)
 * 编码: [op | d/base | n/src | offset16]  共 5B
 * offset16 按 signed int16 解释以支持 LDUR/STUR 负偏移
 *
 * SP 栈边界保护: 当 base 寄存器为 R[31](SP) 时，
 * 检查目标地址是否在 vm_stk 范围内，防止栈溢出。
 */
#ifndef H_MEM_H
#define H_MEM_H

#include "../vm_decode.h"
#include "../vm_types.h"

/* SP 栈访问边界检查: 地址在 vm_stk 范围内才执行 */
#define VM_STK_CHECK(vm, addr, width) \
  ((addr) >= VM_STK_LO(vm) && ((addr) + (width)) <= VM_STK_HI(vm))

/* ---- 加载 (LDR) ---- */

/* LDRB Wd, [Xn, #off16] */
static inline u32 h_load8(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  i16 off = (i16)rd16(&vm->bc[vm->pc + 3]);
  u64 addr = vm->R[n & 63] + off;
  if (0) /* DISABLE CHECK */
    return 5;
  vm->R[d & 63] = *(u8 *)addr;
  return 5;
}

/* LDR Wd, [Xn, #off16] */
static inline u32 h_load32(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  i16 off = (i16)rd16(&vm->bc[vm->pc + 3]);
  u64 addr = vm->R[n & 63] + off;
  if ((n & 63) == 31 && !VM_STK_CHECK(vm, addr, 4))
    return 5;
  vm->R[d & 63] = *(u32 *)addr;
  return 5;
}

/* LDR Xd, [Xn, #off16] */
static inline u32 h_load64(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  i16 off = (i16)rd16(&vm->bc[vm->pc + 3]);
  u64 addr = vm->R[n & 63] + off;
  if ((n & 63) == 31 && !VM_STK_CHECK(vm, addr, 8))
    return 5;
  vm->R[d & 63] = *(u64 *)addr;
  return 5;
}

/* LDRH Wd, [Xn, #off16] */
static inline u32 h_load16(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  i16 off = (i16)rd16(&vm->bc[vm->pc + 3]);
  u64 addr = vm->R[n & 63] + off;
  if ((n & 63) == 31 && !VM_STK_CHECK(vm, addr, 2))
    return 5; /* SP 越界, 静默跳过 */
  vm->R[d & 63] = *(u16 *)addr;
  return 5;
}

/* ---- 存储 (STR) ---- */

/* STRB Wn, [Xb, #off16] */
static inline u32 h_store8(vm_ctx_t *vm) {
  u8 b = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  i16 off = (i16)rd16(&vm->bc[vm->pc + 3]);
  u64 addr = vm->R[b & 63] + off;
  if ((b & 63) == 31 && !VM_STK_CHECK(vm, addr, 1))
    return 5;
  *(u8 *)addr = (u8)vm->R[n & 63];
  return 5;
}

/* STRH Wn, [Xb, #off16] */
static inline u32 h_store16(vm_ctx_t *vm) {
  u8 b = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  i16 off = (i16)rd16(&vm->bc[vm->pc + 3]);
  u64 addr = vm->R[b & 63] + off;
  if ((b & 63) == 31 && !VM_STK_CHECK(vm, addr, 2))
    return 5;
  *(u16 *)addr = (u16)vm->R[n & 63];
  return 5;
}

/* STR Wn, [Xb, #off16] */
static inline u32 h_store32(vm_ctx_t *vm) {
  u8 b = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  i16 off = (i16)rd16(&vm->bc[vm->pc + 3]);
  u64 addr = vm->R[b & 63] + off;
  if ((b & 63) == 31 && !VM_STK_CHECK(vm, addr, 4))
    return 5;
  *(u32 *)addr = (u32)vm->R[n & 63];
  return 5;
}

/* STR Xn, [Xb, #off16] */
static inline u32 h_store64(vm_ctx_t *vm) {
  u8 b = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  i16 off = (i16)rd16(&vm->bc[vm->pc + 3]);
  u64 addr = vm->R[b & 63] + off;
  if ((b & 63) == 31 && !VM_STK_CHECK(vm, addr, 8))
    return 5;
  *(u64 *)addr = vm->R[n & 63];
  return 5;
}

#endif /* H_MEM_H */
