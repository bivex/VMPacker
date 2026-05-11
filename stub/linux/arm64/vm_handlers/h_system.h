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
 
   /* Save LR and SP */
   register u64 x0 __asm__("x0") = VMP_REG_GET(vm, vm->reg_map[0]);
   register u64 x1 __asm__("x1") = VMP_REG_GET(vm, vm->reg_map[1]);
   register u64 x2 __asm__("x2") = VMP_REG_GET(vm, vm->reg_map[2]);
   register u64 x3 __asm__("x3") = VMP_REG_GET(vm, vm->reg_map[3]);
   register u64 x4 __asm__("x4") = VMP_REG_GET(vm, vm->reg_map[4]);
   register u64 x5 __asm__("x5") = VMP_REG_GET(vm, vm->reg_map[5]);
   register u64 x6 __asm__("x6") = VMP_REG_GET(vm, vm->reg_map[6]);
   register u64 x7 __asm__("x7") = VMP_REG_GET(vm, vm->reg_map[7]);
   register u64 x8  __asm__("x8")  = VMP_REG_GET(vm, vm->reg_map[8]);
   register u64 x9  __asm__("x9")  = 0;  /* temp */
   register u64 x10 __asm__("x10") = VMP_REG_GET(vm, vm->reg_map[10]);
   register u64 x11 __asm__("x11") = VMP_REG_GET(vm, vm->reg_map[11]);
   register u64 x12 __asm__("x12") = VMP_REG_GET(vm, vm->reg_map[12]);
   register u64 x13 __asm__("x13") = VMP_REG_GET(vm, vm->reg_map[13]);
   register u64 x14 __asm__("x14") = VMP_REG_GET(vm, vm->reg_map[14]);
   register u64 x15 __asm__("x15") = VMP_REG_GET(vm, vm->reg_map[15]);
   register u64 x16 __asm__("x16") = VMP_REG_GET(vm, vm->reg_map[16]);
   register u64 x17 __asm__("x17") = VMP_REG_GET(vm, vm->reg_map[17]);
   register u64 x18 __asm__("x18") = VMP_REG_GET(vm, vm->reg_map[18]);
   register u64 x19 __asm__("x19") = VMP_REG_GET(vm, vm->reg_map[19]);
   register u64 x20 __asm__("x20") = VMP_REG_GET(vm, vm->reg_map[20]);
   register u64 x21 __asm__("x21") = VMP_REG_GET(vm, vm->reg_map[21]);
   register u64 x22 __asm__("x22") = VMP_REG_GET(vm, vm->reg_map[22]);
   register u64 x23 __asm__("x23") = VMP_REG_GET(vm, vm->reg_map[23]);
   register u64 x24 __asm__("x24") = VMP_REG_GET(vm, vm->reg_map[24]);
   register u64 x25 __asm__("x25") = VMP_REG_GET(vm, vm->reg_map[25]);
   register u64 x26 __asm__("x26") = VMP_REG_GET(vm, vm->reg_map[26]);
   register u64 x27 __asm__("x27") = VMP_REG_GET(vm, vm->reg_map[27]);
   register u64 x28 __asm__("x28") = VMP_REG_GET(vm, vm->reg_map[28]);
   register u64 x29 __asm__("x29") = VMP_REG_GET(vm, vm->reg_map[29]); /* FP */
   register u64 lr  __asm__("x30") = VMP_REG_GET(vm, vm->reg_map[30]); /* LR */
 
   /* Align SP to 16 bytes per ABI */
   __asm__ volatile(
     "mov x9, sp\n\t"
     "bic x9, x9, #15\n\t"
     "mov sp, x9\n\t"
   );
 
    __asm__ volatile(
      "blr %[code]"
      : "+r"(x0), "+r"(x1), "+r"(x2), "+r"(x3), "+r"(x4), "+r"(x5),
        "+r"(x6), "+r"(x7), "+r"(x8), "+r"(x9), "+r"(x10), "+r"(x11),
        "+r"(x12), "+r"(x13), "+r"(x14), "+r"(x15), "+r"(x16), "+r"(x17),
        "+r"(x18), "+r"(x19), "+r"(x20), "+r"(x21), "+r"(x22), "+r"(x23),
        "+r"(x24), "+r"(x25), "+r"(x26), "+r"(x27), "+r"(x28), "+r"(x29),
         "+r"(lr)
      : [code] "r" (code)
      : "memory", "cc"
    );

    /* Capture NZCV (condition flags) from native execution and update VM FL */
    u64 nzcv;
    __asm__ volatile("mrs %0, nzcv" : "=r"(nzcv));
    vm->FL = 0;
    if (nzcv & (1ULL<<31)) vm->FL |= FL_ZERO;   // Z
    if (nzcv & (1ULL<<29)) vm->FL |= FL_CARRY;  // C
    if (nzcv & (1ULL<<30)) vm->FL |= FL_NEG;    // N
    if (nzcv & (1ULL<<28)) vm->FL |= FL_OVER;   // V

    VMP_REG_SET(vm, vm->reg_map[0],  x0);
   VMP_REG_SET(vm, vm->reg_map[1],  x1);
   VMP_REG_SET(vm, vm->reg_map[2],  x2);
   VMP_REG_SET(vm, vm->reg_map[3],  x3);
   VMP_REG_SET(vm, vm->reg_map[4],  x4);
   VMP_REG_SET(vm, vm->reg_map[5],  x5);
   VMP_REG_SET(vm, vm->reg_map[6],  x6);
   VMP_REG_SET(vm, vm->reg_map[7],  x7);
   VMP_REG_SET(vm, vm->reg_map[8],  x8);
   VMP_REG_SET(vm, vm->reg_map[9],  x9);
   VMP_REG_SET(vm, vm->reg_map[10], x10);
   VMP_REG_SET(vm, vm->reg_map[11], x11);
   VMP_REG_SET(vm, vm->reg_map[12], x12);
   VMP_REG_SET(vm, vm->reg_map[13], x13);
   VMP_REG_SET(vm, vm->reg_map[14], x14);
   VMP_REG_SET(vm, vm->reg_map[15], x15);
   VMP_REG_SET(vm, vm->reg_map[16], x16);
   VMP_REG_SET(vm, vm->reg_map[17], x17);
   VMP_REG_SET(vm, vm->reg_map[18], x18);
   VMP_REG_SET(vm, vm->reg_map[19], x19);
   VMP_REG_SET(vm, vm->reg_map[20], x20);
   VMP_REG_SET(vm, vm->reg_map[21], x21);
   VMP_REG_SET(vm, vm->reg_map[22], x22);
   VMP_REG_SET(vm, vm->reg_map[23], x23);
   VMP_REG_SET(vm, vm->reg_map[24], x24);
   VMP_REG_SET(vm, vm->reg_map[25], x25);
   VMP_REG_SET(vm, vm->reg_map[26], x26);
   VMP_REG_SET(vm, vm->reg_map[27], x27);
   VMP_REG_SET(vm, vm->reg_map[28], x28);
   VMP_REG_SET(vm, vm->reg_map[29], x29);
   VMP_REG_SET(vm, vm->reg_map[30], lr);
 
   return 4 + len;
 #else
   u16 len = rd16(&vm->bc[vm->pc + 1]);
   return 4 + len;
 #endif
 }
 
 #endif /* H_SYSTEM_H */
