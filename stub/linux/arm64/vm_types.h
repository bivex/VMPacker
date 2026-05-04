/*
 * vm_types.h — VM type definitions + CPU context struct
 *
 * All VM states are encapsulated in vm_ctx_t for easy passing and extension.
 */
#ifndef VM_TYPES_H
#define VM_TYPES_H

/* ---- Basic Types ---- */
typedef unsigned char u8;
typedef unsigned short u16;
typedef unsigned int u32;
typedef unsigned long long u64;
typedef int i32;
typedef long long i64;
typedef short i16;

/* ---- VM Configuration Constants ---- */
#define VM_REG_COUNT 64        /* X0-X30, X31=SP, 32+=XZR/Temp */
#define VM_STACK_SIZE 64       /* PUSH/POP operation stack depth */
#define VM_EVAL_STACK_SIZE 2048 /* Stack machine evaluation stack depth */
#define VM_MEM_STACK 262144    /* Memory stack (space pointed by SP, 256KB) */
#define VM_BYTECODE_MAX 65536  /* Max bytecode length (64KB, including mapping table) */
#define VM_SIMD_BUF 64         /* SIMD temporary buffer size */

/* ---- Flags (NZCV simplified) ---- */
#define FL_ZERO 1  /* Z: Result is zero */
#define FL_SIGN 2  /* N: Signed less than */
#define FL_CARRY 4 /* C: Unsigned less than */

/* ---- Native Function Pointer Type ---- */
typedef u64 (*native_fn_t)(u64, u64, u64, u64, u64, u64, u64, u64);

/* ---- BR Indirect Jump Mapping Table Entry ---- */
typedef struct {
  u32 arm64_off; /* Offset within ARM64 function */
  u32 vm_off;    /* Corresponding VM bytecode offset */
} addr_map_entry_t;

/* ---- VM CPU Context ---- */
typedef struct {
  /* Register File: R[0]-R[30] = X0-X30, R[31] = SP */
  u64 R[VM_REG_COUNT];

  /* SIMD/FP Registers: V[0]-V[31], 128-bit each (2 x u64) */
  u64 V[32][2] __attribute__((aligned(16)));

  /* Condition Flags */
  u32 FL;

  /* Virtual Program Counter */
  u32 pc;

  /* Bytecode (decrypted) */
  u8 *bc;
  u32 bc_len;

  /* PUSH/POP operation stack (old register-based compatible) */
  u64 stk[VM_STACK_SIZE];
  int sp;

  /* Stack machine evaluation stack (Stack Machine eval stack) */
  u64 eval_stk[VM_EVAL_STACK_SIZE] __attribute__((aligned(16)));
  int eval_sp; /* Stack pointer, 0 = empty */

  u8 ret_reg;   /* Shuffled index of R0 (X0 return value) */
  u8 reverse;   /* Opcode reverse flag */
  u16 padding;
  u32 oc_key;   /* OpcodeCryptor key */

  /* Memory stack (R[31] points to the end of this space) */
  u8 vm_stk[VM_MEM_STACK] __attribute__((aligned(16)));

  /* SIMD temporary buffer */
  u8 vtmp[VM_SIMD_BUF] __attribute__((aligned(16)));


  /* BR indirect jump support */
  u64 func_addr;              /* Original starting address of the protected function */
  u32 func_size;              /* Size of the protected function */
  addr_map_entry_t *addr_map; /* ARM64 offset → VM offset mapping table */
  u32 map_count;              /* Mapping table entry count */

  /* PIE/ASLR: runtime load base slide for CALL_NAT absolute addresses.
   * slide = runtime_base - link_time_base.  0 for ET_EXEC. */
  u64 slide;

  /* Runtime Security: Periodic Integrity Checks */
  u32 expected_bc_crc; /* CRC32 of bytecode (stored in trailer) */
  u32 bc_crc_len;      /* Length of bytecode to CRC check */
  u32 insn_count;      /* Executed instruction counter for periodic checks */
} vm_ctx_t;

/* ---- SP Stack Boundary Check ---- */
/* Check if address is within vm_stk range (only for SP-related access) */
#define VM_STK_LO(vm) ((u64)(vm)->vm_stk)
#define VM_STK_HI(vm) ((u64)(vm)->vm_stk + VM_MEM_STACK)

/* ---- VM Initialization ---- */
static inline void vm_ctx_init(vm_ctx_t *vm, u64 *args, u8 *bytecode, u32 len) {
  /* Zero all registers */
  for (int i = 0; i < VM_REG_COUNT; i++)
    vm->R[i] = 0;

  for (int i = 0; i < 32; i++) {
    vm->V[i][0] = 0;
    vm->V[i][1] = 0;
  }

  /* Restore parameter registers X0-X7 from args pointer */
  for (int i = 0; i < 8; i++)
    vm->R[i] = args[i];

  vm->R[29] = args[8]; /* X29 = caller FP */
  vm->R[30] = args[9]; /* X30 = caller LR */

  /* Restore V0-V7 from args[10..25] (128-bit each = 2x u64) */
  for (int i = 0; i < 8; i++) {
    vm->V[i][0] = args[10 + i * 2];
    vm->V[i][1] = args[10 + i * 2 + 1];
  }

  /* Restore callee-saved X19-X28 from args[26..35] */
  for (int i = 0; i < 10; i++)
    vm->R[19 + i] = args[26 + i];

  /* Set initial SP */

  vm->R[31] = (u64)&vm->vm_stk[VM_MEM_STACK];

  /* Bytecode */
  vm->bc = bytecode;
  vm->bc_len = len;

  /* State initialization */
  vm->FL = 0;
  vm->pc = 0;
  vm->sp = 0;
  vm->eval_sp = 0; /* Stack machine evaluation stack initially empty */

  /* BR indirect jump mapping table: default none */
  vm->func_addr = 0;
  vm->func_size = 0;
  vm->addr_map = 0;
  vm->map_count = 0;

  /* OpcodeCryptor: no encryption by default (key=0 means identity decryption) */
  vm->oc_key = 0;

  /* PC reverse traversal: default forward */
  vm->reverse = 0;

  /* PIE/ASLR slide: default 0 (ET_EXEC) */
  vm->slide = 0;

  /* Security */
  vm->expected_bc_crc = 0;
  vm->bc_crc_len = 0;
  vm->insn_count = 0;
}

#endif /* VM_TYPES_H */

