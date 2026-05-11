#ifndef H_MOV_H
#define H_MOV_H

#include "../vm_decode.h"
#include "../vm_types.h"

/* MOV_IMM Xd, #imm16, LSL #shift */
static __attribute__((always_inline)) u32 h_mov_imm(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1];
  u64 imm = rd64(&vm->bc[vm->pc + 2]);
  VMP_REG_SET(vm, d, imm);
  return 10;
}

/* MOV_IMM32 Wd, #imm32 */
static __attribute__((always_inline)) u32 h_mov_imm32(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1];
  u32 imm = rd32(&vm->bc[vm->pc + 2]);
  VMP_REG_SET(vm, d, (u64)imm);
  return 6;
}

/* MOV Xd, Xn */
static __attribute__((always_inline)) u32 h_mov_reg(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2];
  VMP_REG_SET(vm, d, VMP_REG_GET(vm, n));
  return 3;
}

#endif /* H_MOV_H */
