/*
 * vm_interp.c — Modular VM Interpreter (Linux/ARM64 PIC blob)
 *
 * Architecture:
 *   vm_types.h       → Types + vm_ctx_t struct
 *   vm_opcodes.h     → Opcode definitions
 *   vm_decode.h      → Bytecode reading utilities
 *   vm_handlers/*.h  → Modular instruction handlers
 *
 * Compilation (Cross-compile to blob):
 *   aarch64-linux-gnu-gcc -c -Os -mcmodel=tiny -fno-stack-protector \
 *     -fno-builtin -nostdlib -march=armv8-a vm_interp.c -o vm_interp.o
 */

/* ---- Infrastructure ---- */

#include "vm_decode.h"
#include "vm_opcodes.h"
#include "vm_opcodes_dynamic.h"
#include "vm_types.h"
#include "vm_crc.h"
#include "vm_security.h"

/* ---- Instruction Handler Modules ---- */
#include "vm_handlers/h_alu.h" /* ADD/SUB/MUL/XOR/AND/OR/SHL/SHR/ASR/NOT/ROR + _IMM */
#include "vm_handlers/h_branch.h" /* JMP/JE/JNE/JL/JGE/JGT/JLE/JB/JAE */
#include "vm_handlers/h_cmp.h"    /* CMP, CMP_IMM */
#include "vm_handlers/h_mem.h"    /* LOAD/STORE 8/32/64 */
#include "vm_handlers/h_mov.h"    /* MOV_IMM, MOV_IMM32, MOV_REG */
#include "vm_handlers/h_stack.h"  /* PUSH, POP */
#include "vm_handlers/h_stack_ops.h" /* Stack machine operation handlers (VLOAD/VSTORE/VADD...) */
#include "vm_handlers/h_system.h" /* NOP, CALL_NAT, BR_REG, VLD16, VST16 */
#include "vm_handlers/h_fpu.h"    /* FADD, FMUL, FCVT, ... */
#include "vm_handlers/h_string.h" /* S_DECRYPT_STR */
#include "vm_handlers/h_snprintf.h" /* S_PRINTF */


/* #define VM_DEBUG_TRACE */

/* ---- Indirect Dispatch Jump Table (Conditional Compilation) ---- */
#ifdef VM_INDIRECT_DISPATCH
#include "vm_dispatch.h"
#endif

/* ---- Tokenized Entry (Conditional Compilation) ---- */
/* TOKEN_ONLY: Token entry is always compiled */
#include "vm_token.h"

/* ---- Utils (no libc) ---- */
__attribute__((section(".text.entry")))
void *memcpy(void *dest, const void *src, unsigned long n) {
  void *ret = dest;
  __asm__ volatile (
    "cbz %2, 2f\n"
    "1:\n"
    "ldrb w3, [%1], #1\n"
    "strb w3, [%0], #1\n"
    "subs %2, %2, #1\n"
    "b.ne 1b\n"
    "2:\n"
    : "+r"(dest), "+r"(src), "+r"(n)
    : 
    : "w3", "memory"
  );
  return ret;
}

/* ---- syscall: mmap (no libc dependency) ---- */
static inline void *sys_mmap(unsigned long size) {
  register long x8 __asm__("x8") = 222; /* __NR_mmap */
  register long x0 __asm__("x0") = 0;   /* addr = NULL */
  register long x1 __asm__("x1") = (long)size;
  register long x2 __asm__("x2") = 3;   /* PROT_READ | PROT_WRITE */
  register long x3 __asm__("x3") = 0x22; /* MAP_PRIVATE | MAP_ANONYMOUS */
  register long x4 __asm__("x4") = -1;   /* fd = -1 */
  register long x5 __asm__("x5") = 0;    /* offset = 0 */
  __asm__ volatile(
      "svc #0\n"
      : "+r"(x0)
      : "r"(x8), "r"(x1), "r"(x2), "r"(x3), "r"(x4), "r"(x5)
      : "memory");
  return (void *)x0;
}

/* ---- syscall: munmap ---- */
static inline void sys_munmap(void *addr, unsigned long size) {
  register long x8 __asm__("x8") = 215; /* __NR_munmap */
  register long x0 __asm__("x0") = (long)addr;
  register long x1 __asm__("x1") = (long)size;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1) : "memory");
}

/*
 * vm_entry — VM Interpreter Entry
 *
 * Parameters:
 *   args    : Pointer to saved X0-X7, callerFP, callerLR, V0-V7, X19-X28
 *             (Total 36 u64: args[0..7]=X0-X7, args[8..9]=FP/LR,
 *              args[10..25]=V0-V7, args[26..35]=X19-X28)
 *   enc_bc  : XOR encrypted bytecode
 *   bc_len  : bytecode length
 *   xor_key : XOR decryption key
 *
 * Returns: R[0] (Simulated X0 return value)
 */
__attribute__((section(".text.entry"))) u64
vm_entry(u64 *args, u8 *enc_bc, u32 bc_len, u8 xor_key, u64 slide, void *rtlr_ptr,
         u32 func_id);

/* ================================================================
 * Tokenized Entry (Conditional Compilation)
 *
 * Token trampoline (3 instructions):
 *   MOV  W16, #token_lo16
 *   MOVK W16, #token_hi16, LSL#16
 *   B    vm_entry_token
 *
 * X16 (IP0) passes token, X0-X7 remain original caller parameters.
 * vm_entry_token_asm is responsible for saving registers and calling vm_entry_token_inner.
 * ================================================================ */
/* TOKEN_ONLY: Token entry is always compiled */

/* Packer patches this variable with the token descriptor table VA in payload */
__attribute__((section(".data.entry"), used)) volatile u64 _token_table_va = 0;
/* Packer patches this with the link-time VA of _token_table_va,
 * so the stub can compute ASLR slide = runtime_self_va - link_time_self_va */
__attribute__((section(".data.entry"), used)) volatile u64 _link_time_self_va = 0;

/* Internal C function: Decode token and call vm_entry */
__attribute__((noinline, section(".text.entry"))) u64
vm_entry_token_inner(u64 *args, u32 token) {
  u8 xor_key = (u8)TOKEN_XOR_KEY(token);
  u32 func_id = TOKEN_FUNC_ID(token);

  /* PIE compatible: _token_table_va stores the offset relative to the stub base address
   * Use ADR to get stub base address (PC-relative, ±1MB) */
  extern u8 _vmp_stub_base;
  u64 self_va;
  __asm__ volatile("adr %0, _vmp_stub_base" : "=r"(self_va));
  u64 tbl_off = *(volatile u64 *)&_token_table_va;
  if (__builtin_expect(tbl_off == 0, 0))
    return 0; /* Table not initialized, safe exit */

  /* Compute ASLR slide for PIE/ET_DYN */
  u64 link_time_self = *(volatile u64 *)&_link_time_self_va;
  u64 slide = (link_time_self != 0) ? (self_va - link_time_self) : 0;

  token_desc_t *table = (token_desc_t *)(self_va + tbl_off);
  /* bc_off is also the offset relative to _token_table_va */
  u8 *enc_bc = (u8 *)(self_va + table[func_id].bc_off);
  u32 bc_len = table[func_id].bc_len;

  if (__builtin_expect(enc_bc == (u8 *)self_va || bc_len == 0, 0))
    return 0; /* Invalid entry, safe exit */

  /* ---- Runtime Relocation (RTLR) ---- */
  /* RTLR table offset stored at _token_table_va + 16 */
  u64 rtlr_off = *(volatile u64 *)(&_token_table_va + 2);
  void *rtlr_ptr = (rtlr_off != 0) ? (void *)(self_va + rtlr_off) : 0;

  return vm_entry(args, enc_bc, bc_len, xor_key, slide, rtlr_ptr, func_id);
}

/* ---- vm_entry Implementation ---- */
__attribute__((section(".text.entry"))) u64
vm_entry(u64 *args, u8 *enc_bc, u32 bc_len, u8 xor_key, u64 slide, void *rtlr_ptr,
         u32 func_id) {
  VM_DEBUG("[VM] vm_entry starting...\n");
  u64 ret = 0;
  
  /* ---- 1. Dynamically allocate bytecode buffer (mmap, replacing 64KB on stack) ---- */
  if (bc_len > VM_BYTECODE_MAX)
    bc_len = VM_BYTECODE_MAX;
  u32 alloc_size = (bc_len + 4095u) & ~4095u; /* Page alignment rounded up */
  u8 *bc_buf = (u8 *)sys_mmap(alloc_size);
  if ((long)bc_buf < 0)
    return 0; /* mmap failed, safe exit */

  /* ---- 1b. XOR decryption (8-byte widening, ~8x speedup) ---- */
  u64 xk8 = (u64)xor_key;
  xk8 |= xk8 << 8;
  xk8 |= xk8 << 16;
  xk8 |= xk8 << 32;
  {
    u32 n8 = bc_len >> 3;
    u64 *d8 = (u64 *)bc_buf;
    const u64 *s8 = (const u64 *)enc_bc;
    for (u32 i = 0; i < n8; i++)
      d8[i] = s8[i] ^ xk8;
    for (u32 i = n8 << 3; i < bc_len; i++)
      bc_buf[i] = enc_bc[i] ^ xor_key;
  }

  /* ---- 1c. Runtime Relocation (RTLR) — executed in writable buffer ---- */
  if (rtlr_ptr != 0) {
    u8 *rtlr_start = (u8 *)rtlr_ptr;
    if (*(u32 *)rtlr_start == 0x524C5452) { /* "RTLR" */
      u32 count = *(u32 *)(rtlr_start + 4);
      if (count > 1000000) count = 1000000;
      u8 *entry = rtlr_start + 8;
      for (u32 i = 0; i < count; i++) {
        u64 e_func_id = *(u64 *)entry;
        if (e_func_id == (u64)func_id) {
          u64 bc_off = *(u64 *)(entry + 8);
          u64 target_va = *(u64 *)(entry + 16);
          if (bc_off <= (u64)bc_len - 8) {
             u64 *patch_addr = (u64 *)(bc_buf + bc_off);
             *patch_addr = target_va + slide;
           }
        } else if (e_func_id > (u64)func_id) {
          break; /* entries sorted by func_id */
        }
        entry += 24;
      }
    }
  }

  /* ---- 2b. Initialize VM context (mmap heap allocation) ---- */
  u32 ctx_alloc = (sizeof(vm_ctx_t) + 4095u) & ~4095u;
  vm_ctx_t *vm = (vm_ctx_t *)sys_mmap(ctx_alloc);
  if ((long)vm < 0) {
    sys_munmap(bc_buf, alloc_size);
    return 0;
  }
  vm_ctx_init(vm, args, bc_buf, bc_len);
  vm->slide = slide;

  /* ---- Anti-Tampering: Memory Dump Protection ---- */
  sec_protect_memory(bc_buf, alloc_size);
  sec_protect_memory(vm, ctx_alloc);

  u8 *op_map = 0;

  /* ---- 2c. Parse bytecode trailer ---- */
  if (bc_len >= 85 + 256) {
    u32 trail_func_size = rd32(&bc_buf[bc_len - 4]);
    u64 trail_func_addr = rd64(&bc_buf[bc_len - 12]);
    u32 trail_map_count = rd32(&bc_buf[bc_len - 16]);
    u32 trail_oc_key = rd32(&bc_buf[bc_len - 20]);
    u8 trail_reverse = bc_buf[bc_len - 21];

    u32 crc_size = 24;
    u32 map_data_size = trail_map_count * 8 + 85 + 256 + crc_size;

    u8 *reg_map = &bc_buf[bc_len - map_data_size + crc_size];
    op_map = reg_map + 64;

    if (trail_func_addr != 0 && trail_map_count > 0 && map_data_size <= bc_len) {
      vm->func_addr = trail_func_addr;
      vm->func_size = trail_func_size;
      vm->map_count = trail_map_count;
      vm->reverse = trail_reverse;
      vm->oc_key = trail_oc_key;
      vm->addr_map = (addr_map_entry_t *)&bc_buf[bc_len - map_data_size + crc_size + 320];

      for (int i = 0; i < 32; i++) vm->reg_map[i] = reg_map[i] & 31;
      u64 initial_sp = vm->R[31];

      for (int i = 0; i < 8; i++) vm->R[vm->reg_map[i]] = args[i];
      vm->R[vm->reg_map[29]] = args[8];
      vm->R[vm->reg_map[30]] = args[9];
      vm->R[vm->reg_map[31]] = initial_sp;
      for (int i = 0; i < 8; i++) {
        vm->V[i][0] = args[10 + i * 2];
        vm->V[i][1] = args[10 + i * 2 + 1];
      }
      for (int i = 0; i < 10; i++) vm->R[vm->reg_map[19 + i]] = args[26 + i];
      vm->ret_reg = vm->reg_map[0];
      vm->debug = 0;
      vm->bc_len = bc_len - map_data_size;
    } else {
      vm->bc_len = bc_len - 21;
    }
    
    /* ---- Anti-Tampering: CRC32 Bytecode Integrity Check ---- */
    if (bc_len >= map_data_size + CRC_SECTION_SIZE) {
      u32 magic = rd32(&bc_buf[bc_len - map_data_size - 4]);
      if (magic == CRC_MAGIC) {
        u32 expected_bc_crc = rd32(&bc_buf[bc_len - map_data_size - 8]);
        u32 bc_crc_len = bc_len - map_data_size - CRC_SECTION_SIZE;
        if (expected_bc_crc == crc32_calc(bc_buf, bc_crc_len)) {
            vm->bc_len = bc_crc_len;
        }
      }
    }
  }

if (sec_scan_inline_hook((void *)memcpy)) { return 104; }
if (sec_scan_breakpoints(bc_buf, vm->bc_len)) { return 109; }

#define OC_DECRYPT(pc, key) ((u8)((key) ^ ((pc) * 0x9E3779B9u)))

#ifdef VM_INDIRECT_DISPATCH
  vm_handler_fn vm_jump_table[256];
  u8 phys_insn_size[256];
  for (int i = 0; i < 256; i++) phys_insn_size[i] = 0;

  if (op_map) {
    vm_handler_fn handlers[OP_ID_COUNT];
    vm_init_jump_table(handlers);
    for (int i = 0; i < 256; i++) vm_jump_table[i] = hw_unknown;
    for (int i = 0; i < OP_ID_COUNT; i++) {
      vm_jump_table[op_map[i]] = handlers[i];
      phys_insn_size[op_map[i]] = vm_logical_insn_size(i);
    }
  } else {
    vm_init_jump_table(vm_jump_table);
    for (int i = 0; i < 256; i++) phys_insn_size[i] = vm_insn_size(i);
  }

  if (vm->reverse) vm->pc = vm->bc_len;

  for (;;) {
    if (__builtin_expect(++vm->insn_count > 2000000, 0)) {
        ret = 110;
        goto cleanup;
    }
    if (__builtin_expect((vm->insn_count & 0x3FF) == 0, 0)) {
        int sec_res = sec_runtime_check(vm);
        if (__builtin_expect(sec_res != 0, 0)) { ret = (u64)sec_res; goto cleanup; }
    }
    if (vm->reverse) {
      if (__builtin_expect((i64)vm->pc <= 0, 0)) break;
      vm->pc--;
      if (__builtin_expect(vm->pc >= vm->bc_len, 0)) break;
      u8 _sz = vm->bc[vm->pc];
      if (__builtin_expect(_sz > vm->pc, 0)) break;
      vm->pc -= _sz;
    } else {
      if (__builtin_expect(vm->pc >= vm->bc_len, 0)) break;
    }
    u8 _raw_op = vm->bc[vm->pc];
    u8 _dec_op = vm->oc_key ? (_raw_op ^ OC_DECRYPT(vm->pc, vm->oc_key)) : _raw_op;
    u8 _isz = phys_insn_size[_dec_op];
    if (__builtin_expect(_isz == 0 || vm->pc + _isz > vm->bc_len, 0)) break;
    u32 _step = vm_jump_table[_dec_op](vm);
    if (__builtin_expect(_step == VM_STEP_HALT || _step == VM_STEP_RET, 0)) {
      ret = vm->R[vm->ret_reg];
      goto cleanup;
    }
    if (_step > 0 && !vm->reverse) vm->pc += _step;
  }
#else
  /* ... Computed Goto mode fallback omitted for brevity or implemented similarly ... */
  ret = 0;
#endif

cleanup:
  if (vm) sec_zero_memory(vm, ctx_alloc);
  if (bc_buf) sec_zero_memory(bc_buf, alloc_size);
  return ret;
}
