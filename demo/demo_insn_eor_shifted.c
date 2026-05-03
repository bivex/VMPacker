// demo/demo_insn_eor_shifted.c
// Test all EOR instruction shift variants: LSL, LSR, ASR, ROR
#include <stdio.h>
#include <stdint.h>

// Test EOR with ROR shift (common in CRC32)
// Function needs to be large enough (>72B) to support standard mode trampoline
int __attribute__((noinline)) test_eor_shifted(uint32_t a, uint32_t b) {
    uint32_t r1, r2, r3, r4;
    uint32_t t1, t2, t3, t4;
    uint32_t u1, u2;

    // EOR with ROR #25
    asm volatile("eor %w0, %w1, %w2, ror #25" : "=r"(r1) : "r"(a), "r"(b));
    // EOR with LSR #8
    asm volatile("eor %w0, %w1, %w2, lsr #8" : "=r"(r2) : "r"(a), "r"(b));
    // EOR with ASR #16
    asm volatile("eor %w0, %w1, %w2, asr #16" : "=r"(r3) : "r"(a), "r"(b));
    // EOR with LSL #4 (already supported, as control)
    asm volatile("eor %w0, %w1, %w2, lsl #4" : "=r"(r4) : "r"(a), "r"(b));

    // AND with ROR #13
    asm volatile("and %w0, %w1, %w2, ror #13" : "=r"(t1) : "r"(a), "r"(b));
    // ORR with LSR #7
    asm volatile("orr %w0, %w1, %w2, lsr #7" : "=r"(t2) : "r"(a), "r"(b));
    // AND with ASR #5
    asm volatile("and %w0, %w1, %w2, asr #5" : "=r"(t3) : "r"(a), "r"(b));
    // ORR with ROR #19
    asm volatile("orr %w0, %w1, %w2, ror #19" : "=r"(t4) : "r"(a), "r"(b));

    // Extra instructions to ensure function body > 72 bytes
    // EOR with LSR #3
    asm volatile("eor %w0, %w1, %w2, lsr #3" : "=r"(u1) : "r"(r1), "r"(r2));
    // EOR with ASR #11
    asm volatile("eor %w0, %w1, %w2, asr #11" : "=r"(u2) : "r"(r3), "r"(r4));

    // Combine all results to ensure each variant is used
    uint32_t eor_result = r1 ^ r2 ^ r3 ^ r4;
    uint32_t logic_result = t1 ^ t2 ^ t3 ^ t4;
    uint32_t extra_result = u1 ^ u2;
    return (int)(eor_result ^ logic_result ^ extra_result);
}

int main() {
    uint32_t a = 0xDEADBEEF;
    uint32_t b = 0x12345678;

    int got = test_eor_shifted(a, b);

    // Native run verification: as long as it doesn't crash and returns non-zero
    // Compare with native results during VMP testing
    printf("EOR_SHIFTED result=0x%08X\n", (uint32_t)got);

    // Use adb to get baseline values from native run, then compare with VMP run
    // Simple sanity check: result should not be 0
    if (got != 0) {
        printf("EOR_SHIFTED PASS\n");
        return 0;
    }
    printf("EOR_SHIFTED FAIL: got 0\n");
    return 1;
}
