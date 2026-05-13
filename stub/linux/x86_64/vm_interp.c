/*
 * vm_interp.c — Modular VM Interpreter (Linux/x86_64 PIC blob)
 */

#include "vm_decode.h"
#include "vm_opcodes.h"
#include "vm_opcodes_dynamic.h"
#include "vm_types.h"
#include "vm_crc.h"
#include "vm_security.h"

#include "vm_handlers/h_alu.h"
#include "vm_handlers/h_branch.h"
#include "vm_handlers/h_cmp.h"
#include "vm_handlers/h_mem.h"
#include "vm_handlers/h_mov.h"
#include "vm_handlers/h_stack.h"
#include "vm_handlers/h_stack_ops.h"
#include "vm_handlers/h_system.h"
#include "vm_handlers/h_fpu.h"
#include "vm_handlers/h_string.h"
#include "vm_handlers/h_snprintf.h"

#ifdef VM_INDIRECT_DISPATCH
#include "vm_dispatch.h"
#endif

#include "vm_token.h"

#include "vm_sections.h"

#ifndef VM_SECTION_ENTRY
  #if defined(__APPLE__)
    #define VM_SECTION_ENTRY __attribute__((section("__TEXT,__text")))
    #define VM_SECTION_DATA  __attribute__((section("__DATA,__data")))
  #else
    #define VM_SECTION_ENTRY __attribute__((section(".text.entry")))
    #define VM_SECTION_DATA  __attribute__((section(".data.entry")))
  #endif
#endif

VM_SECTION_ENTRY
void *memcpy(void *dest, const void *src, unsigned long n) {
  unsigned char *d = (unsigned char *)dest;
  const unsigned char *s = (const unsigned char *)src;
  for (unsigned long i = 0; i < n; i++) d[i] = s[i];
  return dest;
}

static inline void *sys_mmap(unsigned long size) {
  register unsigned long _rax __asm__("rax") = 9; // sys_mmap
  register unsigned long _rdi __asm__("rdi") = 0;
  register unsigned long _rsi __asm__("rsi") = size;
  register unsigned long _rdx __asm__("rdx") = 7; // PROT_READ | PROT_WRITE | PROT_EXEC
  register unsigned long _r10 __asm__("r10") = 0x22; // MAP_PRIVATE | MAP_ANONYMOUS
  register unsigned long _r8 __asm__("r8") = -1;
  register unsigned long _r9 __asm__("r9") = 0;
  __asm__ volatile(
    "syscall"
    : "+r"(_rax)
    : "r"(_rdi), "r"(_rsi), "r"(_rdx), "r"(_r10), "r"(_r8), "r"(_r9)
    : "rcx", "r11", "memory"
  );
  return (void *)_rax;
}

static inline void sys_munmap(void *addr, unsigned long size) {
  unsigned long _rax = 11; // sys_munmap
  unsigned long _rdi = (unsigned long)addr;
  unsigned long _rsi = size;
  __asm__ volatile(
    "syscall"
    : "+a"(_rax)
    : "D"(_rdi), "S"(_rsi)
    : "rcx", "r11", "memory"
  );
}

VM_SECTION_ENTRY u64
vm_entry(u64 *args, u8 *enc_bc, u32 bc_len, u8 xor_key, u64 slide, void *rtlr_ptr, u32 func_id);

VM_SECTION_DATA volatile u64 _token_table_va[3] = {0, 0, 0};
#define _link_time_self_va  _token_table_va[1]
#define _rtlr_off_val       _token_table_va[2]

__attribute__((noinline)) VM_SECTION_ENTRY u64
vm_entry_token_inner(u64 *args, u32 token) {
  u8 xor_key = (u8)TOKEN_XOR_KEY(token);
  u32 func_id = TOKEN_FUNC_ID(token);
  extern u8 _vmp_stub_base;
  
  u64 self_va;
  /* RIP-relative access to _vmp_stub_base in x86_64 PIC */
  __asm__ volatile("lea _vmp_stub_base(%%rip), %0" : "=r"(self_va));
  
  u64 tbl_off = _token_table_va[0];
  if (__builtin_expect(tbl_off == 0, 0)) return 0;
  u64 link_time_self = _link_time_self_va;
  u64 slide = (link_time_self != 0) ? (self_va - link_time_self) : 0;
  token_desc_t *table = (token_desc_t *)(self_va + tbl_off);
  u8 *enc_bc = (u8 *)(self_va + table[func_id].bc_off);
  u32 bc_len = table[func_id].bc_len;

  if (__builtin_expect(enc_bc == (u8 *)self_va || bc_len == 0, 0)) return 0;
  u64 rtlr_off = _rtlr_off_val;
  void *rtlr_ptr = (rtlr_off != 0) ? (void *)(self_va + rtlr_off) : 0;
  return vm_entry(args, enc_bc, bc_len, xor_key, slide, rtlr_ptr, func_id);
}

#define OC_DECRYPT(pc, key) ((u8)((key) ^ ((pc) * 0x9E3779B9u)))

VM_SECTION_ENTRY u64
vm_entry(u64 *args, u8 *enc_bc, u32 bc_len, u8 xor_key, u64 slide, void *rtlr_ptr, u32 func_id) {
  u64 ret = 0;
  if (bc_len > VM_BYTECODE_MAX) bc_len = VM_BYTECODE_MAX;
  u32 alloc_size = (bc_len + 4095u) & ~4095u;
  u8 *bc_buf = (u8 *)sys_mmap(alloc_size);
  if ((long)bc_buf < 0) return 0;

  u64 xk8 = (u64)xor_key; xk8 |= xk8 << 8; xk8 |= xk8 << 16; xk8 |= xk8 << 32;
  u32 n8 = bc_len >> 3;
  for (u32 i = 0; i < n8; i++) ((u64 *)bc_buf)[i] = ((const u64 *)enc_bc)[i] ^ xk8;
  for (u32 i = n8 << 3; i < bc_len; i++) bc_buf[i] = enc_bc[i] ^ xor_key;

  if (rtlr_ptr != 0) {
    u8 *rtlr_start = (u8 *)rtlr_ptr;
    if (*(u32 *)rtlr_start == 0x524C5452) { // "RTLR"
      u32 count = *(u32 *)(rtlr_start + 4);
      u8 *entry = rtlr_start + 8;
      for (u32 i = 0; i < count; i++) {
        if (*(u64 *)entry == (u64)func_id) {
          u64 bc_off = *(u64 *)(entry + 8);
          if (bc_off <= (u64)bc_len - 8) *(u64 *)(bc_buf + bc_off) = *(u64 *)(entry + 16) + slide;
        } else if (*(u64 *)entry > (u64)func_id) break;
        entry += 24;
      }
    }
  }

  u32 ctx_alloc = (sizeof(vm_ctx_t) + 4095u) & ~4095u;
  vm_ctx_t *vm = (vm_ctx_t *)sys_mmap(ctx_alloc);
  if ((long)vm < 0) { sys_munmap(bc_buf, alloc_size); return 0; }
  
  if (bc_len >= 88 + 256) {
    u32 trail_map_count = rd32(&bc_buf[bc_len - 16]);
    u32 map_data_size = trail_map_count * 8 + 344 + 21;

    if (map_data_size <= bc_len) {
      u8 *tr_ptr = &bc_buf[bc_len - map_data_size + 24]; // Skip CRC
      u8 *reg_map_ptr = tr_ptr;
      u8 *op_map = tr_ptr + 64;
      
      for (int i = 0; i < VM_REG_COUNT; i++) vm->reg_map[i] = reg_map_ptr[i] & (VM_REG_COUNT-1);
      vm_ctx_init(vm, args, bc_buf, bc_len - map_data_size);
      vm->oc_key = rd32(&bc_buf[bc_len - 20]);
      
      u8 phys_isz[256];
      for (int i = 0; i < 256; i++) phys_isz[i] = 0;
      void *jump_table[256];
      vm_handler_fn handlers[OP_ID_COUNT];
      vm_init_jump_table(handlers);
      for (int i = 0; i < OP_ID_COUNT; i++) {
        jump_table[op_map[i]] = (void *)handlers[i];
        phys_isz[op_map[i]] = vm_logical_insn_size(i);
      }

      for (;;) {
        u8 _raw = vm->bc[vm->pc];
        u8 _dec = vm->oc_key ? (_raw ^ OC_DECRYPT(vm->pc, vm->oc_key)) : _raw;
        u8 _isz = phys_isz[_dec];
        if (__builtin_expect(_isz == 0, 0)) break;
        u32 (*handler)(vm_ctx_t *) = (u32 (*)(vm_ctx_t *))jump_table[_dec];
        u32 status = handler(vm);
        if (__builtin_expect(status == VM_STEP_RET || status == VM_STEP_HALT, 0)) break;
        if (status > 0) vm->pc += status;
      }
      ret = vm->R[vm->reg_map[X86_RAX]];
    }
  }

  sys_munmap(vm, ctx_alloc);
  sys_munmap(bc_buf, alloc_size);
  return ret;
}
