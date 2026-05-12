/*
 * hybrid_test.c — End-to-end test for Hybrid Mode (Native Execution)
 *
 * Tests that OpSNativeExec correctly executes native x86_64 code embedded
 * in bytecode and that condition flags are captured and used by VM branches.
 *
 * Expected behavior:
 *   - Initial: RAX=5, RBX=5
 *   - Native block: sub rax, rbx   => result 0, sets ZF=1
 *   - VM JE  jumps to success (because ZF=1)
 *   - Success: MOV R0, 42
 *   - Return 42 → test PASS
 *
 * If flags are NOT captured, JE falls through and returns 0 (FAIL).
 *
 * Build (from repo root):
 *   gcc -static -O1 -fno-stack-protector -fno-builtin -nostdlib \
 *       -march=x86-64 -I../../stub/linux/x86_64 \
 *       -DVM_INDIRECT_DISPATCH hybrid_test.c \
 *       ../../stub/linux/x86_64/vm_interp.c \
 *       -o hybrid_test_x86_64
 *
 * Run: ./hybrid_test_x86_64
 */

#include <stdint.h>

/* VM entry point declaration (defined in vm_interp.c) */
extern uint64_t vm_entry(uint64_t *args, uint8_t *enc_bc, uint32_t bc_len,
                         uint8_t xor_key, uint64_t slide,
                         void *rtlr_ptr, uint32_t func_id);

/* Bytecode (identity mapping, small, no trailing map) */
static uint8_t bytecode[] = {
    /* OpNativeExec: [0x8D][len=4][sub rax,rbx][RET] */
    0x8D,                   // OP_S_NATIVEEXEC (141)
    0x04, 0x00,            // length = 4 (little-endian; includes sub + RET)
    0x48, 0x29, 0xD8,      // sub rax, rbx (3 bytes)
    0xC3,                  // RET

    /* JE success (ZF=1 -> jump) — uses rd32() for target */
    0x24,                   // OP_JE (36)
    0x17, 0x00, 0x00, 0x00, // target = 23 (little-endian 32-bit)

    /* MOV_IMM32 R0, 0  (fail path) */
    0x02,                  // OP_MOV_IMM32 (2)
    0x00,                  // reg = 0 (RAX)
    0x00, 0x00, 0x00, 0x00, // imm32 = 0

    /* JMP end — uses rd32() for target */
    0x23,                  // OP_JMP (35)
    0x1D, 0x00, 0x00, 0x00, // target = 29 (little-endian)

    /* success: MOV_IMM32 R0, 42 — uses rd32() for imm */
    0x02,                  // OP_MOV_IMM32
    0x00,                  // reg = 0
    0x2A, 0x00, 0x00, 0x00, // imm32 = 42 (little-endian)

    /* HALT */
    0x36                   // OP_HALT (54)
};

/* Syscall helpers (x86_64 Linux) */
static void sys_write(const char *buf, uint64_t len) {
    register long rax __asm__("rax") = 1;          /* SYS_write */
    register long rdi __asm__("rdi") = 1;          /* fd = stdout */
    register long rsi __asm__("rsi") = (long)buf;
    register long rdx __asm__("rdx") = (long)len;
    __asm__ volatile("syscall"
                     : : "a"(rax), "D"(rdi), "S"(rsi), "d"(rdx)
                     : "rcx", "r11", "memory");
}

static void sys_exit(long code) {
    register long rax __asm__("rax") = 60;         /* SYS_exit */
    register long rdi __asm__("rdi") = code;
    __asm__ volatile("syscall" : : "a"(rax), "D"(rdi) : "rcx", "r11", "memory");
}

/* Entry point (no libc) */
void _start(void) {
    /* Initialize all 14 argument registers to zero */
    uint64_t args[14] = {0};

    /* Set initial register values: RAX=5, RBX=5 (identity reg_map) */
    args[6] = 5;  /* RAX is always supplied by args[6] in vm_ctx_init */
    args[7] = 5;  /* RBX is always supplied by args[7] */

    /* Execute bytecode */
    uint64_t ret = vm_entry(args, bytecode, sizeof(bytecode),
                            0,   /* xor_key */
                            0,   /* slide */
                            0,   /* rtlr_ptr */
                            0);  /* func_id */

    /* Check result */
    if (ret == 42) {
        sys_write("PASS\n", 5);
        sys_exit(0);
    } else {
        sys_write("FAIL\n", 5);
        sys_exit(1);
    }
}
