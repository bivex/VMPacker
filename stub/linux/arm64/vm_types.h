#ifndef VM_TYPES_H
#define VM_TYPES_H

#include "vm_opcodes.h"

typedef unsigned long long u64;
typedef unsigned int u32;
typedef unsigned short u16;
typedef unsigned char u8;
typedef long long i64;
typedef int i32;
typedef short i16;

#define VM_REG_COUNT 32
#define VM_STACK_SIZE 256
#define VM_EVAL_STACK_SIZE 256
#define VM_BYTECODE_MAX (1024 * 1024)
#define VM_MEM_STACK 16384
#define VM_SIMD_BUF 128

typedef struct vm_ctx_s vm_ctx_t;
typedef u32 (*vm_handler_fn)(vm_ctx_t *);
typedef u64 (*native_fn_t)(u64, u64, u64, u64, u64, u64, u64, u64);

struct vm_ctx_s {
  /* Group 1: 16-byte aligned arrays */
  u8 vm_stk[VM_MEM_STACK] __attribute__((aligned(16)));
  u64 R[32] __attribute__((aligned(16)));
  u64 V[32][2] __attribute__((aligned(16)));
  u64 stk[VM_STACK_SIZE] __attribute__((aligned(16)));
  u64 eval_stk[VM_EVAL_STACK_SIZE] __attribute__((aligned(16)));
  u8 string_pool[4096] __attribute__((aligned(16)));
  u8 vtmp[VM_SIMD_BUF] __attribute__((aligned(16)));

  /* Group 2: 8-byte pointers/values */
  u8 *bc;
  u8 *addr_map;
  u64 slide;
  u64 func_addr;

  /* Group 3: 4-byte values */
  u32 FL;
  u32 pc;
  u32 sp;
  u32 eval_sp;
  u32 bc_len;
  u32 str_ptr;
  u32 func_size;
  u32 map_count;
  u32 expected_bc_crc;
  u32 bc_crc_len;
  u32 insn_count;
  u32 oc_key;

  /* Group 4: 1-byte values */
  u8 ret_reg;
  u8 reverse;
  u8 debug;
  u8 reg_map[32];
} __attribute__((aligned(16)));

#define FL_ZERO  (1 << 0)
#define FL_CARRY (1 << 1)
#define FL_NEG   (1 << 2)
#define FL_OVER  (1 << 3)
#define FL_SIGN  (1 << 4)

#define VM_STEP_HALT 0xFFFFFFFF
#define VM_STEP_RET  0xFFFFFFFE

#define VM_STK_LO(vm) ((u64)(vm)->vm_stk)
#define VM_STK_HI(vm) ((u64)(vm)->vm_stk + VM_MEM_STACK)

static void vm_ctx_init(vm_ctx_t *vm, u64 *args, u8 *bytecode, u32 len) {
  for (int i = 0; i < 32; i++) {
    vm->R[i] = 0;
    vm->V[i][0] = 0;
    vm->V[i][1] = 0;
  }
  vm->FL = 0; vm->pc = 0; vm->sp = 0; vm->eval_sp = 0;
  vm->bc = bytecode; vm->bc_len = len;
  vm->slide = 0; vm->str_ptr = 0;
  vm->insn_count = 0;

  for (int i = 0; i < 8; i++) vm->R[vm->reg_map[i]] = args[i];
  vm->R[vm->reg_map[29]] = args[8];
  vm->R[vm->reg_map[30]] = args[9];
  for (int i = 0; i < 8; i++) {
    vm->V[i][0] = args[10 + i * 2];
    vm->V[i][1] = args[10 + i * 2 + 1];
  }
  for (int i = 0; i < 10; i++) vm->R[vm->reg_map[19 + i]] = args[26 + i];
  vm->R[vm->reg_map[18]] = args[36];
  vm->R[31] = (u64)&vm->vm_stk[VM_MEM_STACK];
}

/* ---- Safe Register Access Macros ---- */
#define VMP_REG_GET(vm, r) (((r) == 63) ? 0 : (vm)->R[(r) & 31])
#define VMP_REG_SET(vm, r, val) do { if ((r) != 63) (vm)->R[(r) & 31] = (val); } while(0)

#define VM_DEBUG(...)

#endif /* VM_TYPES_H */
