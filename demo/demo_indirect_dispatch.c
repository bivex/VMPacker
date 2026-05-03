 /*
 * demo_indirect_dispatch.c — Indirect Dispatch Jump Table Mechanism Verification Demo
 *
 * Independent verification: relative offset jump table + indirect function pointer call
 * Does not depend on any stub/ headers; all types and handlers are defined inline.
 *
 * Calculation: 10 * 3 + 7 = 37
 *
 * Bytecode:
 *   MOV_IMM32 R0, 10   → [0x49][0x00][0x0A][0x00][0x00][0x00]  (6B)
 *   MOV_IMM32 R1, 3    → [0x49][0x01][0x03][0x00][0x00][0x00]  (6B)
 *   MUL R0, R0, R1     → [0x83][0x00][0x00][0x01]              (4B)
 *   MOV_IMM32 R1, 7    → [0x49][0x01][0x07][0x00][0x00][0x00]  (6B)
 *   ADD R0, R0, R1     → [0x37][0x00][0x00][0x01]              (4B)
 *   HALT               → [0x00]                                 (1B)
 *
 * Compilation:
 *   aarch64-linux-gnu-gcc -O1 -o demo_indirect_dispatch \
 *       demo/demo_indirect_dispatch.c -nostdlib -static
 *
 * Expected: output "PASS", exit code = 37
 */

/* ---- Basic Types (self-contained, no stub/ dependency) ---- */
typedef unsigned char      u8;
typedef unsigned short     u16;
typedef unsigned int       u32;
typedef unsigned long long u64;
typedef long long          i64;
typedef int                i32;

/* ---- Opcodes ---- */
#define OP_HALT      0x00
#define OP_ADD       0x37
#define OP_SUB       0x6E
#define OP_MUL       0x83
#define OP_MOV_IMM32 0x49
#define OP_NOP       0xC3

/* ---- Minimal VM Context ---- */
#define VM_REG_COUNT 32

typedef struct {
    u64 R[VM_REG_COUNT];
    u32 pc;
    u8  *bc;
    u32 bc_len;
    u8  halted;
} vm_ctx_t;

/* ---- Handler function signature: returns instruction step size in bytes ---- */
typedef u32 (*vm_handler_fn)(vm_ctx_t *vm);

/* ================================================================
 * Handlers — each handler must be __attribute__((noinline))
 * to prevent the compiler from inlining and optimizing away the indirect call mechanism
 * ================================================================ */

__attribute__((noinline))
static u32 h_halt(vm_ctx_t *vm) {
    vm->halted = 1;
    return 1;
}

__attribute__((noinline))
static u32 h_mov_imm32(vm_ctx_t *vm) {
    /* 6B: [op][d][imm32_le] */
    u8 d = vm->bc[vm->pc + 1] & 31;
    u32 imm = (u32)vm->bc[vm->pc + 2]
            | ((u32)vm->bc[vm->pc + 3] << 8)
            | ((u32)vm->bc[vm->pc + 4] << 16)
            | ((u32)vm->bc[vm->pc + 5] << 24);
    vm->R[d] = (u64)imm;
    return 6;
}

__attribute__((noinline))
static u32 h_add(vm_ctx_t *vm) {
    /* 4B: [op][d][a][b] */
    u8 d = vm->bc[vm->pc + 1] & 31;
    u8 a = vm->bc[vm->pc + 2] & 31;
    u8 b = vm->bc[vm->pc + 3] & 31;
    vm->R[d] = vm->R[a] + vm->R[b];
    return 4;
}

__attribute__((noinline))
static u32 h_sub(vm_ctx_t *vm) {
    /* 4B: [op][d][a][b] */
    u8 d = vm->bc[vm->pc + 1] & 31;
    u8 a = vm->bc[vm->pc + 2] & 31;
    u8 b = vm->bc[vm->pc + 3] & 31;
    vm->R[d] = vm->R[a] - vm->R[b];
    return 4;
}

__attribute__((noinline))
static u32 h_mul(vm_ctx_t *vm) {
    /* 4B: [op][d][a][b] */
    u8 d = vm->bc[vm->pc + 1] & 31;
    u8 a = vm->bc[vm->pc + 2] & 31;
    u8 b = vm->bc[vm->pc + 3] & 31;
    vm->R[d] = vm->R[a] * vm->R[b];
    return 4;
}

__attribute__((noinline))
static u32 h_nop(vm_ctx_t *vm) {
    (void)vm;
    return 1;
}

__attribute__((noinline))
static u32 h_unknown(vm_ctx_t *vm) {
    /* Unknown opcode: safe halt */
    vm->halted = 1;
    return 1;
}

/* ================================================================
 * Indirect Dispatch Jump Table
 *
 * Core mechanism:
 *   jump_table[opcode] = (int32_t)((char*)handler - (char*)jump_table)
 *   handler_addr = (char*)jump_table + jump_table[opcode]
 *
 * Use int32_t relative offsets instead of absolute function pointers.
 * ================================================================ */

/* Macro for calculating relative offset at compile time */
#define HANDLER_OFFSET(fn) \
    (i32)((const char *)(fn) - (const char *)vm_jump_table)

/* Jump table base address macro */
#define VM_DISPATCH_BASE ((const char *)vm_jump_table)

/*
 * Jump table: 256 int32_t relative offsets
 *
 * Note: GCC [0 ... 255] range initializers may generate implicit
 * memset/memcpy calls under -nostdlib. For safety, use runtime initialization.
 */
static i32 vm_jump_table[256];

/* Runtime initialization of jump table */
__attribute__((noinline))
static void init_jump_table(void) {
    /* Default: all opcodes point to h_unknown */
    for (int i = 0; i < 256; i++)
        vm_jump_table[i] = HANDLER_OFFSET(h_unknown);

    /* Defined opcodes */
    vm_jump_table[OP_HALT]      = HANDLER_OFFSET(h_halt);
    vm_jump_table[OP_MOV_IMM32] = HANDLER_OFFSET(h_mov_imm32);
    vm_jump_table[OP_ADD]       = HANDLER_OFFSET(h_add);
    vm_jump_table[OP_SUB]       = HANDLER_OFFSET(h_sub);
    vm_jump_table[OP_MUL]       = HANDLER_OFFSET(h_mul);
    vm_jump_table[OP_NOP]       = HANDLER_OFFSET(h_nop);
}

/* ================================================================
 * Dispatch loop — call handler indirectly via jump table
 * ================================================================ */

__attribute__((noinline))
static u64 vm_run(vm_ctx_t *vm) {
    while (!vm->halted && vm->pc < vm->bc_len) {
        u8 opcode = vm->bc[vm->pc];

        /* Read relative offset from jump table */
        i32 offset = vm_jump_table[opcode];

        /* Calculate absolute handler address: base + offset */
        vm_handler_fn handler = (vm_handler_fn)(VM_DISPATCH_BASE + offset);

        /* Indirect call via function pointer */
        u32 step = handler(vm);

        vm->pc += step;
    }
    return vm->R[0];
}

/* ================================================================
 * Test bytecode: 10 * 3 + 7 = 37
 * ================================================================ */
static u8 test_bytecode[] = {
    /* MOV_IMM32 R0, 10 */
    0x49, 0x00, 0x0A, 0x00, 0x00, 0x00,
    /* MOV_IMM32 R1, 3 */
    0x49, 0x01, 0x03, 0x00, 0x00, 0x00,
    /* MUL R0, R0, R1 */
    0x83, 0x00, 0x00, 0x01,
    /* MOV_IMM32 R1, 7 */
    0x49, 0x01, 0x07, 0x00, 0x00, 0x00,
    /* ADD R0, R0, R1 */
    0x37, 0x00, 0x00, 0x01,
    /* HALT */
    0x00
};

/* ================================================================
 * ARM64 syscall helper
 * ================================================================ */

static void sys_write(const char *buf, u64 len) {
    register long x0 asm("x0") = 1;        /* fd = stdout */
    register long x1 asm("x1") = (long)buf;
    register long x2 asm("x2") = (long)len;
    register long x8 asm("x8") = 64;       /* __NR_write */
    asm volatile("svc #0"
        : : "r"(x0), "r"(x1), "r"(x2), "r"(x8)
        : "memory");
}

static void __attribute__((noreturn)) sys_exit(int code) {
    register long x0 asm("x0") = code;
    register long x8 asm("x8") = 93;       /* __NR_exit */
    asm volatile("svc #0" : : "r"(x0), "r"(x8));
    __builtin_unreachable();
}

/* ================================================================
 * _start entry — bare-metal style, no libc
 * ================================================================ */
void _start(void) {
    /* 1. Initialize jump table */
    init_jump_table();

    /* 2. Initialize VM context */
    vm_ctx_t vm;
    for (int i = 0; i < VM_REG_COUNT; i++)
        vm.R[i] = 0;
    vm.pc     = 0;
    vm.bc     = test_bytecode;
    vm.bc_len = (u32)sizeof(test_bytecode);
    vm.halted = 0;

    /* 3. Execute */
    u64 result = vm_run(&vm);

    /* 4. Output result */
    if (result == 37) {
        sys_write("PASS\n", 5);
    } else {
        sys_write("FAIL\n", 5);
    }

    /* 5. exit(result) */
    sys_exit((int)result);
}
