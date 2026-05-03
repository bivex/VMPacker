#include <stdio.h>
#include <stdint.h>

/*
 * demo_insn_ldrsb.c — LDRSB instruction test
 *
 * LDRSB Xd, [Xn, #imm] : Load an 8-bit signed value from memory, sign-extended to 64-bit
 * Testing both positive and negative cases
 */

__attribute__((noinline)) int64_t check_ldrsb(void) {
    /* Positive: 0x7F = 127 → Remains 0x000000000000007F after sign-extension */
    /* Negative: 0x80 = -128 (i8) → 0xFFFFFFFFFFFFFF80 after sign-extension */
    int8_t buf[2] = { -128, 127 };
    int64_t r1 = 0, r2 = 0;

    __asm__ volatile(
        "ldrsb %[o1], [%[p]]\n"       /* buf[0] = -128 → sign-extend to i64 */
        "ldrsb %[o2], [%[p], #1]\n"   /* buf[1] = 127 → sign-extend */
        : [o1] "=r"(r1), [o2] "=r"(r2)
        : [p] "r"(buf)
        : "memory"
    );
    __asm__ volatile("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");

    /* r1 should be -128 (0xFFFFFFFFFFFFFF80) */
    /* r2 should be 127 (0x000000000000007F) */
    if (r1 == -128 && r2 == 127) {
        return 1; /* PASS */
    }
    return 0; /* FAIL */
}

int main(void) {
    int64_t v = check_ldrsb();
    if (v == 1) {
        printf("PASS:LDRSB\n");
        return 0;
    }
    printf("FAIL:LDRSB r=%ld\n", v);
    return 1;
}
