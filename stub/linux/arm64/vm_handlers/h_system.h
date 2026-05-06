#ifndef H_SYSTEM_H
#define H_SYSTEM_H

#include "../vm_decode.h"
#include "../vm_types.h"

/* NOP */
static __attribute__((always_inline)) u32 h_nop(vm_ctx_t *vm) {
  (void)vm;
  return 1;
}

/* CALL_NAT: native call [9B: op | addr64] */
static __attribute__((always_inline)) u32 h_call_nat(vm_ctx_t *vm) {
  u64 addr = rd64(&vm->bc[vm->pc + 1]);
#ifdef __aarch64__
  u64 r0 = VMP_REG_GET(vm, vm->reg_map[0]), r1 = VMP_REG_GET(vm, vm->reg_map[1]), r2 = VMP_REG_GET(vm, vm->reg_map[2]), r3 = VMP_REG_GET(vm, vm->reg_map[3]);
  u64 r4 = VMP_REG_GET(vm, vm->reg_map[4]), r5 = VMP_REG_GET(vm, vm->reg_map[5]), r6 = VMP_REG_GET(vm, vm->reg_map[6]), r7 = VMP_REG_GET(vm, vm->reg_map[7]);
  u64 result, saved_sp;
  __asm__ volatile(
    "mov %[sp_save], sp\n\t"
    "mov x9, sp\n\t"
    "mov x10, #15\n\t"
    "bic x9, x9, x10\n\t"
    "mov sp, x9\n\t"
    "mov x0, %[r0]\n\t"
    "mov x1, %[r1]\n\t"
    "mov x2, %[r2]\n\t"
    "mov x3, %[r3]\n\t"
    "mov x4, %[r4]\n\t"
    "mov x5, %[r5]\n\t"
    "mov x6, %[r6]\n\t"
    "mov x7, %[r7]\n\t"
    "mov x8, #0\n\t"
    "mov x10, %[addr]\n\t"
    "blr x10\n\t"
    "mov %[result], x0\n\t"
    "mov sp, %[sp_save]\n\t"
    : [result] "=r" (result), [sp_save] "=r" (saved_sp)
    : [addr] "r" (addr),
      [r0] "r" (r0), [r1] "r" (r1), [r2] "r" (r2), [r3] "r" (r3),
      [r4] "r" (r4), [r5] "r" (r5), [r6] "r" (r6), [r7] "r" (r7)
    : "x0", "x1", "x2", "x3", "x4", "x5", "x6", "x7",
      "x8", "x9", "x10", "x11", "x12", "x13", "x14", "x15",
      "x16", "x17", "x18", "x30", "memory"
  );
  VMP_REG_SET(vm, vm->reg_map[0], result);
#else
  typedef u32 (*fn32_t)(u32, u32, u32, u32);
  fn32_t fn = (fn32_t)(u32)addr;
  VMP_REG_SET(vm, vm->reg_map[0], (u64)fn((u32)VMP_REG_GET(vm, vm->reg_map[0]), (u32)VMP_REG_GET(vm, vm->reg_map[1]), (u32)VMP_REG_GET(vm, vm->reg_map[2]), (u32)VMP_REG_GET(vm, vm->reg_map[3])));
#endif
  return 9;
}

/* CALL_REG: BLR Xn [2B: op | rn] */
static __attribute__((always_inline)) u32 h_call_reg(vm_ctx_t *vm) {
  u8 rn = vm->bc[vm->pc + 1];
  u64 addr = VMP_REG_GET(vm, rn);
#ifdef __aarch64__
  native_fn_t fn = (native_fn_t)addr;
  VMP_REG_SET(vm, vm->reg_map[0], fn(VMP_REG_GET(vm, vm->reg_map[0]), VMP_REG_GET(vm, vm->reg_map[1]), VMP_REG_GET(vm, vm->reg_map[2]), VMP_REG_GET(vm, vm->reg_map[3]), VMP_REG_GET(vm, vm->reg_map[4]), VMP_REG_GET(vm, vm->reg_map[5]),
                VMP_REG_GET(vm, vm->reg_map[6]), VMP_REG_GET(vm, vm->reg_map[7])));
#else
  typedef u32 (*fn32_t)(u32, u32, u32, u32);
  fn32_t fn = (fn32_t)addr;
  VMP_REG_SET(vm, vm->reg_map[0], (u64)fn((u32)VMP_REG_GET(vm, vm->reg_map[0]), (u32)VMP_REG_GET(vm, vm->reg_map[1]), (u32)VMP_REG_GET(vm, vm->reg_map[2]), (u32)VMP_REG_GET(vm, vm->reg_map[3])));
#endif
  return 2;
}

/* BR_REG: BR Xn [2B: op | rn] */
static __attribute__((always_inline)) u32 h_br_reg(vm_ctx_t *vm) {
  u8 rn = vm->bc[vm->pc + 1];
  u64 addr = VMP_REG_GET(vm, rn);
  u64 base = vm->func_addr + vm->slide;
  if (vm->map_count > 0 && addr >= base && addr < base + vm->func_size) {
    u32 arm64_off = (u32)(addr - base);
    u32 lo = 0, hi = vm->map_count;
    while (lo < hi) {
      u32 mid = lo + ((hi - lo) >> 1);
      u32 mid_off = rd32((const u8 *)vm->addr_map + mid * 8);
      if (mid_off < arm64_off) lo = mid + 1;
      else if (mid_off > arm64_off) hi = mid;
      else {
        vm->pc = rd32((const u8 *)vm->addr_map + mid * 8 + 4);
        return 0;
      }
    }
    return 2;
  }
#ifdef __aarch64__
  native_fn_t fn = (native_fn_t)addr;
  VMP_REG_SET(vm, vm->reg_map[0], fn(VMP_REG_GET(vm, vm->reg_map[0]), VMP_REG_GET(vm, vm->reg_map[1]), VMP_REG_GET(vm, vm->reg_map[2]), VMP_REG_GET(vm, vm->reg_map[3]), VMP_REG_GET(vm, vm->reg_map[4]), VMP_REG_GET(vm, vm->reg_map[5]),
                VMP_REG_GET(vm, vm->reg_map[6]), VMP_REG_GET(vm, vm->reg_map[7])));
#else
  typedef u32 (*fn32_t)(u32, u32, u32, u32);
  fn32_t fn = (fn32_t)addr;
  VMP_REG_SET(vm, vm->reg_map[0], (u64)fn((u32)VMP_REG_GET(vm, vm->reg_map[0]), (u32)VMP_REG_GET(vm, vm->reg_map[1]), (u32)VMP_REG_GET(vm, vm->reg_map[2]), (u32)VMP_REG_GET(vm, vm->reg_map[3])));
#endif
  return 2;
}

/* VLD16: LD1 {Vn.16B}, [Xn] */
static __attribute__((always_inline)) u32 h_vld16(vm_ctx_t *vm) {
  u8 rn = vm->bc[vm->pc + 1];
  u8 len = vm->bc[vm->pc + 2];
  const u8 *src = (const u8 *)VMP_REG_GET(vm, rn);
  for (int i = 0; i < len && i < VM_SIMD_BUF; i++)
    vm->vtmp[i] = src[i];
  return 3;
}

/* VST16: ST1 {Vn.16B}, [Xn] */
static __attribute__((always_inline)) u32 h_vst16(vm_ctx_t *vm) {
  u8 rn = vm->bc[vm->pc + 1];
  u8 len = vm->bc[vm->pc + 2];
  u8 *dst = (u8 *)VMP_REG_GET(vm, rn);
  for (int i = 0; i < len && i < VM_SIMD_BUF; i++)
    dst[i] = vm->vtmp[i];
  return 3;
}

/* SVC #imm16 */
static inline __attribute__((always_inline)) u32 h_svc(vm_ctx_t *vm) {
#ifdef __aarch64__
  register long x8 __asm__("x8") = (long)VMP_REG_GET(vm, vm->reg_map[8]);
  register long x0 __asm__("x0") = (long)VMP_REG_GET(vm, vm->reg_map[0]);
  register long x1 __asm__("x1") = (long)VMP_REG_GET(vm, vm->reg_map[1]);
  register long x2 __asm__("x2") = (long)VMP_REG_GET(vm, vm->reg_map[2]);
  register long x3 __asm__("x3") = (long)VMP_REG_GET(vm, vm->reg_map[3]);
  register long x4 __asm__("x4") = (long)VMP_REG_GET(vm, vm->reg_map[4]);
  register long x5 __asm__("x5") = (long)VMP_REG_GET(vm, vm->reg_map[5]);
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2), "r"(x3), "r"(x4), "r"(x5) : "memory");
  VMP_REG_SET(vm, vm->reg_map[0], (u64)x0);
#else
  register long r7 __asm__("r7") = (long)VMP_REG_GET(vm, vm->reg_map[7]);
  register long r0 __asm__("r0") = (long)VMP_REG_GET(vm, vm->reg_map[0]);
  register long r1 __asm__("r1") = (long)VMP_REG_GET(vm, vm->reg_map[1]);
  register long r2 __asm__("r2") = (long)VMP_REG_GET(vm, vm->reg_map[2]);
  register long r3 __asm__("r3") = (long)VMP_REG_GET(vm, vm->reg_map[3]);
  register long r4 __asm__("r4") = (long)VMP_REG_GET(vm, vm->reg_map[4]);
  register long r5 __asm__("r5") = (long)VMP_REG_GET(vm, vm->reg_map[5]);
  __asm__ volatile("svc #0" : "+r"(r0) : "r"(r7), "r"(r1), "r"(r2), "r"(r3), "r"(r4), "r"(r5) : "memory");
  VMP_REG_SET(vm, vm->reg_map[0], (u64)r0);
#endif
  return 3;
}

/* MRS Xd, <sysreg> */
static inline __attribute__((always_inline)) u32 h_mrs(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1];
  u16 sysreg = rd16(&vm->bc[vm->pc + 2]);
  u64 val = 0;
#ifdef __aarch64__
  /* Use a dummy value for now to ensure stability on all Android kernels */
  val = 0x12345678;
  (void)sysreg;
#endif
  VMP_REG_SET(vm, d, val);
  return 4;
}

#endif /* H_SYSTEM_H */
