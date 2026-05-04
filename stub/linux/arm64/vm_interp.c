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
  /* DEBUG: print tbl_off */
  {
    u8 _tb[32];
    _tb[0]='T';_tb[1]=':';_tb[2]='0';_tb[3]='x';
    for(int _i=0;_i<16;_i++) _tb[4+_i]="0123456789ABCDEF"[(tbl_off>>((15-_i)*4))&0xF];
    _tb[20]='\n'; sys_write(1,_tb,21);
  }
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

/* Naked assembly entry: Save caller registers, call internal C function */
/* vm_entry_token moved to vm_entry.S to avoid naked attribute issues with GCC */

/* end TOKEN_ONLY */

/* ---- vm_entry Implementation ---- */
__attribute__((section(".text.entry"))) u64
vm_entry(u64 *args, u8 *enc_bc, u32 bc_len, u8 xor_key, u64 slide, void *rtlr_ptr,
         u32 func_id) {
  VM_DEBUG("[VM] vm_entry starting...\n");
  u64 ret = 0;
  
  /* DEBUG: print incoming bc_len */
  {
    u8 _b[32];
    _b[0]='I';_b[1]=':';_b[2]='0';_b[3]='x';
    for(int _i=0;_i<8;_i++) _b[4+_i]="0123456789ABCDEF"[(bc_len>>((7-_i)*4))&0xF];
    _b[12]='\n'; sys_write(1,_b,13);
  }
  
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
      /* Security check: Limit maximum relocations to prevent out-of-bounds caused by corrupted RTLR table */
      if (count > 1000000) count = 1000000;
      u8 *entry = rtlr_start + 8;
      for (u32 i = 0; i < count; i++) {
        u64 e_func_id = *(u64 *)entry;
        if (e_func_id == (u64)func_id) {
          u64 bc_off = *(u64 *)(entry + 8);
          u64 target_va = *(u64 *)(entry + 16);
          /* Fixup absolute address: runtime_addr = target_link_time_va + slide */
          /* Fix: Prevent bc_off + 8 overflow, check if bc_off is within bounds first */
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
  VM_DEBUG("[VM] allocating ctx...\n");
  vm_ctx_t *vm = (vm_ctx_t *)sys_mmap(ctx_alloc);
  if ((long)vm < 0) {
    VM_DEBUG("[VM] ctx mmap failed!\n");
    sys_munmap(bc_buf, alloc_size);
    return 0;
  }
  VM_DEBUG("[VM] initializing ctx...\n");
  vm_ctx_init(vm, args, bc_buf, bc_len);
  VM_DEBUG("[VM] ctx init done.\n");
  vm->slide = slide;
  VM_DEBUG("[VM] slide set.\n");

  /* ---- Anti-Tampering: Memory Dump Protection ---- */
  sec_protect_memory(bc_buf, alloc_size);
  sec_protect_memory(vm, ctx_alloc);
  /* if (sec_scan_breakpoints(&_vmp_stub_base, 256)) { sec_panic(103); } */

  /* Anti-Debug: Basic Timing Check (measure start time) */
  /* unsigned long long start_time = sec_get_timer(); */
  
  /* Do some dummy work to make timing check more effective */
  /* for (volatile int _i = 0; _i < 1000; _i++) { __asm__ volatile("nop"); } */
  /* unsigned long long end_time = sec_get_timer(); */
  /* if ((end_time - start_time) > 1000000) { sec_panic(104); } */

  u8 *op_map = 0;

  /* ---- 2c. Parse bytecode trailer ---- */
  /* Trailer format (stripped backwards from the end):
   *   [...bytecode...][BR map entries][reverse(1B)][oc_key(4B)]
   *                    [map_count:u32][func_addr:u64][func_size:u32]
   *
   * Stripping order: func_size(4B) → func_addr(8B) → map_count(4B)
   *           → oc_key(4B) → reverse(1B) → BR map entries
   * Fixed trailer size: 4+8+4+4+1 + 64(reg_map) = 85B
   */
  if (bc_len >= 85 + 256) { /* Minimum trailer: 85B + 256B(op_map) */
    VM_DEBUG("[VM] bc_len check passed.\n");
    u32 trail_func_size = rd32(&bc_buf[bc_len - 4]);
    u64 trail_func_addr = rd64(&bc_buf[bc_len - 12]);
    u32 trail_map_count = rd32(&bc_buf[bc_len - 16]);
    u32 trail_oc_key = rd32(&bc_buf[bc_len - 20]);
    u8 trail_reverse = bc_buf[bc_len - 21];
    {
      u8 _rvbuf[4];
      _rvbuf[0] = trail_reverse ? '1' : '0';
      _rvbuf[1] = '\n';
      VM_DEBUG("[VM] trail_reverse: ");
      sys_write(1, _rvbuf, 2);
    }
    /* DEBUG: print trail_map_count */
    {
      u8 _mb[16];
      _mb[0]='M';_mb[1]=':';_mb[2]='0';_mb[3]='x';
      for(int _i=0;_i<4;_i++) _mb[4+_i]="0123456789ABCDEF"[(trail_map_count>>((3-_i)*4))&0xF];
      _mb[8]='\n'; sys_write(1,_mb,9);
    }
    u32 map_data_size =
        trail_map_count * 8 +
        85 + 256; // 85 (fixed) + 256 (op_map)
    /* DEBUG: print map_data_size */
    {
      u8 _md[20];
      _md[0]='D';_md[1]=':';_md[2]='0';_md[3]='x';
      for(int _i=0;_i<8;_i++) _md[4+_i]="0123456789ABCDEF"[(map_data_size>>((7-_i)*4))&0xF];
      _md[12]='\n'; sys_write(1,_md,13);
    }
    
    u8 *reg_map = &bc_buf[bc_len - map_data_size];
    op_map = reg_map + 64;

    if (trail_func_addr != 0 && trail_map_count > 0 &&
        map_data_size <= bc_len) {
      VM_DEBUG("[VM] trailer validation passed.\n");
      vm->func_addr = trail_func_addr + vm->slide;
      vm->func_size = trail_func_size;
      vm->map_count = trail_map_count;
      vm->reverse = trail_reverse;
      vm->oc_key = trail_oc_key;
      vm->addr_map = (addr_map_entry_t *)&bc_buf[bc_len - map_data_size + 320];
      /* 2d. Initialize registers with shuffling */
      for (int i = 0; i < 8; i++) {
        vm->R[reg_map[i] & 63] = args[i];
      }
      /* Save regMap[0] for return */
      vm->ret_reg = reg_map[0] & 63;
      
      vm->bc_len = bc_len - map_data_size; /* Actual bytecode does not include trailer */

      /* Insertion sort addr_map (ascending by arm64_off, for binary search) */
      /* Note: Use field-level copy to avoid compiler-generated implicit memcpy (-nostdlib) */
      for (u32 j = 1; j < vm->map_count; j++) {
        u32 t_arm = vm->addr_map[j].arm64_off;
        u32 t_vm = vm->addr_map[j].vm_off;
        int k = (int)j - 1;
        while (k >= 0 && vm->addr_map[k].arm64_off > t_arm) {
          vm->addr_map[k + 1].arm64_off = vm->addr_map[k].arm64_off;
          vm->addr_map[k + 1].vm_off = vm->addr_map[k].vm_off;
          k--;
        }
        vm->addr_map[k + 1].arm64_off = t_arm;
        vm->addr_map[k + 1].vm_off = t_vm;
      }
    } else {
      /* No BR map: Strip only 21B fixed trailer */
      vm->bc_len = bc_len - 21;
      map_data_size = 21 + 256;
    }
    
    /* ---- Anti-Tampering: CRC32 Bytecode Integrity Check ---- */
    if (bc_len >= map_data_size + CRC_SECTION_SIZE) {
      u32 magic = rd32(&bc_buf[bc_len - map_data_size - 4]);
      if (magic == CRC_MAGIC) {
        u32 expected_bc_crc = rd32(&bc_buf[bc_len - map_data_size - 8]);
        u32 bc_crc_len = bc_len - map_data_size - CRC_SECTION_SIZE;
        u32 actual_bc_crc = crc32_calc(bc_buf, bc_crc_len);
        if (expected_bc_crc != actual_bc_crc) {
          return 103; /* Integrity check failed, abort execution */
        }
        /* Update actual bc_len to exclude the CRC section */
        vm->bc_len = bc_crc_len;
        vm->expected_bc_crc = expected_bc_crc;
        vm->bc_crc_len = bc_crc_len;
      }
    }
  }

/* ---- Anti-Hook: Inline Hook Detection ---- */
/* Detect inline hooks on some common targets */
if (sec_scan_inline_hook((void *)memcpy)) { return 104; }
if (sec_scan_inline_hook((void *)vm_entry)) { return 105; }
if (sec_scan_inline_hook((void *)sys_mmap)) { return 106; }
if (sec_scan_inline_hook((void *)sys_ptrace)) { return 107; }

/* ---- Anti-Debug: Timing Check (Verify execution time hasn't been delayed) ---- */
/*
end_time = sec_get_timer();
if (end_time - start_time > 1000000) { 
  return 108;
}
*/

/* ---- Anti-Debug: Breakpoint Scanning ---- */
if (sec_scan_breakpoints(bc_buf, vm->bc_len)) { return 109; }

/* ---- OpcodeCryptor decryption macro (common to both modes) ---- */
#define OC_DECRYPT(pc, key) ((u8)((key) ^ ((pc) * 0x9E3779B9u)))

#ifdef VM_INDIRECT_DISPATCH
  /* ================================================================
   * Indirect Dispatch mode: Relative offset jump table + indirect function pointer call
   *
   * Replaces computed goto, making static analysis tools like IDA Pro
   * unable to track all handler target addresses.
   * ================================================================ */

  /* ---- Runtime initialization of jump table (stack allocation, RX blob non-writable BSS) ---- */
  vm_handler_fn vm_jump_table[256];
  u8 phys_insn_size[256];
  for (int i = 0; i < 256; i++)
    phys_insn_size[i] = 0;

  u8 inv_map[256];
  for (int i = 0; i < 256; i++) inv_map[i] = 255;

  if (op_map) {
    vm_handler_fn handlers[OP_ID_COUNT];
    vm_init_jump_table(handlers);
    for (int i = 0; i < 256; i++)
      vm_jump_table[i] = hw_unknown;
    for (int i = 0; i < OP_ID_COUNT; i++) {
      vm_jump_table[op_map[i]] = handlers[i];
      phys_insn_size[op_map[i]] = vm_logical_insn_size(i);
      inv_map[op_map[i]] = (u8)i;
    }
    VM_DEBUG("[VM] op_map[50] (CallNative): ");
    {
      u8 _opbuf[4];
      u8 _op = op_map[50];
      _opbuf[0] = (_op >> 4) < 10 ? '0' + (_op >> 4) : 'A' + (_op >> 4) - 10;
      _opbuf[1] = (_op & 0xF) < 10 ? '0' + (_op & 0xF) : 'A' + (_op & 0xF) - 10;
      _opbuf[2] = '\n';
      sys_write(1, _opbuf, 3);
    }
    vm_jump_table[OP_S_DECRYPT_STR] = h_s_decrypt_str;
    phys_insn_size[OP_S_DECRYPT_STR] = 1;
  } else {
    vm_init_jump_table(vm_jump_table);
    for (int i = 0; i < 256; i++)
      phys_insn_size[i] = vm_insn_size(i);
  }

  /* ---- PC initialization: reverse mode starts from bc_len ---- */
  if (vm->reverse) {
    vm->pc = vm->bc_len;
  }
  /* DEBUG: dump oc_key, bc_len, first 4 opcodes at bc_len-size-1 */
  {
    u8 _db[64];
    _db[0]='O';_db[1]='K';_db[2]=':';_db[3]='0';_db[4]='x';
    u32 _ok=vm->oc_key;
    for(int _i=0;_i<8;_i++){_db[5+_i]="0123456789ABCDEF"[(_ok>>((7-_i)*4))&0xF];}
    _db[13]=' ';_db[14]='B';_db[15]='L';_db[16]=':';
    u32 _bl=vm->bc_len;
    for(int _i=0;_i<4;_i++){_db[17+_i]="0123456789ABCDEF"[(_bl>>((3-_i)*4))&0xF];}
    _db[21]=' ';_db[22]='S';_db[23]='Z';_db[24]=':';
    u8 _sz0=vm->bc[vm->bc_len-1];
    for(int _i=0;_i<2;_i++){_db[25+_i]="0123456789ABCDEF"[(_sz0>>((1-_i)*4))&0xF];}
    u32 _p0=vm->bc_len-1-_sz0;
    _db[27]=' ';_db[28]='P';_db[29]='0';_db[30]=':';
    for(int _i=0;_i<4;_i++){_db[31+_i]="0123456789ABCDEF"[(_p0>>((3-_i)*4))&0xF];}
    _db[35]=' ';_db[36]='O';_db[37]='P';_db[38]=':';
    u8 _raw=vm->bc[_p0];
    u8 _dec=vm->oc_key?(_raw^(u8)(vm->oc_key^((u32)_p0*0x9E3779B9u))):_raw;
    for(int _i=0;_i<2;_i++){_db[39+_i]="0123456789ABCDEF"[(_raw>>((1-_i)*4))&0xF];}
    _db[41]='>';for(int _i=0;_i<2;_i++){_db[42+_i]="0123456789ABCDEF"[(_dec>>((1-_i)*4))&0xF];}
    _db[44]='\n';
    sys_write(1,_db,45);
  }
  {
    u8 _pcbuf[32];
#define _HX(n) ((u8)((n) < 10 ? '0' + (n) : 'A' + (n) - 10))
    _pcbuf[0] = 'P'; _pcbuf[1] = 'C'; _pcbuf[2] = ':';
    _pcbuf[3] = _HX((vm->pc >> 12) & 0xF);
    _pcbuf[4] = _HX((vm->pc >> 8) & 0xF);
    _pcbuf[5] = _HX((vm->pc >> 4) & 0xF);
    _pcbuf[6] = _HX(vm->pc & 0xF);
    _pcbuf[7] = ' '; _pcbuf[8] = 'L'; _pcbuf[9] = 'E'; _pcbuf[10] = 'N'; _pcbuf[11] = ':';
    _pcbuf[12] = _HX((vm->bc_len >> 12) & 0xF);
    _pcbuf[13] = _HX((vm->bc_len >> 8) & 0xF);
    _pcbuf[14] = _HX((vm->bc_len >> 4) & 0xF);
    _pcbuf[15] = _HX(vm->bc_len & 0xF);
    _pcbuf[16] = '\n';
#undef _HX
    sys_write(1, _pcbuf, 17);
  }

  /* ---- Indirect Dispatch main loop ---- */
  VM_DEBUG("[VM] entering main loop...\n");
  for (;;) {
    /* -- Runtime Security Periodic Check -- */
    int sec_res = sec_runtime_check(vm);
    if (__builtin_expect(sec_res != 0, 0)) {
      ret = (u64)sec_res;
      goto cleanup;
    }

    /* -- Reverse/Forward PC positioning -- */
    if (vm->reverse) {
      if (__builtin_expect((i64)vm->pc <= 0, 0))
        break;
      vm->pc--;
      if (__builtin_expect(vm->pc >= vm->bc_len, 0))
        break;
      u8 _sz = vm->bc[vm->pc];
      if (__builtin_expect(_sz > vm->pc, 0))
        break;
      vm->pc -= _sz;
    } else {
      if (__builtin_expect(vm->pc >= vm->bc_len, 0))
        break;
    }

    /* -- OpcodeCryptor decryption -- */
    u8 _raw_op = vm->bc[vm->pc];
    u8 _dec_op = vm->oc_key ? (_raw_op ^ OC_DECRYPT(vm->pc, vm->oc_key)) : _raw_op;

    /* -- Instruction size validation -- */
    u8 _isz = phys_insn_size[_dec_op];
    if (__builtin_expect(_isz == 0 || vm->pc + _isz > vm->bc_len, 0))
      break;

    /* -- Indirect Dispatch: Call function pointer directly from jump table -- */
    {
      u8 _dbgbuf[16];
#define _HX(n) ((u8)((n) < 10 ? '0' + (n) : 'A' + (n) - 10))
      _dbgbuf[0] = _HX((vm->pc >> 12) & 0xF);
      _dbgbuf[1] = _HX((vm->pc >> 8) & 0xF);
      _dbgbuf[2] = _HX((vm->pc >> 4) & 0xF);
      _dbgbuf[3] = _HX(vm->pc & 0xF);
      _dbgbuf[4] = ':';
      _dbgbuf[5] = _HX((_dec_op >> 4) & 0xF);
      _dbgbuf[6] = _HX(_dec_op & 0xF);
      _dbgbuf[7] = '(';
      u8 _lid = inv_map[_dec_op];
      _dbgbuf[8] = (_lid >> 4) < 10 ? '0' + (_lid >> 4) : 'A' + (_lid >> 4) - 10;
      _dbgbuf[9] = (_lid & 0xF) < 10 ? '0' + (_lid & 0xF) : 'A' + (_lid & 0xF) - 10;
      _dbgbuf[10] = ')';
      _dbgbuf[11] = '\n';
#undef _HX
      sys_write(1, _dbgbuf, 12);
    }
    vm_handler_fn _handler = vm_jump_table[_dec_op];
    u32 _step = _handler(vm);
    VM_DEBUG("[VM] handler returned.\n");

    /* -- Check HALT/RET sentinel -- */
    if (__builtin_expect(_step == VM_STEP_HALT || _step == VM_STEP_RET, 0)) {
      ret = vm->R[vm->ret_reg];
      goto cleanup;
    }

    /* -- Advance PC -- */
    /* _step == 0: Branch handler has directly set pc, do not advance */
    /* _step > 0 and not reverse: Normal advancement */
    if (_step > 0 && !vm->reverse) {
      vm->pc += _step;
      VM_DEBUG("[VM] PC advanced.\n");
    }
  }

#else /* !VM_INDIRECT_DISPATCH */

  /* ================================================================
   * Original Computed Goto mode (remains unchanged)
   * ================================================================ */

  /* ---- 3. Computed goto dispatch table (alternative to switch-case, ~20-30% speedup) ---- */
  /* GCC extension: &&label gets label address, goto *ptr jumps */
  /* Note: Use loop to fill defaults to avoid [0...255] range initialization generating implicit memcpy */
  const void *dtab[256];
  u8 phys_insn_size[256];
  for (int _i = 0; _i < 256; _i++) {
    dtab[_i] = &&L_UNKNOWN;
    phys_insn_size[_i] = 0;
  }

  if (op_map) {
    const void *labels[OP_ID_COUNT];
    for (int _i = 0; _i < OP_ID_COUNT; _i++)
      labels[_i] = &&L_UNKNOWN;

    /* System */
    labels[OP_ID_NOP] = &&L_NOP;
    labels[OP_ID_HALT] = &&L_HALT;
    labels[OP_ID_RET] = &&L_RET;
    /* Data Movement */
    labels[OP_ID_MOVIMM] = &&L_MOV_IMM;
    labels[OP_ID_MOVIMM32] = &&L_MOV_IMM32;
    labels[OP_ID_MOVREG] = &&L_MOV_REG;
    /* Memory */
    labels[OP_ID_LOAD8] = &&L_LOAD8;
    labels[OP_ID_LOAD32] = &&L_LOAD32;
    labels[OP_ID_LOAD64] = &&L_LOAD64;
    labels[OP_ID_STORE8] = &&L_STORE8;
    labels[OP_ID_STORE32] = &&L_STORE32;
    labels[OP_ID_STORE64] = &&L_STORE64;
    labels[OP_ID_LOAD16] = &&L_LOAD16;
    labels[OP_ID_STORE16] = &&L_STORE16;
    /* ALU Three-Register */
    labels[OP_ID_ADD] = &&L_ADD;
    labels[OP_ID_SUB] = &&L_SUB;
    labels[OP_ID_MUL] = &&L_MUL;
    labels[OP_ID_XOR] = &&L_XOR;
    labels[OP_ID_AND] = &&L_AND;
    labels[OP_ID_OR] = &&L_OR;
    labels[OP_ID_SHL] = &&L_SHL;
    labels[OP_ID_SHR] = &&L_SHR;
    labels[OP_ID_ASR] = &&L_ASR;
    labels[OP_ID_NOT] = &&L_NOT;
    labels[OP_ID_ROR] = &&L_ROR;
    labels[OP_ID_UMULH] = &&L_UMULH;
    /* ALU Immediate */
    labels[OP_ID_ADDIMM] = &&L_ADD_IMM;
    labels[OP_ID_SUBIMM] = &&L_SUB_IMM;
    labels[OP_ID_XORIMM] = &&L_XOR_IMM;
    labels[OP_ID_ANDIMM] = &&L_AND_IMM;
    labels[OP_ID_ORIMM] = &&L_OR_IMM;
    labels[OP_ID_MULIMM] = &&L_MUL_IMM;
    labels[OP_ID_SHLIMM] = &&L_SHL_IMM;
    labels[OP_ID_SHRIMM] = &&L_SHR_IMM;
    labels[OP_ID_ASRIMM] = &&L_ASR_IMM;
    /* Comparison */
    labels[OP_ID_CMP] = &&L_CMP;
    labels[OP_ID_CMPIMM] = &&L_CMP_IMM;
    /* Branch */
    labels[OP_ID_JMP] = &&L_JMP;
    labels[OP_ID_JE] = &&L_JE;
    labels[OP_ID_JNE] = &&L_JNE;
    labels[OP_ID_JL] = &&L_JL;
    labels[OP_ID_JGE] = &&L_JGE;
    labels[OP_ID_JGT] = &&L_JGT;
    labels[OP_ID_JLE] = &&L_JLE;
    labels[OP_ID_JB] = &&L_JB;
    labels[OP_ID_JAE] = &&L_JAE;
    labels[OP_ID_JBE] = &&L_JBE;
    labels[OP_ID_JA] = &&L_JA;
    /* Stack Operations */
    labels[OP_ID_PUSH] = &&L_PUSH;
    labels[OP_ID_POP] = &&L_POP;
    /* Native Call */
    labels[OP_ID_CALLNATIVE] = &&L_CALL_NAT;
    labels[OP_ID_CALLREG] = &&L_CALL_REG;
    labels[OP_ID_BRREG] = &&L_BR_REG;
    /* SIMD */
    labels[OP_ID_VLD16] = &&L_VLD16;
    labels[OP_ID_VST16] = &&L_VST16;
    /* TBZ/TBNZ */
    labels[OP_ID_TBZ] = &&L_TBZ;
    labels[OP_ID_TBNZ] = &&L_TBNZ;
    /* CCMP/CCMN */
    labels[OP_ID_CCMPREG] = &&L_CCMP_REG;
    labels[OP_ID_CCMPIMM] = &&L_CCMP_IMM;
    labels[OP_ID_CCMNREG] = &&L_CCMN_REG;
    labels[OP_ID_CCMNIMM] = &&L_CCMN_IMM;
    /* SVC */
    labels[OP_ID_SVC] = &&L_SVC;
    /* UDIV/SDIV */
    labels[OP_ID_UDIV] = &&L_UDIV;
    labels[OP_ID_SDIV] = &&L_SDIV;
    /* MRS */
    labels[OP_ID_MRS] = &&L_MRS;
    /* SMULH/CLZ/CLS/RBIT/REV */
    labels[OP_ID_SMULH] = &&L_SMULH;
    labels[OP_ID_CLZ] = &&L_CLZ;
    labels[OP_ID_CLS] = &&L_CLS;
    labels[OP_ID_RBIT] = &&L_RBIT;
    labels[OP_ID_REV] = &&L_REV;
    labels[OP_ID_REV16] = &&L_REV16;
    labels[OP_ID_REV32] = &&L_REV32;
    /* ADC/SBC */
    labels[OP_ID_ADC] = &&L_ADC;
    labels[OP_ID_SBC] = &&L_SBC;

    /* ---- FP ALU ---- */
    labels[OP_ID_SFADD] = &&L_SFADD;
    labels[OP_ID_SFSUB] = &&L_SFSUB;
    labels[OP_ID_SFMUL] = &&L_SFMUL;
    labels[OP_ID_SFDIV] = &&L_SFDIV;
    labels[OP_ID_SFMOV] = &&L_SFMOV;
    labels[OP_ID_SFCMP] = &&L_SFCMP;
    labels[OP_ID_SFNEG] = &&L_SFNEG;
    labels[OP_ID_SFABS] = &&L_SFABS;
    labels[OP_ID_SFSQRT] = &&L_SFSQRT;
    labels[OP_ID_SFMAX] = &&L_SFMAX;
    labels[OP_ID_SFMIN] = &&L_SFMIN;
    labels[OP_ID_SFCVTIF] = &&L_SFCVTIF;
    labels[OP_ID_SFCVTFI] = &&L_SFCVTFI;
    labels[OP_ID_SFMOVRV] = &&L_SFMOVRV;
    labels[OP_ID_SFMOVVR] = &&L_SFMOVVR;
    labels[OP_ID_SFCVT] = &&L_SFCVT;
    labels[OP_ID_SDECRYPTSTR] = &&L_S_DECRYPT_STR;

    /* ---- Stack Machine Opcodes ---- */
    labels[OP_ID_SVLOAD] = &&L_S_VLOAD;
    labels[OP_ID_SVSTORE] = &&L_S_VSTORE;
    labels[OP_ID_SPUSHIMM32] = &&L_S_PUSH32;
    labels[OP_ID_SPUSHIMM64] = &&L_S_PUSH64;
    labels[OP_ID_SDUP] = &&L_S_DUP;
    labels[OP_ID_SSWAP] = &&L_S_SWAP;
    labels[OP_ID_SDROP] = &&L_S_DROP;
    labels[OP_ID_SADD] = &&L_S_ADD;
    labels[OP_ID_SSUB] = &&L_S_SUB;
    labels[OP_ID_SMUL] = &&L_S_MUL;
    labels[OP_ID_SXOR] = &&L_S_XOR;
    labels[OP_ID_SAND] = &&L_S_AND;
    labels[OP_ID_SOR] = &&L_S_OR;
    labels[OP_ID_SSHL] = &&L_S_SHL;
    labels[OP_ID_SSHR] = &&L_S_SHR;
    labels[OP_ID_SASR] = &&L_S_ASR;
    labels[OP_ID_SNOT] = &&L_S_NOT;
    labels[OP_ID_SNEG] = &&L_S_NEG;
    labels[OP_ID_SROR] = &&L_S_ROR;
    labels[OP_ID_SUMULH] = &&L_S_UMULH;
    labels[OP_ID_SSMULH] = &&L_S_SMULH;
    labels[OP_ID_SUDIV] = &&L_S_UDIV;
    labels[OP_ID_SSDIV] = &&L_S_SDIV;
    labels[OP_ID_SADC] = &&L_S_ADC;
    labels[OP_ID_SSBC] = &&L_S_SBC;
    labels[OP_ID_SCLZ] = &&L_S_CLZ;
    labels[OP_ID_SCLS] = &&L_S_CLS;
    labels[OP_ID_SRBIT] = &&L_S_RBIT;
    labels[OP_ID_SREV] = &&L_S_REV;
    labels[OP_ID_SREV16] = &&L_S_REV16;
    labels[OP_ID_SREV32] = &&L_S_REV32;
    labels[OP_ID_STRUNC32] = &&L_S_TRUNC32;
    labels[OP_ID_SSEXT32] = &&L_S_SEXT32;
    labels[OP_ID_SCMP] = &&L_S_CMP;
    labels[OP_ID_SLD8] = &&L_S_LD8;
    labels[OP_ID_SLD16] = &&L_S_LD16;
    labels[OP_ID_SLD32] = &&L_S_LD32;
    labels[OP_ID_SLD64] = &&L_S_LD64;
    labels[OP_ID_SST8] = &&L_S_ST8;
    labels[OP_ID_SST16] = &&L_S_ST16;
    labels[OP_ID_SST32] = &&L_S_ST32;
    labels[OP_ID_SST64] = &&L_S_ST64;
    labels[OP_ID_SLOADSLIDE] = &&L_S_LDSIDE;
    labels[OP_ID_SVLD] = &&L_SVLD;
    labels[OP_ID_SVST] = &&L_SVST;
    labels[OP_ID_SDECRYPTSTR] = &&L_S_DECRYPT_STR;

    for (int i = 0; i < OP_ID_COUNT; i++) {
      dtab[op_map[i]] = labels[i];
      phys_insn_size[op_map[i]] = vm_logical_insn_size(i);
    }
  } else {
    /* Fallback (no op_map): use hardcoded OP_xxx values as indices */
    /* System */
    dtab[OP_NOP] = &&L_NOP;
    dtab[OP_HALT] = &&L_HALT;
    dtab[OP_RET] = &&L_RET;
    /* Data Movement */
    dtab[OP_MOV_IMM] = &&L_MOV_IMM;
    dtab[OP_MOV_IMM32] = &&L_MOV_IMM32;
    dtab[OP_MOV_REG] = &&L_MOV_REG;
    /* Memory */
    dtab[OP_LOAD8] = &&L_LOAD8;
    dtab[OP_LOAD32] = &&L_LOAD32;
    dtab[OP_LOAD64] = &&L_LOAD64;
    dtab[OP_STORE8] = &&L_STORE8;
    dtab[OP_STORE32] = &&L_STORE32;
    dtab[OP_STORE64] = &&L_STORE64;
    dtab[OP_LOAD16] = &&L_LOAD16;
    dtab[OP_STORE16] = &&L_STORE16;
    /* ALU Three-Register */
    dtab[OP_ADD] = &&L_ADD;
    dtab[OP_SUB] = &&L_SUB;
    dtab[OP_MUL] = &&L_MUL;
    dtab[OP_XOR] = &&L_XOR;
    dtab[OP_AND] = &&L_AND;
    dtab[OP_OR] = &&L_OR;
    dtab[OP_SHL] = &&L_SHL;
    dtab[OP_SHR] = &&L_SHR;
    dtab[OP_ASR] = &&L_ASR;
    dtab[OP_NOT] = &&L_NOT;
    dtab[OP_ROR] = &&L_ROR;
    dtab[OP_UMULH] = &&L_UMULH;
    /* ALU Immediate */
    dtab[OP_ADD_IMM] = &&L_ADD_IMM;
    dtab[OP_SUB_IMM] = &&L_SUB_IMM;
    dtab[OP_XOR_IMM] = &&L_XOR_IMM;
    dtab[OP_AND_IMM] = &&L_AND_IMM;
    dtab[OP_OR_IMM] = &&L_OR_IMM;
    dtab[OP_MUL_IMM] = &&L_MUL_IMM;
    dtab[OP_SHL_IMM] = &&L_SHL_IMM;
    dtab[OP_SHR_IMM] = &&L_SHR_IMM;
    dtab[OP_ASR_IMM] = &&L_ASR_IMM;
    /* Comparison */
    dtab[OP_CMP] = &&L_CMP;
    dtab[OP_CMP_IMM] = &&L_CMP_IMM;
    /* Branch */
    dtab[OP_JMP] = &&L_JMP;
    dtab[OP_JE] = &&L_JE;
    dtab[OP_JNE] = &&L_JNE;
    dtab[OP_JL] = &&L_JL;
    dtab[OP_JGE] = &&L_JGE;
    dtab[OP_JGT] = &&L_JGT;
    dtab[OP_JLE] = &&L_JLE;
    dtab[OP_JB] = &&L_JB;
    dtab[OP_JAE] = &&L_JAE;
    dtab[OP_JBE] = &&L_JBE;
    dtab[OP_JA] = &&L_JA;
    /* Stack Operations */
    dtab[OP_PUSH] = &&L_PUSH;
    dtab[OP_POP] = &&L_POP;
    /* Native Call */
    dtab[OP_CALL_NAT] = &&L_CALL_NAT;
    dtab[OP_CALL_REG] = &&L_CALL_REG;
    dtab[OP_BR_REG] = &&L_BR_REG;
    /* SIMD */
    dtab[OP_VLD16] = &&L_VLD16;
    dtab[OP_VST16] = &&L_VST16;
    /* TBZ/TBNZ */
    dtab[OP_TBZ] = &&L_TBZ;
    dtab[OP_TBNZ] = &&L_TBNZ;
    /* CCMP/CCMN */
    dtab[OP_CCMP_REG] = &&L_CCMP_REG;
    dtab[OP_CCMP_IMM] = &&L_CCMP_IMM;
    dtab[OP_CCMN_REG] = &&L_CCMN_REG;
    dtab[OP_CCMN_IMM] = &&L_CCMN_IMM;
    /* SVC */
    dtab[OP_SVC] = &&L_SVC;
    /* UDIV/SDIV */
    dtab[OP_UDIV] = &&L_UDIV;
    dtab[OP_SDIV] = &&L_SDIV;
    /* MRS */
    dtab[OP_MRS] = &&L_MRS;

    /* ---- FP ALU ---- */
    dtab[OP_SFADD] = &&L_SFADD;
    dtab[OP_SFSUB] = &&L_SFSUB;
    dtab[OP_SFMUL] = &&L_SFMUL;
    dtab[OP_SFDIV] = &&L_SFDIV;
    dtab[OP_SFMOV] = &&L_SFMOV;
    dtab[OP_SFCMP] = &&L_SFCMP;
    dtab[OP_SFNEG] = &&L_SFNEG;
    dtab[OP_SFABS] = &&L_SFABS;
    dtab[OP_SFSQRT] = &&L_SFSQRT;
    dtab[OP_SFMAX] = &&L_SFMAX;
    dtab[OP_SFMIN] = &&L_SFMIN;
    dtab[OP_SFCVTIF] = &&L_SFCVTIF;
    dtab[OP_SFCVTFI] = &&L_SFCVTFI;
    dtab[OP_SFMOVRV] = &&L_SFMOVRV;
    dtab[OP_SFMOVVR] = &&L_SFMOVVR;
    dtab[OP_SFCVT] = &&L_SFCVT;

    /* ---- Stack Machine Opcodes ---- */
    dtab[OP_S_VLOAD] = &&L_S_VLOAD;
    dtab[OP_S_VSTORE] = &&L_S_VSTORE;
    dtab[OP_S_PUSH_IMM32] = &&L_S_PUSH32;
    dtab[OP_S_PUSH_IMM64] = &&L_S_PUSH64;
    dtab[OP_S_DUP] = &&L_S_DUP;
    dtab[OP_S_SWAP] = &&L_S_SWAP;
    dtab[OP_S_DROP] = &&L_S_DROP;
    dtab[OP_S_ADD] = &&L_S_ADD;
    dtab[OP_S_SUB] = &&L_S_SUB;
    dtab[OP_S_MUL] = &&L_S_MUL;
    dtab[OP_S_XOR] = &&L_S_XOR;
    dtab[OP_S_AND] = &&L_S_AND;
    dtab[OP_S_OR] = &&L_S_OR;
    dtab[OP_S_SHL] = &&L_S_SHL;
    dtab[OP_S_SHR] = &&L_S_SHR;
    dtab[OP_S_ASR] = &&L_S_ASR;
    dtab[OP_S_NOT] = &&L_S_NOT;
    dtab[OP_S_NEG] = &&L_S_NEG;
    dtab[OP_S_ROR] = &&L_S_ROR;
    dtab[OP_S_UMULH] = &&L_S_UMULH;
    dtab[OP_S_SMULH] = &&L_S_SMULH;
    dtab[OP_S_UDIV] = &&L_S_UDIV;
    dtab[OP_S_SDIV] = &&L_S_SDIV;
    dtab[OP_S_ADC] = &&L_S_ADC;
    dtab[OP_S_SBC] = &&L_S_SBC;
    dtab[OP_S_CLZ] = &&L_S_CLZ;
    dtab[OP_S_CLS] = &&L_S_CLS;
    dtab[OP_S_RBIT] = &&L_S_RBIT;
    dtab[OP_S_REV] = &&L_S_REV;
    dtab[OP_S_REV16] = &&L_S_REV16;
    dtab[OP_S_REV32] = &&L_S_REV32;
    dtab[OP_S_TRUNC32] = &&L_S_TRUNC32;
    dtab[OP_S_SEXT32] = &&L_S_SEXT32;
    dtab[OP_S_CMP] = &&L_S_CMP;
    dtab[OP_S_LD8] = &&L_S_LD8;
    dtab[OP_S_LD16] = &&L_S_LD16;
    dtab[OP_S_LD32] = &&L_S_LD32;
    dtab[OP_S_LD64] = &&L_S_LD64;
    dtab[OP_S_ST8] = &&L_S_ST8;
    dtab[OP_S_ST16] = &&L_S_ST16;
    dtab[OP_S_ST32] = &&L_S_ST32;
    dtab[OP_S_ST64] = &&L_S_ST64;
    dtab[OP_S_LOAD_SLIDE] = &&L_S_LDSIDE;
    dtab[OP_SVLD] = &&L_SVLD;
    dtab[OP_SVST] = &&L_SVST;
    dtab[OP_S_DECRYPT_STR] = &&L_S_DECRYPT_STR;

    for (int i = 0; i < 256; i++)
      phys_insn_size[i] = vm_insn_size(i);
  }

/* Reverse mode: pc points after the size marker at the end of instruction
 * Steps: pc--; size = bc[pc]; pc -= size; now pc points to instruction start */
#define DISPATCH()                                                             \
  do {                                                                         \
    int _sec_res = sec_runtime_check(vm);                                      \
    if (__builtin_expect(_sec_res != 0, 0)) {                                  \
      ret = (u64)_sec_res;                                                     \
      goto cleanup;                                                            \
    }                                                                          \
    if (vm->reverse) {                                                         \
      if (__builtin_expect((i64)vm->pc <= 0, 0))                               \
        goto cleanup;                                                          \
      vm->pc--;                                                                \
      if (__builtin_expect(vm->pc >= vm->bc_len, 0))                           \
        goto cleanup;                                                          \
      u8 _sz = vm->bc[vm->pc];                                                 \
      if (__builtin_expect(_sz > vm->pc, 0))                                   \
        goto cleanup;                                                          \
      vm->pc -= _sz;                                                           \
    } else {                                                                   \
      if (__builtin_expect(vm->pc >= vm->bc_len, 0))                           \
        goto cleanup;                                                          \
    }                                                                          \
    u8 _raw_op = vm->bc[vm->pc];                                               \
    u8 _dec_op = vm->oc_key ? (_raw_op ^ OC_DECRYPT(vm->pc, vm->oc_key)) : _raw_op;\
    u8 _isz = phys_insn_size[_dec_op];                                         \
    if (__builtin_expect(_isz == 0 || vm->pc + _isz > vm->bc_len, 0))          \
      goto cleanup;                                                            \
    goto *dtab[_dec_op];                                                       \
  } while (0)

/* NEXT: handler must always execute; forward pc += n, reverse ignores advance */
#define NEXT(n)                                                                \
  do {                                                                         \
    u32 _adv = (n);                                                            \
    __asm__ volatile("" ::: "memory");                                         \
    if (!vm->reverse)                                                          \
      vm->pc += _adv;                                                          \
    DISPATCH();                                                                \
  } while (0)
#define NEXT0() DISPATCH() /* handler has set pc */

  /* ---- PC initialization: reverse mode starts from bc_len ---- */
  if (vm->reverse) {
    vm->pc = vm->bc_len; /* DISPATCH will first decrement to locate the last instruction */
  }

  /* ---- Start Execution ---- */
  VM_DEBUG("[VM] starting dispatch...\n");
  DISPATCH();

/* ---- System ---- */
L_NOP:
  NEXT(h_nop(vm));
L_HALT:
  ret = vm->R[vm->ret_reg];
  goto cleanup;
L_RET: {
  ret = vm->R[vm->ret_reg];
  goto cleanup;
}

/* ---- Data Movement ---- */
L_MOV_IMM:
  NEXT(h_mov_imm(vm));
L_MOV_IMM32:
  NEXT(h_mov_imm32(vm));
L_MOV_REG:
  NEXT(h_mov_reg(vm));

/* ---- Memory Access ---- */
L_LOAD8:
  NEXT(h_load8(vm));
L_LOAD32:
  NEXT(h_load32(vm));
L_LOAD64:
  NEXT(h_load64(vm));
L_STORE8:
  NEXT(h_store8(vm));
L_STORE32:
  NEXT(h_store32(vm));
L_STORE64:
  NEXT(h_store64(vm));
L_LOAD16:
  NEXT(h_load16(vm));
L_STORE16:
  NEXT(h_store16(vm));

/* ---- ALU Three-Register ---- */
L_ADD:
  NEXT(h_add(vm));
L_SUB:
  NEXT(h_sub(vm));
L_MUL:
  NEXT(h_mul(vm));
L_XOR:
  NEXT(h_xor(vm));
L_AND:
  NEXT(h_and(vm));
L_OR:
  NEXT(h_or(vm));
L_SHL:
  NEXT(h_shl(vm));
L_SHR:
  NEXT(h_shr(vm));
L_ASR:
  NEXT(h_asr(vm));
L_NOT:
  NEXT(h_not(vm));
L_ROR:
  NEXT(h_ror(vm));
L_UMULH:
  NEXT(h_umulh(vm));

/* ---- ALU Immediate ---- */
L_ADD_IMM:
  NEXT(h_add_imm(vm));
L_SUB_IMM:
  NEXT(h_sub_imm(vm));
L_XOR_IMM:
  NEXT(h_xor_imm(vm));
L_AND_IMM:
  NEXT(h_and_imm(vm));
L_OR_IMM:
  NEXT(h_or_imm(vm));
L_MUL_IMM:
  NEXT(h_mul_imm(vm));
L_SHL_IMM:
  NEXT(h_shl_imm(vm));
L_SHR_IMM:
  NEXT(h_shr_imm(vm));
L_ASR_IMM:
  NEXT(h_asr_imm(vm));

/* ---- Comparison ---- */
L_CMP:
  NEXT(h_cmp(vm));
L_CMP_IMM:
  NEXT(h_cmp_imm(vm));

/* ---- Branch (handler returns 0, pc already set) ---- */
L_JMP:
  h_jmp(vm);
  NEXT0();
L_JE:
  h_je(vm);
  NEXT0();
L_JNE:
  h_jne(vm);
  NEXT0();
L_JL:
  h_jl(vm);
  NEXT0();
L_JGE:
  h_jge(vm);
  NEXT0();
L_JGT:
  h_jgt(vm);
  NEXT0();
L_JLE:
  h_jle(vm);
  NEXT0();
L_JB:
  h_jb(vm);
  NEXT0();
L_JAE:
  h_jae(vm);
  NEXT0();
L_JBE:
  h_jbe(vm);
  NEXT0();
L_JA:
  h_ja(vm);
  NEXT0();

/* ---- Stack Operations ---- */
L_PUSH:
  NEXT(h_push(vm));
L_POP:
  NEXT(h_pop(vm));

/* ---- Native Call ---- */
L_CALL_NAT:
  NEXT(h_call_nat(vm));
L_CALL_REG:
  NEXT(h_call_reg(vm));
L_BR_REG: {
  u32 a = h_br_reg(vm);
  if (a)
    NEXT(a);
  else
    NEXT0();
}

/* ---- SIMD ---- */
L_VLD16:
  NEXT(h_vld16(vm));
L_VST16:
  NEXT(h_vst16(vm));

/* ---- TBZ/TBNZ (branch, handler returns 0, pc already set) ---- */
L_TBZ:
  h_tbz(vm);
  NEXT0();
L_TBNZ:
  h_tbnz(vm);
  NEXT0();

/* ---- CCMP/CCMN ---- */
L_CCMP_REG:
  NEXT(h_ccmp_reg(vm));
L_CCMP_IMM:
  NEXT(h_ccmp_imm(vm));
L_CCMN_REG:
  NEXT(h_ccmn_reg(vm));
L_CCMN_IMM:
  NEXT(h_ccmn_imm(vm));

/* ---- SVC ---- */
L_SVC:
  NEXT(h_svc(vm));

/* ---- UDIV/SDIV ---- */
L_UDIV:
  NEXT(h_udiv(vm));
L_SDIV:
  NEXT(h_sdiv(vm));

/* ---- MRS ---- */
L_MRS:
  NEXT(h_mrs(vm));

/* ---- FP ALU ---- */
L_SFADD:
  NEXT(h_fadd(vm));
L_SFSUB:
  NEXT(h_fsub(vm));
L_SFMUL:
  NEXT(h_fmul(vm));
L_SFDIV:
  NEXT(h_fdiv(vm));
L_SFMOV:
  NEXT(h_fmov(vm));
L_SFCMP:
  NEXT(h_fcmp(vm));
L_SFNEG:
  NEXT(h_fneg(vm));
L_SFABS:
  NEXT(h_fabs(vm));
L_SFSQRT:
  NEXT(h_fsqrt(vm));
L_SFMAX:
  NEXT(h_fmax(vm));
L_SFMIN:
  NEXT(h_fmin(vm));
L_SFCVTIF:
  NEXT(h_fcvt_if(vm));
L_SFCVTFI:
  NEXT(h_fcvt_fi(vm));
L_SFMOVRV:
  NEXT(h_fmov_rv(vm));
L_SFMOVVR:
  NEXT(h_fmov_vr(vm));
L_SFCVT:
  NEXT(h_fcvt(vm));

/* ---- Stack Machine ---- */
L_S_PUSH32:
  NEXT(h_s_push_imm32(vm));
L_S_PUSH64:
  NEXT(h_s_push_imm64(vm));
L_S_VLOAD:
  NEXT(h_s_vload(vm));
L_S_VSTORE:
  NEXT(h_s_vstore(vm));
L_S_ADD:
  NEXT(h_s_add(vm));
L_S_SUB:
  NEXT(h_s_sub(vm));
L_S_MUL:
  NEXT(h_s_mul(vm));
L_S_XOR:
  NEXT(h_s_xor(vm));
L_S_AND:
  NEXT(h_s_and(vm));
L_S_OR:
  NEXT(h_s_or(vm));
L_S_SHL:
  NEXT(h_s_shl(vm));
L_S_SHR:
  NEXT(h_s_shr(vm));
L_S_ASR:
  NEXT(h_s_asr(vm));
L_S_NOT:
  NEXT(h_s_not(vm));
L_S_NEG:
  NEXT(h_s_neg(vm));
L_S_DUP:
  NEXT(h_s_dup(vm));
L_S_SWAP:
  NEXT(h_s_swap(vm));
L_S_DROP:
  NEXT(h_s_drop(vm));
L_S_LDSIDE:
  NEXT(h_s_load_slide(vm));
L_S_LD8:
  NEXT(h_s_ld8(vm));
L_S_LD16:
  NEXT(h_s_ld16(vm));
L_S_LD32:
  NEXT(h_s_ld32(vm));
L_S_LD64:
  NEXT(h_s_ld64(vm));
L_S_ST8:
  NEXT(h_s_st8(vm));
L_S_ST16:
  NEXT(h_s_st16(vm));
L_S_ST32:
  NEXT(h_s_st32(vm));
L_S_ST64:
  NEXT(h_s_st64(vm));
L_SVLD:
  NEXT(h_s_vld(vm));
L_SVST:
  NEXT(h_s_vst(vm));
L_S_ROR:
  NEXT(h_s_ror(vm));
L_S_UMULH:
  NEXT(h_s_umulh(vm));
L_S_SMULH:
  NEXT(h_s_smulh(vm));
L_S_UDIV:
  NEXT(h_s_udiv(vm));
L_S_SDIV:
  NEXT(h_s_sdiv(vm));
L_S_ADC:
  NEXT(h_s_adc(vm));
L_S_SBC:
  NEXT(h_s_sbc(vm));
L_S_CLZ:
  NEXT(h_s_clz(vm));
L_S_CLS:
  NEXT(h_s_cls(vm));
L_S_RBIT:
  NEXT(h_s_rbit(vm));
L_S_REV:
  NEXT(h_s_rev(vm));
L_S_REV16:
  NEXT(h_s_rev16(vm));
L_S_REV32:
  NEXT(h_s_rev32(vm));
L_S_TRUNC32:
  NEXT(h_s_trunc32(vm));
L_S_SEXT32:
  NEXT(h_s_sext32(vm));
L_S_CMP:
  NEXT(h_s_cmp(vm));
L_S_DECRYPT_STR:
  NEXT(h_s_decrypt_str(vm));

/* ---- Unknown Instruction ---- */
L_UNKNOWN:
  ret = vm->R[0]; /* fall through to cleanup */

#undef DISPATCH
#undef NEXT
#undef NEXT0

#endif /* VM_INDIRECT_DISPATCH */

  /* ---- Unified Exit: Release mmap to prevent leaks ---- */
cleanup:
  /* ---- Anti-Tampering: Buffer Zeroing ---- */
  if (vm) sec_zero_memory(vm, ctx_alloc);
  if (bc_buf) sec_zero_memory(bc_buf, alloc_size);
  /* sys_munmap(vm, ctx_alloc); */
  /* sys_munmap(bc_buf, alloc_size); */
  return ret;
}
