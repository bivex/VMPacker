#include <stdio.h>
#include <stdint.h>

/*
 * demo_insn_ldrh.c — LDRH (16-bit unsigned load) test
 *
 * Test LDRH loading 16-bit unsigned value from memory and zero-extending it to 64-bit
 */

__attribute__((noinline)) int64_t check_ldrh(void) {
    uint16_t arr[4] = {0x1234, 0xABCD, 0xFFFF, 0x0001};
    uint64_t r0 = 0, r1 = 0, r2 = 0, r3 = 0;

    /* LDRH various offsets */
    __asm__ volatile(
        "ldrh %w[o0], [%[base], #0]\n"
        "ldrh %w[o1], [%[base], #2]\n"
        "ldrh %w[o2], [%[base], #4]\n"
        "ldrh %w[o3], [%[base], #6]\n"
        : [o0] "=r"(r0), [o1] "=r"(r1), [o2] "=r"(r2), [o3] "=r"(r3)
        : [base] "r"(arr)
        : "memory"
    );
    __asm__ volatile("nop; nop; nop; nop; nop; nop; nop; nop;");

    if (r0 == 0x1234 && r1 == 0xABCD && r2 == 0xFFFF && r3 == 0x0001) return 1;
    printf("DETAIL: r0=0x%lx r1=0x%lx r2=0x%lx r3=0x%lx\n", r0, r1, r2, r3);
    return 0;
}

int main(void) {
    int64_t v = check_ldrh();
    if (v == 1) { printf("PASS:LDRH\n"); return 0; }
    printf("FAIL:LDRH\n");
    return 1;
}
