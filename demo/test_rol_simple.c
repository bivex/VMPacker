#include <stdio.h>

// Minimal function: ROL + extra ops to be big enough for trampoline
__attribute__((noinline))
unsigned int simple_rol(unsigned int val, unsigned int count) {
    unsigned int hash = val ^ 0xDEADBEEF;
    hash += 0x1234;
    hash ^= (hash >> 16);
    count = (count & 31);
    hash = (hash << count) | (hash >> (32 - count));
    hash *= 0x9E3779B9;
    count = (hash & 0xF) + 1;
    hash = (hash << (count & 31)) | (hash >> (32 - (count & 31)));
    return hash;
}

int main() {
    unsigned int r = simple_rol(0xA5A6, 5);
    printf("simple_rol(0xA5A6, 5) = 0x%08X\n", r);
    return 0;
}
