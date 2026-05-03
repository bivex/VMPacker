/*
 * demo_insn_umulh.c — UMULH (unsigned multiply high 64-bit) 测试
 *
 * UMULH Xd, Xn, Xm: Xd = (Xn * Xm)[127:64]
 *
 * All UMULH instructions are inlined in test_umulh, without subroutine calls.
 * 使用 X9-X14 作为安全临时寄存器，避免与 ABI 参数冲突。
 *
 * .inst 编码:
 *   0x9BCA7D29 = UMULH X9, X9, X10
 *     1_00_11011_110_01010_0_11111_01001_01001
 *   0x9BCC7D69 = UMULH X9, X11, X12
 *     1_00_11011_110_01100_0_11111_01011_01001
 *
 * 交叉编译:
 *   aarch64-linux-gnu-gcc -O1 -static -o build/demo_insn_umulh demo/demo_insn_umulh.c
 */
#include <stdio.h>
#include <stdint.h>

__attribute__((noinline))
uint64_t test_umulh(uint64_t a, uint64_t b,
                    uint64_t c, uint64_t d,
                    uint64_t e) {
    /*
     * 计算: (UMULH(a, b) ^ UMULH(c, d)) + e
     *
     * ABI: a=X0, b=X1, c=X2, d=X3, e=X4
     * 先把所有参数保存到 X9-X13，再用 .inst
     */
    uint64_t result;
    __asm__ volatile(
        /* 保存所有参数到安全寄存器 */
        "mov x9,  %[a]\n"
        "mov x10, %[b]\n"
        "mov x11, %[c]\n"
        "mov x12, %[d]\n"
        "mov x13, %[e]\n"

        /* UMULH X9, X9, X10 → X9 = hi(a*b) */
        ".inst 0x9BCA7D29\n"

        /* 保存 hi1 到 X14 */
        "mov x14, x9\n"

        /* 重新加载 X11, X12 (可能被编译器优化掉，但 .inst 不会碰它们) */
        /* UMULH X9, X11, X12 → X9 = hi(c*d) */
        ".inst 0x9BCC7D69\n"

        /* X9 = hi1 ^ hi2 */
        "eor x9, x14, x9\n"

        /* result = (hi1 ^ hi2) + e */
        "add %[out], x9, x13\n"

        /* padding NOPs to ensure function > 72 bytes */
        "nop\n" "nop\n" "nop\n" "nop\n"
        "nop\n" "nop\n" "nop\n" "nop\n"
        "nop\n" "nop\n" "nop\n" "nop\n"
        "nop\n" "nop\n" "nop\n" "nop\n"

        : [out] "=r" (result)
        : [a] "r" (a), [b] "r" (b), [c] "r" (c), [d] "r" (d), [e] "r" (e)
        : "x9", "x10", "x11", "x12", "x13", "x14", "memory"
    );
    return result;
}

int main(void) {
    /*
     * a = 0xDEADBEEFCAFEBABE
     * b = 0x1234567890ABCDEF
     * c = 0xFEDCBA9876543210
     * d = 0x0123456789ABCDEF
     * e = 0x1111111111111111
     *
     * UMULH(a, b) = hi64(0xDEADBEEFCAFEBABE * 0x1234567890ABCDEF)
     *            = 0x1010E8F9C66C6412
     * UMULH(c, d) = hi64(0xFEDCBA9876543210 * 0x0123456789ABCDEF)
     *            = 0x0123456789ABCDE0
     * hi1 ^ hi2  = 0x1133ADB04FC7A8D2
     * result     = 0x1133ADB04FC7A8D2 + 0x1111111111111111
     *            = 0x2244BEC160D8B9E3
     *
     * 但实际值需要在设备上验证，这里用运行时比较。
     */
    uint64_t a = 0xDEADBEEFCAFEBABEULL;
    uint64_t b = 0x1234567890ABCDEFULL;
    uint64_t c = 0xFEDCBA9876543210ULL;
    uint64_t d = 0x0123456789ABCDEFULL;
    uint64_t e = 0x1111111111111111ULL;

    uint64_t got = test_umulh(a, b, c, d, e);

    /* 用 __uint128_t 计算期望值 */
    __uint128_t prod1 = (__uint128_t)a * b;
    __uint128_t prod2 = (__uint128_t)c * d;
    uint64_t hi1 = (uint64_t)(prod1 >> 64);
    uint64_t hi2 = (uint64_t)(prod2 >> 64);
    uint64_t expected = (hi1 ^ hi2) + e;

    printf("UMULH test: a=0x%lX b=0x%lX c=0x%lX d=0x%lX e=0x%lX\n", a, b, c, d, e);
    printf("  hi1=0x%lX hi2=0x%lX\n", hi1, hi2);
    printf("  expected=0x%lX got=0x%lX\n", expected, got);

    if (got == expected) {
        printf("UMULH PASS (result=0x%lX)\n", got);
        return 0;
    } else {
        printf("UMULH FAIL (expected=0x%lX got=0x%lX)\n", expected, got);
        return 1;
    }
}
