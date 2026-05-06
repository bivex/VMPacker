/*
 * h_branch.h — 分支指令 handler
 *
 * 所有分支指令编码 [5B: op | target32]
 * 返回 0 表示 pc 已被直接设置 (不需要外部 += adv)
 *
 * reverse 模式: fall-through 时不手动设置 pc，
 * 让 DISPATCH 自动递减定位下一条指令。
 * 条件成立时 pc = target (已被 packer 重映射到反转后偏移)。
 */
#ifndef H_BRANCH_H
#define H_BRANCH_H

#include "../vm_decode.h"
#include "../vm_types.h"

/*
 * BRANCH_FALLTHROUGH: 条件不成立时的 fall-through 处理
 * 正向: pc += 5 (跳过当前 5 字节分支指令)
 * 反向: 不改 pc, 返回 0 让 DISPATCH 自动递减
 */
#define BRANCH_FALLTHROUGH(vm) \
  do { \
    if (!(vm)->reverse) (vm)->pc += 5; \
  } while(0)

/*
 * BRANCH_TARGET_VALID: 验证分支目标在字节码范围内
 * 无效目标时 fall-through (安全降级, 不崩溃)
 */
#define BRANCH_TARGET_VALID(vm, t) ((t) < (vm)->bc_len)

/* B target (无条件跳转) */
static inline u32 h_jmp(vm_ctx_t *vm) {
  u32 t = rd32(&vm->bc[vm->pc + 1]);
  if (BRANCH_TARGET_VALID(vm, t))
    vm->pc = t;
  else
    BRANCH_FALLTHROUGH(vm); /* 无效目标: 安全 fall-through */
  return 0; /* pc 已设置 */
}

/* B.EQ target (ZF=1) */
static inline u32 h_je(vm_ctx_t *vm) {
  u32 t = rd32(&vm->bc[vm->pc + 1]);
  if ((vm->FL & FL_ZERO) && BRANCH_TARGET_VALID(vm, t)) { vm->pc = t; }
  else { BRANCH_FALLTHROUGH(vm); }
  return 0;
}

/* B.NE target (ZF=0) */
static inline u32 h_jne(vm_ctx_t *vm) {
  u32 t = rd32(&vm->bc[vm->pc + 1]);
  if (!(vm->FL & FL_ZERO) && BRANCH_TARGET_VALID(vm, t)) { vm->pc = t; }
  else { BRANCH_FALLTHROUGH(vm); }
  return 0;
}

/* B.LT target (SF=1, 有符号小于) */
static inline u32 h_jl(vm_ctx_t *vm) {
  u32 t = rd32(&vm->bc[vm->pc + 1]);
  if ((vm->FL & FL_SIGN) && BRANCH_TARGET_VALID(vm, t)) { vm->pc = t; }
  else { BRANCH_FALLTHROUGH(vm); }
  return 0;
}

/* B.GE target (SF=0, 有符号大于等于) */
static inline u32 h_jge(vm_ctx_t *vm) {
  u32 t = rd32(&vm->bc[vm->pc + 1]);
  if (!(vm->FL & FL_SIGN) && BRANCH_TARGET_VALID(vm, t)) { vm->pc = t; }
  else { BRANCH_FALLTHROUGH(vm); }
  return 0;
}

/* B.GT target (!ZF && !SF) */
static inline u32 h_jgt(vm_ctx_t *vm) {
  u32 t = rd32(&vm->bc[vm->pc + 1]);
  if (!(vm->FL & (FL_ZERO | FL_SIGN)) && BRANCH_TARGET_VALID(vm, t)) { vm->pc = t; }
  else { BRANCH_FALLTHROUGH(vm); }
  return 0;
}

/* B.LE target (ZF || SF) */
static inline u32 h_jle(vm_ctx_t *vm) {
  u32 t = rd32(&vm->bc[vm->pc + 1]);
  if ((vm->FL & (FL_ZERO | FL_SIGN)) && BRANCH_TARGET_VALID(vm, t)) { vm->pc = t; }
  else { BRANCH_FALLTHROUGH(vm); }
  return 0;
}

/* B.CC/B.LO target (!CF, 无符号小于) */
static inline u32 h_jb(vm_ctx_t *vm) {
  u32 t = rd32(&vm->bc[vm->pc + 1]);
  if (!(vm->FL & FL_CARRY) && BRANCH_TARGET_VALID(vm, t)) { vm->pc = t; }
  else { BRANCH_FALLTHROUGH(vm); }
  return 0;
}

/* B.CS/B.HS target (CF, 无符号大于等于) */
static inline u32 h_jae(vm_ctx_t *vm) {
  u32 t = rd32(&vm->bc[vm->pc + 1]);
  if ((vm->FL & FL_CARRY) && BRANCH_TARGET_VALID(vm, t)) { vm->pc = t; }
  else { BRANCH_FALLTHROUGH(vm); }
  return 0;
}

/* B.LS target (!CF || ZF, 无符号小于等于) */
static inline u32 h_jbe(vm_ctx_t *vm) {
  u32 t = rd32(&vm->bc[vm->pc + 1]);
  if ((!(vm->FL & FL_CARRY) || (vm->FL & FL_ZERO)) && BRANCH_TARGET_VALID(vm, t)) { vm->pc = t; }
  else { BRANCH_FALLTHROUGH(vm); }
  return 0;
}

/* B.HI target (CF && !ZF, 无符号大于) */
static inline u32 h_ja(vm_ctx_t *vm) {
  u32 t = rd32(&vm->bc[vm->pc + 1]);
  if ((vm->FL & FL_CARRY) && !(vm->FL & FL_ZERO) && BRANCH_TARGET_VALID(vm, t)) { vm->pc = t; }
  else { BRANCH_FALLTHROUGH(vm); }
  return 0;
}

/* TBZ Xt, #bit, target  [7B: op | reg | bit | target32]
 * 测试寄存器指定位是否为零，为零则跳转 */
static inline u32 h_tbz(vm_ctx_t *vm) {
  u8 reg = vm->bc[vm->pc + 1];
  u8 bit = vm->bc[vm->pc + 2];
  u32 t = rd32(&vm->bc[vm->pc + 3]);
  if (!(vm->R[reg & 31] & ((u64)1 << (bit & 63))) && BRANCH_TARGET_VALID(vm, t)) {
    vm->pc = t;
  } else {
    if (!(vm)->reverse) (vm)->pc += 7;
  }
  return 0;
}

/* TBNZ Xt, #bit, target  [7B: op | reg | bit | target32]
 * 测试寄存器指定位是否非零，非零则跳转 */
static inline u32 h_tbnz(vm_ctx_t *vm) {
  u8 reg = vm->bc[vm->pc + 1];
  u8 bit = vm->bc[vm->pc + 2];
  u32 t = rd32(&vm->bc[vm->pc + 3]);
  if ((vm->R[reg & 31] & ((u64)1 << (bit & 63))) && BRANCH_TARGET_VALID(vm, t)) {
    vm->pc = t;
  } else {
    if (!(vm)->reverse) (vm)->pc += 7;
  }
  return 0;
}

#endif /* H_BRANCH_H */
