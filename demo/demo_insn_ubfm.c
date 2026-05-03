#include <stdio.h>
#include <stdint.h>

/*
 * demo_insn_ubfm.c — UBFM width>=32 test
 *
 * Test cases where UBFX extracts bitfield width >= 32
 * UBFX Xd, Xn, #lsb, #width  =>  UBFM Xd, Xn, #lsb, #(lsb+width-1)
 *
 * Test 1: UBFX X0, X0, #0, #33  (extract lower 33 bits)
 *   UBFM X0, X0, #0, #32  (immr=0, imms=32)
 *   Input: 0x3FFFFFFFF (34 bits) → Output: 0x1FFFFFFFF (lower 33 bits)
 *
 * Test 2: UBFX X0, X0, #16, #33 (extract 33 bits starting from bit 16)
 *   UBFM X0, X0, #16, #48  (immr=16, imms=48)
 *   Input: 0x1FFFFFFFFFFFF (49 bits) → Output: 0x1FFFFFFFF (33 bits)
 */

__attribute__((noinline)) int64_t check_ubfm(void) {
    uint64_t v1 = 0x3FFFFFFFFULL;  /* Lower 34 bits all 1 */
    uint64_t r1 = 0;

    /* UBFX x0, x0, #0, #33 → extract lower 33 bits → 0x1FFFFFFFF */
    __asm__ volatile(
        "ubfx %[out], %[in], #0, #33\n"
        : [out] "=r"(r1)
        : [in] "r"(v1)
    );
    __asm__ volatile("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");

    uint64_t v2 = 0x1FFFFFFFFFFFFULL; /* Lower 49 bits all 1 */
    uint64_t r2 = 0;

    /* UBFX x0, x0, #16, #33 → extract 33 bits from bit 16 → 0x1FFFFFFFF */
    __asm__ volatile(
        "ubfx %[out], %[in], #16, #33\n"
        : [out] "=r"(r2)
        : [in] "r"(v2)
    );
    __asm__ volatile("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");

    if (r1 == 0x1FFFFFFFFULL && r2 == 0x1FFFFFFFFULL) return 1;
    printf("r1=%llx, r2=%llx\n", r1, r2);
    return 0;
}

__attribute__((noinline)) int64_t check_ubfm2(void) {
    uint64_t v1 = 0x3FFFFFFFFFFFFULL; /* Lower 50 bits all 1, but bit 33 is 0? */
    uint64_t r1 = 0;

    /* UBFX x0, x0, #0, #33 → extract lower 33 bits → 0x1FFFFFFFF */
    __asm__ volatile(
        "ubfx %[out], %[in], #16, #33\n"
        : [out] "=r"(r1)
        : [in] "r"(v1)
    );
    __asm__ volatile("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");


    if (r1 == 0x1FFFFFFFFULL) return 1;
    printf("r1=%llx\n", r1);
    return 0;
}


__attribute__((noinline)) int64_t check_ubfm3(void) {
    // Set a test value to make low and high bits easy to distinguish
    // Use 0xFEDCBA9876543210 so each byte is different
    uint64_t v1 = 0xFEDCBA9876543210ULL;
    uint64_t r1 = 0;

    /* UBFIZ x0, x1, #16, #8
     * Extract the lower 8 bits of x1 (0x10), shift left by 16 bits
     * Result should be 0x0000000000100000
     */
    __asm__ volatile(
        "ubfiz %[out], %[in], #16, #8\n"
        : [out] "=r"(r1)
        : [in] "r"(v1)
    );
    __asm__ volatile("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");

    // Expected result: lower 8 bits are 0x10, becomes 0x100000 after 16-bit left shift
    if (r1 == 0x100000ULL) return 1;
    printf("r1=%llx\n", r1);
    return 0;
}


int main(void) {
    int64_t v = check_ubfm();
    if (v == 1) { 
        printf("PASS:UBFM\n");
    } else {
        printf("FAIL:UBFM r=%ld\n", v);
    }
    v = check_ubfm2();
    if (v == 1) { 
        printf("PASS:UBFM2\n");
    } else {
        printf("FAIL:UBFM2 r=%ld\n", v);
    }
    v = check_ubfm3();
    if (v == 1) { 
        printf("PASS:UBFM3\n");
    } else {
        printf("FAIL:UBFM3 r=%ld\n", v);
    }
    return 1;
}