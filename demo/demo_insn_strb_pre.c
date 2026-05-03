// demo_insn_strb_pre.c
// T2: STURB (byte store, unscaled offset) 测试程序
// 验证 STURB W0, [Xn, #imm9] 的正确性
//
// 注意: 原始 raw 0x381FF260 实际是 STURB (unscaled offset, bits[11:10]=00)
// 而非 STRB pre-index。此 demo 覆盖 STURB 的各种偏移场景。
#include <stdio.h>
#include <stdint.h>
#include <string.h>

__attribute__((noinline))
int test_strb_pre(void) {
    // 使用栈上缓冲区测试 STURB
    uint8_t buf[32];
    memset(buf, 0, sizeof(buf));

    // 准备测试值
    uint64_t val_a = 0xAB;
    uint64_t val_b = 0xCD;
    uint64_t val_c = 0xEF;
    uint64_t val_d = 0x42;

    // Get pointer to the middle of buf
    uint8_t *ptr = &buf[16];

    // Test 1: STURB with negative offset (-1)
    // STURB W0, [X1, #-1]  → 存储到 ptr[-1] = buf[15]
    __asm__ volatile(
        "mov x1, %[p]\n\t"
        "mov w0, %w[va]\n\t"
        ".inst 0x38000020\n\t"  // STURB W0, [X1, #0] → buf[16]
        :
        : [p] "r"(ptr), [va] "r"(val_a)
        : "x0", "x1", "memory"
    );

    // Test 2: STURB with negative offset
    __asm__ volatile(
        "mov x1, %[p]\n\t"
        "mov w0, %w[vb]\n\t"
        ".inst 0x381FF020\n\t"  // STURB W0, [X1, #-1] → buf[15]
        :
        : [p] "r"(ptr), [vb] "r"(val_b)
        : "x0", "x1", "memory"
    );

    // Test 3: STURB with positive offset
    __asm__ volatile(
        "mov x1, %[p]\n\t"
        "mov w0, %w[vc]\n\t"
        ".inst 0x38001020\n\t"  // STURB W0, [X1, #1] → buf[17]
        :
        : [p] "r"(ptr), [vc] "r"(val_c)
        : "x0", "x1", "memory"
    );

    // Test 4: STURB with offset +2
    __asm__ volatile(
        "mov x1, %[p]\n\t"
        "mov w0, %w[vd]\n\t"
        ".inst 0x38002020\n\t"  // STURB W0, [X1, #2] → buf[18]
        :
        : [p] "r"(ptr), [vd] "r"(val_d)
        : "x0", "x1", "memory"
    );

    // 验证结果
    uint32_t result = 0;
    result |= ((uint32_t)buf[15]) << 24;  // 0xCD
    result |= ((uint32_t)buf[16]) << 16;  // 0xAB
    result |= ((uint32_t)buf[17]) << 8;   // 0xEF
    result |= ((uint32_t)buf[18]);         // 0x42

    // 额外填充指令确保函数体 > 72 字节
    __asm__ volatile(
        "nop\n\t" "nop\n\t" "nop\n\t" "nop\n\t"
        "nop\n\t" "nop\n\t" "nop\n\t" "nop\n\t"
        "nop\n\t" "nop\n\t" "nop\n\t" "nop\n\t"
        "nop\n\t" "nop\n\t" "nop\n\t" "nop\n\t"
        ::: "memory"
    );

    return (int)result;
}

int main(void) {
    int result = test_strb_pre();
    int expected = 0xCDABEF42;

    if (result == expected) {
        printf("STRB_PRE PASS (result=0x%08X)\n", result);
        return 0;
    } else {
        printf("STRB_PRE FAIL (result=0x%08X, expected=0x%08X)\n", result, expected);
        return 1;
    }
}
