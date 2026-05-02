/*
 * demo_complex.c — VMP Complex Patterns Test
 *
 * This demo exercises features critical for Clang/Android compatibility:
 * 1. Floating Point (FPU) math.
 * 2. SIMD memory operations (LDP/STP Q registers).
 * 3. Jump Tables (BR Xn for switch statements).
 * 4. System registers (MRS).
 */

#include <stdint.h>

/* Target function for virtualization */
__attribute__((noinline))
int64_t check_complex(int32_t selector, double factor) {
    double result = 0.0;
    
    /* 1. Exercise Jump Table (Clang -O1/-O2 often uses BR Xn for this) */
    switch (selector) {
        case 10: result = 1.5; break;
        case 20: result = 2.5; break;
        case 30: result = 3.5; break;
        case 42: result = 42.0; break;
        case 100: result = 100.1; break;
        default: result = 0.1; break;
    }

    /* 2. Floating Point Arithmetic (FADD, FMUL) */
    result = (result * factor) + 7.0;

    /* 3. SIMD-style memory movement (LDP/STP)
     * Using a local array of 64 bytes to encourage Clang to use Q registers. */
    uint64_t data[8];
    for (int i = 0; i < 8; i++) data[i] = (uint64_t)result + i;
    
    /* Sum it up to ensure side effects */
    uint64_t sum = 0;
    for (int i = 0; i < 8; i++) sum += data[i];

    /* 4. System instruction (MRS cntvct_el0) */
    uint64_t ticks;
    __asm__ volatile("mrs %0, cntvct_el0" : "=r"(ticks));
    
    return (int64_t)result + (sum % 10) + (ticks & 0);
}

/* Static buffer for output */
char out_buf[16];

void _start(void) {
    /* Test case: selector=42, factor=2.0
     * result = (42.0 * 2.0) + 7.0 = 91.0
     * data[0..7] = 91..98
     * sum = 91+92+93+94+95+96+97+98 = 756
     * sum % 10 = 6
     * Final: 91 + 6 = 97
     */
    int64_t val = check_complex(42, 2.0);

    /* Prepare output string: "Result: XX, Hex: XXXX\n" */
    for(int i=0; i<16; i++) out_buf[i] = ' ';
    out_buf[0] = 'R'; out_buf[1] = 'e'; out_buf[2] = 's'; out_buf[3] = ':';
    out_buf[5] = '0' + (val / 100 % 10);
    out_buf[6] = '0' + (val / 10 % 10);
    out_buf[7] = '0' + (val % 10);
    out_buf[8] = '\n';

    /* syscall: write(1, out_buf, 9) */
    register long x0 __asm__("x0") = 1;
    register long x1 __asm__("x1") = (long)out_buf;
    register long x2 __asm__("x2") = 9;
    register long x8 __asm__("x8") = 64;
    __asm__ volatile("svc #0" : : "r"(x0), "r"(x1), "r"(x2), "r"(x8) : "memory");

    /* syscall: exit(val) */
    x0 = val;
    x8 = 93;
    __asm__ volatile("svc #0" : : "r"(x0), "r"(x8));
}
