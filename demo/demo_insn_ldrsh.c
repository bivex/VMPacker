#include <stdio.h>
#include <stdint.h>

/*
 * demo_insn_ldrsh.c — LDRSH (16-bit signed load) test
 *
 * Testing LDRSH to load 16-bit signed values from memory and sign-extend to 64-bit
 */

__attribute__((noinline)) int64_t check_ldrsh(void) {
    int16_t arr[4] = {0x1234, -1, -0x1234, 0x7FFF};
    int64_t r0 = 0, r1 = 0, r2 = 0, r3 = 0;

    __asm__ volatile(
        "ldrsh %[o0], [%[base], #0]\n"
        "ldrsh %[o1], [%[base], #2]\n"
        "ldrsh %[o2], [%[base], #4]\n"
        "ldrsh %[o3], [%[base], #6]\n"
        : [o0] "=r"(r0), [o1] "=r"(r1), [o2] "=r"(r2), [o3] "=r"(r3)
        : [base] "r"(arr)
        : "memory"
    );
    __asm__ volatile("nop; nop; nop; nop; nop; nop; nop; nop;");

    /* r0 = 0x1234 (positive, remains same after sign-extension) */
    /* r1 = -1 = 0xFFFFFFFFFFFFFFFF */
    /* r2 = -0x1234 = 0xFFFFFFFFFFFFEDCC */
    /* r3 = 0x7FFF (maximum positive value) */
    if (r0 == 0x1234 && r1 == -1 && r2 == -0x1234 && r3 == 0x7FFF) return 1;
    printf("DETAIL: r0=0x%lx r1=0x%lx r2=0x%lx r3=0x%lx\n", r0, r1, r2, r3);
    return 0;
}

int main(void) {
    int64_t v = check_ldrsh();
    if (v == 1) { printf("PASS:LDRSH\n"); return 0; }
    printf("FAIL:LDRSH\n");
    return 1;
}
