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
 
 /* SYSCALL */
 static inline __attribute__((always_inline)) u32 h_svc(vm_ctx_t *vm) {
 #ifdef __aarch64__
   long x8 = (long)VMP_REG_GET(vm, vm->reg_map[8]);
   long x0 = (long)VMP_REG_GET(vm, vm->reg_map[0]);
   long x1 = (long)VMP_REG_GET(vm, vm->reg_map[1]);
   long x2 = (long)VMP_REG_GET(vm, vm->reg_map[2]);
   long x3 = (long)VMP_REG_GET(vm, vm->reg_map[3]);
   long x4 = (long)VMP_REG_GET(vm, vm->reg_map[4]);
   long x5 = (long)VMP_REG_GET(vm, vm->reg_map[5]);
   __asm__ volatile("svc #0" : "+r"(x0) : "r"(x1), "r"(x2), "r"(x3), "r"(x4), "r"(x5), "r"(x8) : "memory");
   VMP_REG_SET(vm, vm->reg_map[0], (u64)x0);
 #endif
   return 3;
 }
 
  /* MRS (Special Register Access) - NOP on ARM64 for now */
  static inline __attribute__((always_inline)) u32 h_mrs(vm_ctx_t *vm) {
    u8 d = vm->bc[vm->pc + 1];
    (void)vm; (void)d;
    return 4;
  }
 
  /* NATIVE_EXEC: Execute native ARM64 code embedded in bytecode */
  static inline __attribute__((always_inline)) u32 h_native_exec(vm_ctx_t *vm) {
  #ifdef __aarch64__
    u16 len = rd16(&vm->bc[vm->pc + 1]);
    u8 *code = &vm->bc[vm->pc + 3];

    /* Allocate save area on stack (33 qwords: x0-x30 + nzcv) */
    u64 save_area[33];
    u64 *p = save_area;

    /* Save all VM registers into save_area */
    for (int i = 0; i < 31; i++) {
        p[i] = VMP_REG_GET(vm, vm->reg_map[i]);
    }
    p[31] = VMP_REG_GET(vm, vm->reg_map[30]); /* LR (x30) */
    p[32] = 0; /* NZCV placeholder */

    /* Align SP to 16 bytes */
    __asm__ volatile("mov x9, sp\n\tbic x9, x9, #15\n\tmov sp, x9\n\t" ::: "x9", "memory");

    /* Pass pointer in x9 */
    register u64 *reg_p __asm__("x9") = p;
    register u64 *reg_code __asm__("x10") = (u64*)code;

    /* Load registers from save_area, call native code, store back */
    __asm__ volatile(
        /* Load x0-x30 */
        "ldp x0, x1, [x9], #16\n\t"
        "ldp x2, x3, [x9], #16\n\t"
        "ldp x4, x5, [x9], #16\n\t"
        "ldp x6, x7, [x9], #16\n\t"
        "ldp x8, x9, [x9], #16\n\t"
        "ldp x10, x11, [x9], #16\n\t"
        "ldp x12, x13, [x9], #16\n\t"
        "ldp x14, x15, [x9], #16\n\t"
        "ldp x16, x17, [x9], #16\n\t"
        "ldp x18, x19, [x9], #16\n\t"
        "ldp x20, x21, [x9], #16\n\t"
        "ldp x22, x23, [x9], #16\n\t"
        "ldp x24, x25, [x9], #16\n\t"
        "ldp x26, x27, [x9], #16\n\t"
        "ldp x28, x29, [x9], #16\n\t"
        "ldr x30, [x9], #8\n\t"
        /* x9 points at p+264, x10 holds code pointer, need to move x10 to link register */
        "mov x11, x10\n\t"  /* temp: code -> x11 */
        "blr x11\n\t"
        /* x9 = p+264, rewind to p */
        "sub x9, x9, #264\n\t"
        /* Store back results */
        "stp x0, x1, [x9], #16\n\t"
        "stp x2, x3, [x9], #16\n\t"
        "stp x4, x5, [x9], #16\n\t"
        "stp x6, x7, [x9], #16\n\t"
        "stp x8, x9, [x9], #16\n\t"
        "stp x10, x11, [x9], #16\n\t"
        "stp x12, x13, [x9], #16\n\t"
        "stp x14, x15, [x9], #16\n\t"
        "stp x16, x17, [x9], #16\n\t"
        "stp x18, x19, [x9], #16\n\t"
        "stp x20, x21, [x9], #16\n\t"
        "stp x22, x23, [x9], #16\n\t"
        "stp x24, x25, [x9], #16\n\t"
        "stp x26, x27, [x9], #16\n\t"
        "stp x28, x29, [x9], #16\n\t"
        "str x30, [x9], #8\n\t"
        /* NZCV */
        "mrs x12, nzcv\n\t"
        "str x12, [x9], #8\n\t"
        : 
        : "r" (reg_p), "r" (reg_code)
        : "memory", "cc",
          "x0","x1","x2","x3","x4","x5","x6","x7","x8","x9","x10","x11",
          "x12","x13","x14","x15","x16","x17","x18","x19","x20","x21","x22",
          "x23","x24","x25","x26","x27","x28","x29","x30"
    );

    /* Restore VM registers from save_area */
    for (int i = 0; i < 31; i++) {
        VMP_REG_SET(vm, vm->reg_map[i], p[i]);
    }
    VMP_REG_SET(vm, vm->reg_map[30], p[31]); /* LR */
    /* Update flags from p[32] */
    u64 nzcv = p[32];
    vm->FL = 0;
    if (nzcv & (1ULL<<31)) vm->FL |= FL_ZERO;
    if (nzcv & (1ULL<<29)) vm->FL |= FL_CARRY;
    if (nzcv & (1ULL<<30)) vm->FL |= FL_NEG;
    if (nzcv & (1ULL<<28)) vm->FL |= FL_OVER;

    return 4 + len;
  #else
   u16 len = rd16(&vm->bc[vm->pc + 1]);
   return 4 + len;
 #endif
 }
 
 #endif /* H_SYSTEM_H */
