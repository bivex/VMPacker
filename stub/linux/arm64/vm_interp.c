/*
 * vm_interp.c — Modular VM Interpreter (Linux/ARM64 PIC blob)
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
    : "+r"(dest), "+r"(src), "+r"(n) : : "w3", "memory"
  );
  return ret;
}

static inline void *sys_mmap(unsigned long size) {
  register long x8 __asm__("x8") = 222;
  register long x0 __asm__("x0") = 0;
  register long x1 __asm__("x1") = (long)size;
  register long x2 __asm__("x2") = 3;
  register long x3 __asm__("x3") = 0x22;
  register long x4 __asm__("x4") = -1;
  register long x5 __asm__("x5") = 0;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2), "r"(x3), "r"(x4), "r"(x5) : "memory");
  return (void *)x0;
}

static inline void sys_munmap(void *addr, unsigned long size) {
  register long x8 __asm__("x8") = 215;
  register long x0 __asm__("x0") = (long)addr;
  register long x1 __asm__("x1") = (long)size;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1) : "memory");
}

__attribute__((section(".text.entry"))) u64
vm_entry(u64 *args, u8 *enc_bc, u32 bc_len, u8 xor_key, u64 slide, void *rtlr_ptr, u32 func_id);

__attribute__((section(".data.entry"), used)) volatile u64 _token_table_va = 0;
__attribute__((section(".data.entry"), used)) volatile u64 _link_time_self_va = 0;

__attribute__((noinline, section(".text.entry"))) u64
vm_entry_token_inner(u64 *args, u32 token) {
  u8 xor_key = (u8)TOKEN_XOR_KEY(token);
  u32 func_id = TOKEN_FUNC_ID(token);
  extern u8 _vmp_stub_base;
  u64 self_va;
  __asm__ volatile("adr %0, _vmp_stub_base" : "=r"(self_va));
  u64 tbl_off = *(volatile u64 *)&_token_table_va;
  if (__builtin_expect(tbl_off == 0, 0)) return 0;
  u64 link_time_self = *(volatile u64 *)&_link_time_self_va;
  u64 slide = (link_time_self != 0) ? (self_va - link_time_self) : 0;
  token_desc_t *table = (token_desc_t *)(self_va + tbl_off);
  u8 *enc_bc = (u8 *)(self_va + table[func_id].bc_off);
  u32 bc_len = table[func_id].bc_len;
  if (__builtin_expect(enc_bc == (u8 *)self_va || bc_len == 0, 0)) return 0;
  u64 rtlr_off = *(volatile u64 *)(&_token_table_va + 2);
  void *rtlr_ptr = (rtlr_off != 0) ? (void *)(self_va + rtlr_off) : 0;
  return vm_entry(args, enc_bc, bc_len, xor_key, slide, rtlr_ptr, func_id);
}

__attribute__((section(".text.entry"))) u64
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
    if (*(u32 *)rtlr_start == 0x524C5452) {
      u32 count = *(u32 *)(rtlr_start + 4);
      if (count > 1000000) count = 1000000;
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
  
  u8 *op_map = 0;
  if (bc_len >= 88 + 256) {
    u32 trail_map_count = rd32(&bc_buf[bc_len - 16]);
    u32 map_data_size = trail_map_count * 8 + 344 + 24;
    
    if (map_data_size <= bc_len) {
      u8 *tr_ptr = &bc_buf[bc_len - map_data_size + 24]; // Skip CRC
      u8 *reg_map_ptr = tr_ptr;
      for (int i = 0; i < 32; i++) vm->reg_map[i] = reg_map_ptr[i] & 31;
      op_map = tr_ptr + 64;
      vm->addr_map = tr_ptr + 64 + 256;
      
      vm_ctx_init(vm, args, bc_buf, bc_len - map_data_size);
      vm->func_addr = rd64(&bc_buf[bc_len - 12]);
      vm->func_size = rd32(&bc_buf[bc_len - 4]);
      vm->map_count = trail_map_count;
      vm->reverse = bc_buf[bc_len - 21];
      vm->oc_key = rd32(&bc_buf[bc_len - 20]);
      vm->slide = slide;
      vm->ret_reg = vm->reg_map[0];
    } else {
      for(int i=0; i<32; i++) vm->reg_map[i] = i;
      vm_ctx_init(vm, args, bc_buf, bc_len);
      vm->ret_reg = 0;
    }
  } else {
    for(int i=0; i<32; i++) vm->reg_map[i] = i;
    vm_ctx_init(vm, args, bc_buf, bc_len);
    vm->ret_reg = 0;
  }

#define OC_DECRYPT(pc, key) ((u8)((key) ^ ((pc) * 0x9E3779B9u)))
#ifdef VM_INDIRECT_DISPATCH
  vm_handler_fn jump_table[256]; u8 phys_isz[256];
  for (int i = 0; i < 256; i++) { jump_table[i] = hw_unknown; phys_isz[i] = 0; }
  
  u8 inv_map[256];
  for (int i = 0; i < 256; i++) inv_map[i] = 255;

  if (op_map) {
    vm_handler_fn handlers[OP_ID_COUNT]; vm_init_jump_table(handlers);
    for (int i = 0; i < OP_ID_COUNT; i++) {
      jump_table[op_map[i]] = handlers[i];
      phys_isz[op_map[i]] = vm_logical_insn_size(i);
      inv_map[op_map[i]] = (u8)i;
    }
  } else {
    vm_init_jump_table(jump_table);
    for (int i = 0; i < 256; i++) {
        phys_isz[i] = vm_logical_insn_size((u8)i);
        inv_map[i] = (u8)i;
    }
  }

  if (vm->reverse) vm->pc = vm->bc_len;
  for (;;) {
    if (++vm->insn_count > 1000000) { ret = 110; goto cleanup; }
    if (vm->reverse) {
      if (vm->pc <= 0) break;
      vm->pc--; if (vm->pc >= vm->bc_len) break;
      u8 _sz = vm->bc[vm->pc]; if (_sz > vm->pc) break;
      vm->pc -= _sz;
    } else { if (vm->pc >= vm->bc_len) break; }
    u8 _raw = vm->bc[vm->pc];
    u8 _dec = vm->oc_key ? (_raw ^ OC_DECRYPT(vm->pc, vm->oc_key)) : _raw;
    if (phys_isz[_dec] == 0) break;
    u32 _s = jump_table[_dec](vm);
    if (_s == VM_STEP_HALT || _s == VM_STEP_RET) { ret = vm->R[vm->ret_reg]; goto cleanup; }
    if (_s > 0 && !vm->reverse) vm->pc += _s;
  }
#endif
cleanup:
  /* if (vm) sec_zero_memory(vm, ctx_alloc); */
  /* if (bc_buf) sec_zero_memory(bc_buf, alloc_size); */
  return ret;
}
