/*
 * h_stack.h — 栈操作指令 handler
 *
 * PUSH Xn  [2B: op | n]   将 R[n] 压入操作栈
 * POP  Xn  [2B: op | n]   从操作栈弹出到 R[n]
 *
 * 溢出/下溢保护: 越界时设置 FL_CARRY 标志位作为错误指示,
 * 操作被忽略但不会导致内存越界访问。
 */
#ifndef H_STACK_H
#define H_STACK_H

#include "../vm_types.h"

/* PUSH Xn */
static __attribute__((always_inline)) u32 h_push(vm_ctx_t *vm) {
  u8 r = vm->bc[vm->pc + 1];
  if (__builtin_expect(vm->sp >= VM_STACK_SIZE, 0)) {
    vm->FL |= FL_CARRY; /* 栈溢出标志 */
    return 2;
  }
  vm->stk[vm->sp++] = vm->R[r & 63];
  return 2;
}

/* POP Xn */
static __attribute__((always_inline)) u32 h_pop(vm_ctx_t *vm) {
  u8 r = vm->bc[vm->pc + 1];
  if (__builtin_expect(vm->sp <= 0, 0)) {
    vm->FL |= FL_CARRY; /* 栈下溢标志 */
    return 2;
  }
  vm->R[r & 63] = vm->stk[--vm->sp];
  return 2;
}

#endif /* H_STACK_H */
