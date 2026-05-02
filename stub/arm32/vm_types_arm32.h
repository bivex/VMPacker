/*
 * vm_types_arm32.h — ARM32-specific VM type overrides
 *
 * ARM32 uses the same vm_ctx_t structure as ARM64 (with u64 R[]),
 * but only the lower 32 bits of each register are used.
 * The translator emits OpSTrunc32 to mask results.
 */
#ifndef VM_TYPES_ARM32_H
#define VM_TYPES_ARM32_H

#include "vm_types.h"

/* ARM32 native function pointer: AAPCS uses R0-R3 for args */
typedef u32 (*native_fn_arm32_t)(u32, u32, u32, u32);

/* ARM32 register aliases in vm_ctx_t */
#define ARM32_SP 13
#define ARM32_LR 14
#define ARM32_PC 15

/* ARM32 syscall ABI: R7 = syscall number, SVC #0 / SWI #0 */
#define ARM32_NR_MMAP2   192
#define ARM32_NR_MUNMAP  91

/* ARM32 vm_ctx_init override: only use R0-R3 args, R11=FP, R14=LR */
static inline void vm_ctx_init_arm32(vm_ctx_t *vm, u32 *args, u8 *bytecode, u32 len) {
  for (int i = 0; i < VM_REG_COUNT; i++)
    vm->R[i] = 0;

  /* ARM32 AAPCS: R0-R3 are argument registers */
  for (int i = 0; i < 4; i++)
    vm->R[i] = (u64)args[i];

  /* Additional saved registers from the trampoline */
  for (int i = 4; i < 8; i++)
    vm->R[i] = (u64)args[i];

  vm->R[9]  = (u64)args[8];   /* R9 */
  vm->R[10] = (u64)args[9];   /* R10 */
  vm->R[11] = (u64)args[10];  /* R11 (FP) */
  vm->R[14] = (u64)args[11];  /* R14 (LR) */

  /* SP: point to internal VM memory stack */
  vm->R[ARM32_SP] = (u64)&vm->vm_stk[VM_MEM_STACK];

  vm->bc = bytecode;
  vm->bc_len = len;
  vm->FL = 0;
  vm->pc = 0;
  vm->sp = 0;
  vm->eval_sp = -1;
  vm->func_addr = 0;
  vm->func_size = 0;
  vm->addr_map = 0;
  vm->map_count = 0;
  vm->oc_key = 0;
  vm->reverse = 0;
}

#endif /* VM_TYPES_ARM32_H */
