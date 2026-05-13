#include <stdio.h>
#include <stdint.h>
int main() {
    uint64_t rflags = 0;
    asm volatile(
        "mov $5, %%rax\n"
        "mov $5, %%rbx\n"
        "sub %%rbx, %%rax\n"
        "pushfq\n"
        "pop %0\n"
        : "=r" (rflags)
        :
        : "rax", "rbx"
    );
    printf("RFLAGS: %llx\n", rflags);
    printf("ZF (bit 6): %d\n", !!(rflags & (1ULL<<6)));
    return 0;
}
