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
#ifdef __x86_64__
  u64 result;
  register u64 _rdi __asm__("rdi") = VMP_REG_GET(vm, vm->reg_map[X86_RDI]);
  register u64 _rsi __asm__("rsi") = VMP_REG_GET(vm, vm->reg_map[X86_RSI]);
  register u64 _rdx __asm__("rdx") = VMP_REG_GET(vm, vm->reg_map[X86_RDX]);
  register u64 _rcx __asm__("rcx") = VMP_REG_GET(vm, vm->reg_map[X86_RCX]);
  register u64 _r8  __asm__("r8")  = VMP_REG_GET(vm, vm->reg_map[X86_R8]);
  register u64 _r9  __asm__("r9")  = VMP_REG_GET(vm, vm->reg_map[X86_R9]);
  
  __asm__ volatile(
    "xor %%eax, %%eax\n\t"
    "call *%[addr]\n\t"
    : "=a" (result)
    : [addr] "r" (addr), "r"(_rdi), "r"(_rsi), "r"(_rdx), "r"(_rcx), "r"(_r8), "r"(_r9)
    : "r10", "r11", "memory"
  );
  VMP_REG_SET(vm, vm->reg_map[X86_RAX], result);
#else
  (void)addr;
#endif
  return 9;
}

/* CALL_REG: CALL register [2B: op | rn] */
static __attribute__((always_inline)) u32 h_call_reg(vm_ctx_t *vm) {
  u8 rn = vm->bc[vm->pc + 1];
  u64 addr = VMP_REG_GET(vm, rn);
#ifdef __x86_64__
  native_fn_t fn = (native_fn_t)addr;
  VMP_REG_SET(vm, vm->reg_map[X86_RAX], fn(VMP_REG_GET(vm, vm->reg_map[X86_RDI]), VMP_REG_GET(vm, vm->reg_map[X86_RSI]), 
                VMP_REG_GET(vm, vm->reg_map[X86_RDX]), VMP_REG_GET(vm, vm->reg_map[X86_RCX]), 
                VMP_REG_GET(vm, vm->reg_map[X86_R8]), VMP_REG_GET(vm, vm->reg_map[X86_R9]),
                0, 0));
#endif
  return 2;
}

/* BR_REG: JMP register [2B: op | rn] */
static __attribute__((always_inline)) u32 h_br_reg(vm_ctx_t *vm) {
  u8 rn = vm->bc[vm->pc + 1];
  u64 addr = VMP_REG_GET(vm, rn);
  u64 base = vm->func_addr + vm->slide;
  if (vm->map_count > 0 && addr >= base && addr < base + vm->func_size) {
    u32 off = (u32)(addr - base);
    u32 lo = 0, hi = vm->map_count;
    while (lo < hi) {
      u32 mid = lo + ((hi - lo) >> 1);
      u32 mid_off = rd32((const u8 *)vm->addr_map + mid * 8);
      if (mid_off < off) lo = mid + 1;
      else if (mid_off > off) hi = mid;
      else {
        vm->pc = rd32((const u8 *)vm->addr_map + mid * 8 + 4);
        return 0;
      }
    }
    return 2;
  }
#ifdef __x86_64__
  native_fn_t fn = (native_fn_t)addr;
  VMP_REG_SET(vm, vm->reg_map[X86_RAX], fn(VMP_REG_GET(vm, vm->reg_map[X86_RDI]), VMP_REG_GET(vm, vm->reg_map[X86_RSI]), 
                VMP_REG_GET(vm, vm->reg_map[X86_RDX]), VMP_REG_GET(vm, vm->reg_map[X86_RCX]), 
                VMP_REG_GET(vm, vm->reg_map[X86_R8]), VMP_REG_GET(vm, vm->reg_map[X86_R9]),
                0, 0));
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
#ifdef __x86_64__
  long rax = (long)VMP_REG_GET(vm, vm->reg_map[X86_RAX]);
  long rdi = (long)VMP_REG_GET(vm, vm->reg_map[X86_RDI]);
  long rsi = (long)VMP_REG_GET(vm, vm->reg_map[X86_RSI]);
  long rdx = (long)VMP_REG_GET(vm, vm->reg_map[X86_RDX]);
  long r10 = (long)VMP_REG_GET(vm, vm->reg_map[X86_R10]);
  long r8 = (long)VMP_REG_GET(vm, vm->reg_map[X86_R8]);
  long r9 = (long)VMP_REG_GET(vm, vm->reg_map[X86_R9]);
  __asm__ volatile("syscall" : "+a"(rax) : "D"(rdi), "S"(rsi), "d"(rdx), "r"(r10), "r"(r8), "r"(r9) : "rcx", "r11", "memory"
  );
  VMP_REG_SET(vm, vm->reg_map[X86_RAX], (u64)rax);
#endif
  return 3;
}

 /* MRS (Special Register Access) - NOP on x86_64 for now */
 static inline __attribute__((always_inline)) u32 h_mrs(vm_ctx_t *vm) {
   u8 d = vm->bc[vm->pc + 1];
   (void)vm; (void)d;
   return 4;
 }

  /* NATIVE_EXEC: Execute native x86_64 code embedded in bytecode.
     Saves RAX,RBX,RCX,RDX,RSI,RDI,R8,R9 + RFLAGS via stack-based buf.
     */
   static inline __attribute__((always_inline)) u32 h_native_exec(vm_ctx_t *vm) {
     u16 len = rd16(&vm->bc[vm->pc + 1]);
     u8 *code = &vm->bc[vm->pc + 3];

     u64 buf[10];
     buf[0] = VMP_REG_GET(vm, vm->reg_map[X86_RAX]);
     buf[1] = VMP_REG_GET(vm, vm->reg_map[X86_RBX]);
     buf[2] = VMP_REG_GET(vm, vm->reg_map[X86_RCX]);
     buf[3] = VMP_REG_GET(vm, vm->reg_map[X86_RDX]);
     buf[4] = VMP_REG_GET(vm, vm->reg_map[X86_RSI]);
     buf[5] = VMP_REG_GET(vm, vm->reg_map[X86_RDI]);
     buf[6] = VMP_REG_GET(vm, vm->reg_map[X86_R8]);
     buf[7] = VMP_REG_GET(vm, vm->reg_map[X86_R9]);
     buf[8] = 0; /* RFLAGS */

     const u8 *code_ptr = code;

     __asm__ volatile(
       "push %[b]\n\t"
       "push %%rax\n\t"
       "mov 0(%[b]),  %%rax\n\t"
       "mov 8(%[b]),  %%rbx\n\t"
       "mov 16(%[b]), %%rcx\n\t"
       "mov 24(%[b]), %%rdx\n\t"
       "mov 32(%[b]), %%rsi\n\t"
       "mov 40(%[b]), %%rdi\n\t"
       "mov 48(%[b]), %%r8\n\t"
       "mov 56(%[b]), %%r9\n\t"
       "call *%[c]\n\t"
       "pop %%r11\n\t"
       "pop %%r11\n\t"
       "mov %%rax, 0(%%r11)\n\t"
       "mov %%rbx, 8(%%r11)\n\t"
       "mov %%rcx, 16(%%r11)\n\t"
       "mov %%rdx, 24(%%r11)\n\t"
       "mov %%rsi, 32(%%r11)\n\t"
       "mov %%rdi, 40(%%r11)\n\t"
       "mov %%r8,  48(%%r11)\n\t"
       "mov %%r9,  56(%%r11)\n\t"
       "pushfq\n\t"
       "pop %%rax\n\t"
       "mov %%rax, 64(%%r11)\n\t"
       : /* no outputs */
       : [b] "r" (buf), [c] "r" (code_ptr)
       : "memory", "cc", "rax", "rbx", "rcx", "rdx", "rsi", "rdi",
         "r8", "r9", "r10", "r11"
     );

     VMP_REG_SET(vm, vm->reg_map[X86_RAX], buf[0]);
     VMP_REG_SET(vm, vm->reg_map[X86_RBX], buf[1]);
     VMP_REG_SET(vm, vm->reg_map[X86_RCX], buf[2]);
     VMP_REG_SET(vm, vm->reg_map[X86_RDX], buf[3]);
     VMP_REG_SET(vm, vm->reg_map[X86_RSI], buf[4]);
     VMP_REG_SET(vm, vm->reg_map[X86_RDI], buf[5]);
     VMP_REG_SET(vm, vm->reg_map[X86_R8],  buf[6]);
     VMP_REG_SET(vm, vm->reg_map[X86_R9],  buf[7]);

     u64 rflags = buf[8];
     vm->FL = 0;
     if (rflags & (1ULL<<6))  vm->FL |= FL_ZERO;
     if (rflags & (1ULL<<0))  vm->FL |= FL_CARRY;
     if (rflags & (1ULL<<7))  vm->FL |= FL_NEG;
     if (rflags & (1ULL<<11)) vm->FL |= FL_OVER;

     return 4 + len;
   }

 #endif /* H_SYSTEM_H */
