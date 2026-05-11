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
  __asm__ volatile("syscall" : "+a"(rax) : "D"(rdi), "S"(rsi), "d"(rdx), "r"(r10), "r"(r8), "r"(r9) : "rcx", "r11", "memory");
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

 /* NATIVE_EXEC: Execute native x86_64 code embedded in bytecode */
 static inline __attribute__((always_inline)) u32 h_native_exec(vm_ctx_t *vm) {
   u16 len = rd16(&vm->bc[vm->pc + 1]);
   u8 *code = &vm->bc[vm->pc + 3];

   /* Load VM registers into hardware registers.
      We map VM register indices directly to x86_64 architectural registers.
      RSP (index 4) is intentionally skipped. */
   register u64 _rax __asm__("rax") = VMP_REG_GET(vm, vm->reg_map[X86_RAX]);
   register u64 _rcx __asm__("rcx") = VMP_REG_GET(vm, vm->reg_map[X86_RCX]);
   register u64 _rdx __asm__("rdx") = VMP_REG_GET(vm, vm->reg_map[X86_RDX]);
   register u64 _rbx __asm__("rbx") = VMP_REG_GET(vm, vm->reg_map[X86_RBX]);
   register u64 _rbp __asm__("rbp") = VMP_REG_GET(vm, vm->reg_map[X86_RBP]);
   register u64 _rsi __asm__("rsi") = VMP_REG_GET(vm, vm->reg_map[X86_RSI]);
   register u64 _rdi __asm__("rdi") = VMP_REG_GET(vm, vm->reg_map[X86_RDI]);
   register u64 _r8  __asm__("r8")  = VMP_REG_GET(vm, vm->reg_map[X86_R8]);
   register u64 _r9  __asm__("r9")  = VMP_REG_GET(vm, vm->reg_map[X86_R9]);
   register u64 _r10 __asm__("r10") = VMP_REG_GET(vm, vm->reg_map[X86_R10]);
   register u64 _r11 __asm__("r11") = VMP_REG_GET(vm, vm->reg_map[X86_R11]);
   register u64 _r12 __asm__("r12") = VMP_REG_GET(vm, vm->reg_map[X86_R12]);
   register u64 _r13 __asm__("r13") = VMP_REG_GET(vm, vm->reg_map[X86_R13]);
   register u64 _r14 __asm__("r14") = VMP_REG_GET(vm, vm->reg_map[X86_R14]);
   register u64 _r15 __asm__("r15") = VMP_REG_GET(vm, vm->reg_map[X86_R15]);

    __asm__ volatile(
      "call *%[code]"
      : "+r"(_rax), "+r"(_rcx), "+r"(_rdx), "+r"(_rbx), "+r"(_rbp),
        "+r"(_rsi), "+r"(_rdi), "+r"(_r8),  "+r"(_r9),  "+r"(_r10),
        "+r"(_r11), "+r"(_r12), "+r"(_r13), "+r"(_r14), "+r"(_r15)
      : [code] "r" (code)
      : "memory", "cc"
    );

    /* Capture RFLAGS from native execution and update VM condition flags */
    u64 rflags;
    __asm__ volatile("pushfq\n\tpop %0" : "=r"(rflags));
    vm->FL = 0;
    if (rflags & (1ULL<<6))  vm->FL |= FL_ZERO;   // ZF
    if (rflags & (1ULL<<0))  vm->FL |= FL_CARRY;  // CF
    if (rflags & (1ULL<<7))  vm->FL |= FL_NEG;    // SF
    if (rflags & (1ULL<<11)) vm->FL |= FL_OVER;   // OF

    VMP_REG_SET(vm, vm->reg_map[X86_RAX], _rax);
   VMP_REG_SET(vm, vm->reg_map[X86_RCX], _rcx);
   VMP_REG_SET(vm, vm->reg_map[X86_RDX], _rdx);
   VMP_REG_SET(vm, vm->reg_map[X86_RBX], _rbx);
   VMP_REG_SET(vm, vm->reg_map[X86_RBP], _rbp);
   VMP_REG_SET(vm, vm->reg_map[X86_RSI], _rsi);
   VMP_REG_SET(vm, vm->reg_map[X86_RDI], _rdi);
   VMP_REG_SET(vm, vm->reg_map[X86_R8],  _r8);
   VMP_REG_SET(vm, vm->reg_map[X86_R9],  _r9);
   VMP_REG_SET(vm, vm->reg_map[X86_R10], _r10);
   VMP_REG_SET(vm, vm->reg_map[X86_R11], _r11);
   VMP_REG_SET(vm, vm->reg_map[X86_R12], _r12);
   VMP_REG_SET(vm, vm->reg_map[X86_R13], _r13);
   VMP_REG_SET(vm, vm->reg_map[X86_R14], _r14);
   VMP_REG_SET(vm, vm->reg_map[X86_R15], _r15);
   // RSP is not saved/restored (index 4)

   return 4 + len; // opcode(1) + len(2) + native_bytes + RET(1)
 }

 #endif /* __x86_64__ */
