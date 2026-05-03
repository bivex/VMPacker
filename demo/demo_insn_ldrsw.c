#include <stdio.h>
#include <stdint.h>

/*
 * demo_insn_ldrsw.c — LDRSW instruction test
 *
 * LDRSW Xd, [Xn, #imm] : Load a 32-bit signed value from memory and sign-extend it to 64 bits
 * Test both positive and negative cases
 */

__attribute__((noinline)) int64_t check_ldrsw(void) {
    /* Positive: 0x7FFFFFFF = 2147483647 → still 0x000000007FFFFFFF after sign extension */
    /* Negative: 0xFFFFFF00 = -256 (i32) → 0xFFFFFFFFFFFFFF00 after sign extension */
    int32_t buf[2] = { -256, 2147483647 };
    int64_t r1 = 0, r2 = 0;

    __asm__ volatile(
        "ldrsw %[o1], [%[p]]\n"       /* buf[0] = -256 → sign-extend to i64 */
        "ldrsw %[o2], [%[p], #4]\n"   /* buf[1] = 0x7FFFFFFF → sign-extend */
        : [o1] "=r"(r1), [o2] "=r"(r2)
        : [p] "r"(buf)
        : "memory"
    );
    __asm__ volatile("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");

    /* r1 should be -256 (0xFFFFFFFFFFFFFF00) */
    /* r2 should be 2147483647 (0x000000007FFFFFFF) */
    if (r1 == -256 && r2 == 2147483647) {
        return 1; /* PASS */
    }
    return 0; /* FAIL */
}

int main(void) {
    int64_t v = check_ldrsw();
    if (v == 1) {
        printf("PASS:LDRSW\n");
        return 0;
    }
    printf("FAIL:LDRSW r=%ld\n", v);
    return 1;
}
