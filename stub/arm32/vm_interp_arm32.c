/*
 * vm_interp_arm32.c — ARM32 VM interpreter stub
 *
 * This is the ARM32 version of the VM interpreter, adapted from the ARM64 version.
 * Key differences:
 *   - ARM32 syscall ABI: R7 = syscall number, SVC #0
 *   - ARM32 AAPCS: R0-R3 for args, R4-R11 callee-saved
 *   - Compiled with -mthumb-interwork for ARM/Thumb compatibility
 *   - 32-bit native_fn_t
 *   - Uses mmap2 (NR=192) instead of mmap (NR=222)
 *
 * Build:
 *   arm-linux-gnueabihf-gcc -c -Os -marm -mthumb-interwork -fno-stack-protector \
 *     -fno-builtin -nostdlib -march=armv7-a \
 *     -DVM_INDIRECT_DISPATCH -DVM_FUNC_SPLIT -DVM_TOKEN_ENTRY \
 *     vm_interp_arm32.c -o vm_interp_arm32.o
 */

#include "vm_types_arm32.h"
#include "vm_decode.h"
#include "vm_opcodes.h"

/* Include instruction handlers (shared with ARM64 — they operate on vm_ctx_t) */
#include "vm_handlers/h_alu.h"
#include "vm_handlers/h_branch.h"
#include "vm_handlers/h_cmp.h"
#include "vm_handlers/h_mem.h"
#include "vm_handlers/h_mov.h"
#include "vm_handlers/h_stack.h"
#include "vm_handlers/h_stack_ops.h"
#include "vm_handlers/h_system.h"

#ifdef VM_INDIRECT_DISPATCH
#include "vm_dispatch.h"
#endif

#include "vm_token.h"

/* ARM32 packer writes 8 bytes/entry (bc_off u32 + bc_len u32), not 16 like token_desc_t */
typedef struct { u32 bc_off; u32 bc_len; } token_desc_arm32_t;

/* ---- ARM32 syscall numbers ---- */
#define ARM32_NR_WRITE 4

#ifdef VM_DEBUG
static void debug_char(char c) {
  register long r7 __asm__("r7") = ARM32_NR_WRITE;
  register long r0 __asm__("r0") = 2;
  register long r1 __asm__("r1") = (long)&c;
  register long r2 __asm__("r2") = 1;
  __asm__ volatile("svc #0" : "+r"(r0) : "r"(r7), "r"(r1), "r"(r2) : "memory");
}
#define DBG(c) do { char _d = (c); debug_char(_d); } while(0)
#else
#define DBG(c) ((void)0)
#endif

/* ---- ARM32 syscall: mmap2 ---- */
static inline void *sys_mmap_arm32(unsigned long size) {
  register long r7 __asm__("r7") = ARM32_NR_MMAP2;
  register long r0 __asm__("r0") = 0;
  register long r1 __asm__("r1") = (long)size;
  register long r2 __asm__("r2") = 3;    /* PROT_READ | PROT_WRITE */
  register long r3 __asm__("r3") = 0x22; /* MAP_PRIVATE | MAP_ANONYMOUS */
  register long r4 __asm__("r4") = -1;   /* fd = -1 */
  register long r5 __asm__("r5") = 0;    /* offset (in pages for mmap2) */
  __asm__ volatile("svc #0"
                   : "+r"(r0)
                   : "r"(r7), "r"(r1), "r"(r2), "r"(r3), "r"(r4), "r"(r5)
                   : "memory");
  return (void *)r0;
}

/* ---- ARM32 syscall: munmap ---- */
static inline void sys_munmap_arm32(void *addr, unsigned long size) {
  register long r7 __asm__("r7") = ARM32_NR_MUNMAP;
  register long r0 __asm__("r0") = (long)addr;
  register long r1 __asm__("r1") = (long)size;
  __asm__ volatile("svc #0" : "+r"(r0) : "r"(r7), "r"(r1) : "memory");
}

/* ---- VM entry point ---- */
__attribute__((section(".text.entry")))
u64 vm_entry(u64 *args, u8 *enc_bc, u32 bc_len, u8 xor_key, u64 slide);

/* get_self_va: returns runtime address of _token_table_va (PIE-safe via ADR) */
extern u32 get_self_va(void);

/* ---- Token entry inner function ---- */
__attribute__((noinline, section(".text.entry")))
u64 vm_entry_token_inner(u32 *args, u32 token) {
  DBG('1'); /* inner start */
  u8 xor_key = (u8)TOKEN_XOR_KEY(token);
  u32 func_id = TOKEN_FUNC_ID(token);

  /* PIE: get_self_va() returns &_token_table_va; read value via pointer to avoid
   * absolute-address load (stub not -fPIC, would fault at link-time addr) */
  u32 self_va = get_self_va();
  DBG('2'); /* after get_self_va */
  u32 tbl_off = *(const u32 *)self_va;
  if (__builtin_expect(tbl_off == 0, 0))
    return 0;
  DBG('3'); /* tbl_off ok */

  /* Compute ASLR slide: _link_time_self_va is the word right after _token_table_va */
  u32 link_time_self = *(const u32 *)(self_va + 4);
  u64 slide = (link_time_self != 0) ? (u64)(self_va - link_time_self) : 0;

  token_desc_arm32_t *table = (token_desc_arm32_t *)(self_va + tbl_off);
  u8 *enc_bc = (u8 *)(self_va + table[func_id].bc_off);
  u32 bc_len = table[func_id].bc_len;
  DBG('4'); /* table lookup ok */
  if (__builtin_expect(enc_bc == (u8 *)self_va || bc_len == 0, 0))
    return 0;
  DBG('5'); /* before vm_entry */
  /* Promote args to u64 array for vm_entry compatibility.
   * Stack layout after push {r0-r12, lr}: 14 words.
   * args[0..12] = R0..R12, args[13] = LR. */
  u64 args64[14];
  for (int i = 0; i < 14; i++)
    args64[i] = (u64)args[i];

  return vm_entry(args64, enc_bc, bc_len, xor_key, slide);
}

/*
 * Naked assembly entry: saves ALL ARM32 general registers R0-R12 + LR,
 * passes saved register block and token (R12/IP) to C inner function.
 * 14 registers = 56 bytes, maintains 8-byte stack alignment.
 * Works for both ARM and Thumb callers via BX LR return.
 */
__attribute__((naked, section(".text.entry"), used))
void vm_entry_token(void) {
  __asm__ volatile(
      "push {r0-r12, lr}\n"
      "mov r0, sp\n"           /* R0 = pointer to saved registers (14 words) */
      "mov r1, r12\n"          /* R1 = token (passed via R12/IP) */
      "bl vm_entry_token_inner\n"
      "str r0, [sp]\n"         /* overwrite saved r0 with return value */
      "pop {r0-r12, lr}\n"
      "bx lr\n"                /* interwork-safe return */
  );
}

/* ---- vm_entry implementation ---- */
__attribute__((section(".text.entry")))
u64 vm_entry(u64 *args, u8 *enc_bc, u32 bc_len, u8 xor_key, u64 slide) {
  DBG('6'); /* vm_entry start */
  u64 ret = 0;

  if (bc_len > VM_BYTECODE_MAX)
    bc_len = VM_BYTECODE_MAX;
  u32 alloc_size = (bc_len + 4095u) & ~4095u;
  u8 *bc_buf = (u8 *)sys_mmap_arm32(alloc_size);
  DBG('7'); /* after bc mmap */
  if ((unsigned long)bc_buf >= 0xFFFFF000u)
    return 0;

  /* XOR decrypt (4-byte wide for ARM32) */
  u32 xk4 = (u32)xor_key;
  xk4 |= xk4 << 8;
  xk4 |= xk4 << 16;
  {
    u32 n4 = bc_len >> 2;
    u32 *d4 = (u32 *)bc_buf;
    const u32 *s4 = (const u32 *)enc_bc;
    for (u32 i = 0; i < n4; i++)
      d4[i] = s4[i] ^ xk4;
    for (u32 i = n4 << 2; i < bc_len; i++)
      bc_buf[i] = enc_bc[i] ^ xor_key;
  }

  /* Allocate VM context via mmap */
  u32 ctx_alloc = (sizeof(vm_ctx_t) + 4095u) & ~4095u;
  vm_ctx_t *vm = (vm_ctx_t *)sys_mmap_arm32(ctx_alloc);
  DBG('8'); /* after ctx mmap */
  if ((unsigned long)vm >= 0xFFFFF000u) {
    sys_munmap_arm32(bc_buf, alloc_size);
    return 0;
  }

  /* Initialize context: args[0..12] = R0..R12, args[13] = LR (R14) */
  for (int i = 0; i < VM_REG_COUNT; i++)
    vm->R[i] = 0;
  for (int i = 0; i < 13 && i < VM_REG_COUNT; i++)
    vm->R[i] = args[i];
  vm->R[ARM32_LR] = args[13];
  vm->R[ARM32_SP] = (u64)&vm->vm_stk[VM_MEM_STACK];
  vm->bc = bc_buf;
  vm->bc_len = bc_len;
  vm->FL = 0;
  vm->pc = 0;
  vm->sp = 0;
  vm->eval_sp = -1;
  vm->func_addr = 0;
  vm->func_size = 0;
  vm->addr_map = 0;
  vm->map_count = 0;
  vm->oc_key = 0;
  vm->reverse = 0;
  vm->slide = slide;

  /* Parse trailer (same format as ARM64) */
  if (bc_len >= 21) {
    u32 trail_func_size = rd32(&bc_buf[bc_len - 4]);
    u64 trail_func_addr = rd64(&bc_buf[bc_len - 12]);
    u32 trail_map_count = rd32(&bc_buf[bc_len - 16]);
    u32 trail_oc_key = rd32(&bc_buf[bc_len - 20]);
    u8  trail_reverse = bc_buf[bc_len - 21];
    u32 map_data_size = trail_map_count * 8 + 21;

    vm->oc_key = trail_oc_key;
    vm->reverse = trail_reverse;

    if (trail_func_addr != 0 && trail_map_count > 0 &&
        map_data_size <= bc_len) {
      vm->func_addr = trail_func_addr;
      vm->func_size = trail_func_size;
      vm->map_count = trail_map_count;
      vm->addr_map = (addr_map_entry_t *)&bc_buf[bc_len - map_data_size];
      vm->bc_len = bc_len - map_data_size;

      /* Insertion sort addr_map by arm64_off (field name kept for compatibility) */
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
      vm->bc_len = bc_len - 21;
    }
  }

/* OpcodeCryptor decrypt macro */
#define OC_DECRYPT(pc, key) ((u8)((key) ^ ((pc) * 0x9E3779B9u)))

#ifdef VM_INDIRECT_DISPATCH
  vm_handler_fn vm_jump_table[256];
  
  if (bc_len >= 21 + 256 && vm->map_count > 0) {
    u8 *op_map = &bc_buf[vm->bc_len];
    vm_handler_fn handlers[OP_ID_COUNT];
    vm_init_jump_table(handlers);
    for (int i = 0; i < 256; i++) {
        u8 id = op_map[i];
        if (id < OP_ID_COUNT) {
            vm_jump_table[i] = handlers[id];
        } else {
            vm_jump_table[i] = handlers[0]; // fallback
        }
    }
  } else {
    vm_init_jump_table(vm_jump_table); // Should not happen, but safe fallback
  }

  DBG('9'); /* before VM loop */

  if (vm->reverse) {
    vm->pc = vm->bc_len;
  }

  for (;;) {
    if (vm->reverse) {
      if (__builtin_expect((i32)vm->pc <= 0, 0))
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

    u8 _raw_op = vm->bc[vm->pc];
    u8 _dec_op = _raw_op ^ OC_DECRYPT(vm->pc, vm->oc_key);
    u8 _isz = vm_insn_size(_dec_op);
    if (__builtin_expect(_isz == 0 || vm->pc + _isz > vm->bc_len, 0))
      break;

    if (_dec_op == OP_HALT) {
      ret = vm->R[0];
      goto cleanup;
    }
    if (_dec_op == OP_RET) {
      u8 _r = vm->bc[vm->pc + 1];
      ret = vm->R[_r & 31];
      goto cleanup;
    }

    vm_handler_fn _handler = vm_jump_table[_dec_op];
    u32 _step = _handler(vm);

    if (__builtin_expect(_step == VM_STEP_HALT, 0)) {
      ret = vm->R[0];
      goto cleanup;
    }

    if (_step > 0 && !vm->reverse) {
      vm->pc += _step;
    }
  }

#else /* Computed goto fallback */
  /* For ARM32, use the indirect dispatch mode only */
  ret = vm->R[0];
#endif

cleanup:
  sys_munmap_arm32(vm, ctx_alloc);
  sys_munmap_arm32(bc_buf, alloc_size);
  return ret;
}
