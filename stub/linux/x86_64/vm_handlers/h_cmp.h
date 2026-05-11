#ifndef H_CMP_H
#define H_CMP_H

#include "../vm_decode.h"
#include "../vm_types.h"

/* CMP Xn, Xm */
static __attribute__((always_inline)) u32 h_cmp(vm_ctx_t *vm) {
  u8 n = vm->bc[vm->pc + 1], m = vm->bc[vm->pc + 2];
  u64 a = VMP_REG_GET(vm, n), b = VMP_REG_GET(vm, m);
  u64 res = a - b;
  u32 fl = 0;
  if (res == 0) fl |= FL_ZERO;
  if (a >= b) fl |= FL_CARRY;
  if (res >> 63) fl |= FL_SIGN;
  vm->FL = fl;
  return 3;
}

/* CMP_IMM Xn, #imm32 */
static __attribute__((always_inline)) u32 h_cmp_imm(vm_ctx_t *vm) {
  u8 n = vm->bc[vm->pc + 1];
  u32 imm = rd32(&vm->bc[vm->pc + 2]);
  u64 a = VMP_REG_GET(vm, n), b = (u64)imm;
  u64 res = a - b;
  u32 fl = 0;
  if (res == 0) fl |= FL_ZERO;
  if (a >= b) fl |= FL_CARRY;
  if (res >> 63) fl |= FL_SIGN;
  vm->FL = fl;
  return 6;
}

#endif /* H_CMP_H */
