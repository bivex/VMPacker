#include <stdio.h>
#include <stdint.h>

/*
 * demo_insn_strh.c — STRH (16-bit store) test
 *
 * Test STRH storing the lower 16 bits of a register to memory
 */

__attribute__((noinline)) int64_t check_strh(void) {
    uint16_t arr[4] = {0, 0, 0, 0};
    uint64_t v0 = 0x1234, v1 = 0xABCD, v2 = 0xFFFF, v3 = 0x0001;

    __asm__ volatile(
        "strh %w[s0], [%[base], #0]\n"
        "strh %w[s1], [%[base], #2]\n"
        "strh %w[s2], [%[base], #4]\n"
        "strh %w[s3], [%[base], #6]\n"
        :
        : [base] "r"(arr), [s0] "r"(v0), [s1] "r"(v1), [s2] "r"(v2), [s3] "r"(v3)
        : "memory"
    );
    __asm__ volatile("nop; nop; nop; nop; nop; nop; nop; nop;");

    if (arr[0] == 0x1234 && arr[1] == 0xABCD && arr[2] == 0xFFFF && arr[3] == 0x0001) return 1;
    printf("DETAIL: [0]=0x%x [1]=0x%x [2]=0x%x [3]=0x%x\n", arr[0], arr[1], arr[2], arr[3]);
    return 0;
}

int main(void) {
    int64_t v = check_strh();
    if (v == 1) { printf("PASS:STRH\n"); return 0; }
    printf("FAIL:STRH\n");
    return 1;
}
