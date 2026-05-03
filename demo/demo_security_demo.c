/*
 * demo_security_demo.c — Security Protection Demo & Attack Simulation
 */
#include <stdint.h>

// A loop to ensure we cross the VM_CHECK_INTERVAL (1024)
__attribute__((noinline))
int security_test_loop(int iterations) {
    int sum = 0;
    for (int i = 0; i < iterations; i++) {
        sum += (i % 3);
    }
    return sum;
}

char out_buf[32];
volatile int iters = 2000;

void _start(void) {
    // Run the loop with enough iterations to trigger periodic checks
    int val = security_test_loop(iters);

    // Preparation for output
    for(int i=0; i<32; i++) out_buf[i] = ' ';
    out_buf[0] = 'D'; out_buf[1] = 'o'; out_buf[2] = 'n'; out_buf[3] = 'e';
    out_buf[4] = '\n';

    // write(1, out_buf, 5)
    register long x0 __asm__("x0") = 1;
    register long x1 __asm__("x1") = (long)out_buf;
    register long x2 __asm__("x2") = 5;
    register long x8 __asm__("x8") = 64;
    __asm__ volatile("svc #0" : : "r"(x0), "r"(x1), "r"(x2), "r"(x8) : "memory");

    // exit(val)
    x0 = (long)val;
    x8 = 93;
    __asm__ volatile("svc #0" : : "r"(x0), "r"(x8));
}
