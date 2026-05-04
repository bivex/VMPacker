/*
 * h_system.h — 系统/特殊指令 handler
 *
 * NOP / HALT / RET / CALL_NAT / VLD16 / VST16
 */
#ifndef H_SYSTEM_H
#define H_SYSTEM_H

#include "../vm_decode.h"
#include "../vm_types.h"

/* NOP  [1B] */
static inline u32 h_nop(vm_ctx_t *vm) {
  (void)vm;
  return 1;
}

/* CALL_NAT: BLR 绝对地址调用  [9B: op | addr64]
 * The immediate address has already been patched by RTLR to the final
 * runtime address (target_va + slide). Do NOT add slide again. */
static inline u32 h_call_nat(vm_ctx_t *vm) {
  u64 addr = rd64(&vm->bc[vm->pc + 1]); /* final runtime address from RTLR */
  {
    u8 _dbgbuf[32];
#define _HX(n) ((u8)((n) < 10 ? '0' + (n) : 'A' + (n) - 10))
    _dbgbuf[0] = 'N'; _dbgbuf[1] = 'A'; _dbgbuf[2] = 'T'; _dbgbuf[3] = ':';
    _dbgbuf[4] = _HX((addr >> 12) & 0xF);
    _dbgbuf[5] = _HX((addr >> 8) & 0xF);
    _dbgbuf[6] = _HX((addr >> 4) & 0xF);
    _dbgbuf[7] = _HX(addr & 0xF);
    _dbgbuf[8] = '\n';
#undef _HX
    sys_write(1, _dbgbuf, 9);
  }
#ifdef __aarch64__
  /* Load VM registers into temporaries */
  u64 r0 = vm->R[0], r1 = vm->R[1], r2 = vm->R[2], r3 = vm->R[3];
  u64 r4 = vm->R[4], r5 = vm->R[5], r6 = vm->R[6], r7 = vm->R[7];
  u64 result, saved_sp;
  __asm__ volatile(
    "mov %[sp_save], sp\n\t"
    "bic sp, sp, #15\n\t"          /* Align SP to 16 bytes */
    "mov x0, %[r0]\n\t"
    "mov x1, %[r1]\n\t"
    "mov x2, %[r2]\n\t"
    "mov x3, %[r3]\n\t"
    "mov x4, %[r4]\n\t"
    "mov x5, %[r5]\n\t"
    "mov x6, %[r6]\n\t"
    "mov x7, %[r7]\n\t"
    "mov x10, %[addr]\n\t"
    "blr x10\n\t"
    "mov %[result], x0\n\t"
    "mov sp, %[sp_save]\n\t"
    : [result] "=r" (result), [sp_save] "=r" (saved_sp)
    : [addr] "r" (addr),
      [r0] "r" (r0), [r1] "r" (r1), [r2] "r" (r2), [r3] "r" (r3),
      [r4] "r" (r4), [r5] "r" (r5), [r6] "r" (r6), [r7] "r" (r7)
    : "x0", "x1", "x2", "x3", "x4", "x5", "x6", "x7",
      "x10", "x30", "memory"
  );
  vm->R[0] = result;
#else
  typedef u32 (*fn32_t)(u32, u32, u32, u32);
  fn32_t fn = (fn32_t)(u32)addr;
  vm->R[0] = (u64)fn((u32)vm->R[0], (u32)vm->R[1], (u32)vm->R[2], (u32)vm->R[3]);
#endif
  return 9;
}

/* CALL_REG: BLR Xn (寄存器间接调用) [2B: op | rn] */
static inline u32 h_call_reg(vm_ctx_t *vm) {
  u8 rn = vm->bc[vm->pc + 1];
  u64 addr = vm->R[rn & 63];
#ifdef __aarch64__
  native_fn_t fn = (native_fn_t)addr;
  vm->R[0] = fn(vm->R[0], vm->R[1], vm->R[2], vm->R[3], vm->R[4], vm->R[5],
                vm->R[6], vm->R[7]);
#else
  typedef u32 (*fn32_t)(u32, u32, u32, u32);
  fn32_t fn = (fn32_t)(u32)addr;
  vm->R[0] = (u64)fn((u32)vm->R[0], (u32)vm->R[1], (u32)vm->R[2], (u32)vm->R[3]);
#endif
  return 2;
}

/* BR_REG: BR Xn (寄存器间接跳转) [2B: op | rn]
 * 两种情况:
 *   1) 目标在被保护函数内 → computed goto, 查映射表做 VM 内部跳转
 *   2) 目标在函数外 → 尾调用, 当 native call 处理
 * 返回 0 表示已直接设置 vm->pc (内部跳转) */
static inline u32 h_br_reg(vm_ctx_t *vm) {
  u8 rn = vm->bc[vm->pc + 1];
  u64 addr = vm->R[rn & 63];

  /* 检查目标是否在被保护函数的地址范围内 */
  if (vm->map_count > 0 && addr >= vm->func_addr &&
      addr < vm->func_addr + vm->func_size) {
    u32 arm64_off = (u32)(addr - vm->func_addr);
    /* 二分查找 (addr_map 已按 arm64_off 升序排序) */
    u32 lo = 0, hi = vm->map_count;
    while (lo < hi) {
      u32 mid = lo + ((hi - lo) >> 1);
      u32 mid_off = vm->addr_map[mid].arm64_off;
      if (mid_off < arm64_off)
        lo = mid + 1;
      else if (mid_off > arm64_off)
        hi = mid;
      else {
        vm->pc = vm->addr_map[mid].vm_off;
        return 0; /* 已设置 pc, 不再 advance */
      }
    }
    /* 未找到映射 */
    return 2; /* skip, 继续 */
  }

  /* 外部尾调用 → native call */
#ifdef __aarch64__
  native_fn_t fn = (native_fn_t)addr;
  vm->R[0] = fn(vm->R[0], vm->R[1], vm->R[2], vm->R[3], vm->R[4], vm->R[5],
                vm->R[6], vm->R[7]);
#else
  typedef u32 (*fn32_t)(u32, u32, u32, u32);
  fn32_t fn = (fn32_t)(u32)addr;
  vm->R[0] = (u64)fn((u32)vm->R[0], (u32)vm->R[1], (u32)vm->R[2], (u32)vm->R[3]);
#endif
  return 2;
}

/* VLD16: LD1 {Vn.16B}, [Xn]  [3B: op | rn | len] */
static inline u32 h_vld16(vm_ctx_t *vm) {
  u8 rn = vm->bc[vm->pc + 1];
  u8 len = vm->bc[vm->pc + 2];
  const u8 *src = (const u8 *)vm->R[rn & 63];
  for (int i = 0; i < len && i < VM_SIMD_BUF; i++)
    vm->vtmp[i] = src[i];
  return 3;
}

/* VST16: ST1 {Vn.16B}, [Xn]  [3B: op | rn | len] */
static inline u32 h_vst16(vm_ctx_t *vm) {
  u8 rn = vm->bc[vm->pc + 1];
  u8 len = vm->bc[vm->pc + 2];
  u8 *dst = (u8 *)vm->R[rn & 63];
  for (int i = 0; i < len && i < VM_SIMD_BUF; i++)
    dst[i] = vm->vtmp[i];
  return 3;
}

/* SVC #imm16  [3B: op | imm16_lo | imm16_hi]
 * 执行 Linux syscall: X8/R7=syscall号, X0-X5/R0-R5=参数, 结果写回 X0/R0
 * imm16 通常为 0 (Linux 只用 svc #0) */
static inline u32 h_svc(vm_ctx_t *vm) {
#ifdef __aarch64__
  /* ARM64: x8=syscall, x0-x5=args */
  register long x8 __asm__("x8") = (long)vm->R[8];
  register long x0 __asm__("x0") = (long)vm->R[0];
  register long x1 __asm__("x1") = (long)vm->R[1];
  register long x2 __asm__("x2") = (long)vm->R[2];
  register long x3 __asm__("x3") = (long)vm->R[3];
  register long x4 __asm__("x4") = (long)vm->R[4];
  register long x5 __asm__("x5") = (long)vm->R[5];
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2), "r"(x3), "r"(x4), "r"(x5) : "memory");
  vm->R[0] = (u64)x0;
#else
  /* ARM32: r7=syscall, r0-r6=args */
  register long r7 __asm__("r7") = (long)vm->R[7];
  register long r0 __asm__("r0") = (long)vm->R[0];
  register long r1 __asm__("r1") = (long)vm->R[1];
  register long r2 __asm__("r2") = (long)vm->R[2];
  register long r3 __asm__("r3") = (long)vm->R[3];
  register long r4 __asm__("r4") = (long)vm->R[4];
  register long r5 __asm__("r5") = (long)vm->R[5];
  __asm__ volatile("svc #0" : "+r"(r0) : "r"(r7), "r"(r1), "r"(r2), "r"(r3), "r"(r4), "r"(r5) : "memory");
  vm->R[0] = (u64)r0;
#endif
  return 3;
}

/* MRS Xd, <sysreg>  [4B: op | d | sysreg_lo | sysreg_hi]
 * 读取系统寄存器到 VM 虚拟寄存器。
 * sysreg 是 15-bit 编码 = bits[19:5] of the MRS instruction.
 */
static inline u32 h_mrs(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1];
  u16 sysreg = (u16)vm->bc[vm->pc + 2] | ((u16)vm->bc[vm->pc + 3] << 8);
  u64 val = 0;
#ifdef __aarch64__
  switch (sysreg) {
  case 0x3E02: /* cntvct_el0 (op1=3, CRn=14, CRm=0, op2=2) */
  case 0x5F02: __asm__ volatile("mrs %0, cntvct_el0" : "=r"(val)); break;
  case 0x3E00: /* cntfrq_el0 (op1=3, CRn=14, CRm=0, op2=0) */
  case 0x5F00: __asm__ volatile("mrs %0, cntfrq_el0" : "=r"(val)); break;
  default: break;
  }
#else
  /* ARM32: cntvct_el0/cntfrq_el0 不存在，返回 0 */
  (void)sysreg;
#endif
  vm->R[d & 63] = val;
  return 4;
}

#endif /* H_SYSTEM_H */
