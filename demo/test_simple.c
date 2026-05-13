#include <stdio.h>

__attribute__((noinline))
unsigned int simple_add(unsigned int a, unsigned int b) {
    unsigned int r = a ^ 0xDEADBEEF;
    r += b;
    r ^= (r >> 16);
    r *= 0x12345678;
    r += 0xCAFEBABE;
    return r;
}

int main() {
    printf("simple_add(42, 100) = 0x%08X\n", simple_add(42, 100));
    return 0;
}
