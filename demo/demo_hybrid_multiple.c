#include <stdio.h>

__attribute__((noinline)) int vmp_test_add(int a, int b) {
    __asm__ volatile ("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");
    return a + b + 5;
}

__attribute__((noinline)) int vmp_test_xor(int x) {
    __asm__ volatile ("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");
    return x ^ 0xAAAA;
}

__attribute__((noinline)) int vmp_test_max(int a, int b) {
    __asm__ volatile ("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");
    if (a > b) return a;
    return b;
}

int main() {
    printf("Add: %d\n", vmp_test_add(10, 20));
    printf("Xor: 0x%X\n", vmp_test_xor(0x1234));
    printf("Max: %d\n", vmp_test_max(50, 40));
    return 0;
}
