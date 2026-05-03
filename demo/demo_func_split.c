/*
 * demo_func_split.c — Multi-Function Splitting Demo
 *
 * Verify that __attribute__((section(...))) disperses functions into different ELF sections
 * and they can still execute correctly through indirect function pointer calls.
 *
 * Compilation: aarch64-linux-gnu-gcc -static -O1 -nostdlib -march=armv8-a \
 *       -o demo_func_split demo/demo_func_split.c
 *
 * Expected output: "AMBS" + newline (functions in all 4 sections execute correctly)
 *   A = ALU, M = MEM, B = BRANCH, S = SYSTEM
 */

typedef unsigned long u64;
typedef unsigned int  u32;
typedef unsigned char u8;

/* ---- Simulated handler function signature ---- */
typedef u32 (*handler_fn)(u32 a, u32 b);

/* ---- Section Splitting: 4 functions placed into different sections ---- */

__attribute__((section(".text.vm_alu"), noinline))
u32 h_alu_add(u32 a, u32 b) {
    return a + b;  /* ALU: Addition, 10+3=13 */
}

__attribute__((section(".text.vm_mem"), noinline))
u32 h_mem_load(u32 a, u32 b) {
    return a * b;  /* MEM: Multiplication, 10*3=30 */
}

__attribute__((section(".text.vm_branch"), noinline))
u32 h_branch_cmp(u32 a, u32 b) {
    return (a > b) ? a - b : b - a;  /* BRANCH: Difference, |10-3|=7 */
}

__attribute__((section(".text.vm_system"), noinline))
u32 h_system_xor(u32 a, u32 b) {
    return a ^ b;  /* SYSTEM: XOR, 10^3=9 */
}

/* ---- _start ---- */
void _start(void) {
    /* Build function pointer table (volatile prevents compiler optimization to direct calls) */
    volatile handler_fn table[4];
    table[0] = h_alu_add;
    table[1] = h_mem_load;
    table[2] = h_branch_cmp;
    table[3] = h_system_xor;

    u32 a = 10, b = 3;
    char buf[6];
    int idx = 0;

    /* Indirect call via function pointers */
    buf[idx++] = (table[0](a, b) == 13) ? 'A' : 'x';
    buf[idx++] = (table[1](a, b) == 30) ? 'M' : 'x';
    buf[idx++] = (table[2](a, b) ==  7) ? 'B' : 'x';
    buf[idx++] = (table[3](a, b) ==  9) ? 'S' : 'x';
    buf[idx++] = '\n';

    /* Single write syscall output all results */
    register long x0 __asm__("x0") = 1;
    register long x1 __asm__("x1") = (long)buf;
    register long x2 __asm__("x2") = (long)idx;
    register long x8 __asm__("x8") = 64;
    __asm__ volatile("svc #0"
        : "+r"(x0)
        : "r"(x1), "r"(x2), "r"(x8)
        : "memory");

    /* exit(0) */
    x0 = 0;
    x8 = 93;
    __asm__ volatile("svc #0" : : "r"(x0), "r"(x8));
}
