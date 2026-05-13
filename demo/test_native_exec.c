#include <stdio.h>
#include <string.h>
#include <stdint.h>

// Standalone test of the native exec mechanism
// Simulates exactly what h_native_exec does, without the VM
int main() {
    // Simulate: RAX=0xA5A6, RCX=5, execute ROL EAX,CL → expect EAX=0x14B4C0
    uint64_t buf[10];
    buf[0] = 0xA5A6;    // RAX
    buf[1] = 0;          // RBX
    buf[2] = 5;          // RCX (shift amount)
    buf[3] = 0;          // RDX
    buf[4] = 0;          // RSI
    buf[5] = 0;          // RDI
    buf[6] = 0;          // R8
    buf[7] = 0;          // R9
    buf[8] = 0;          // RFLAGS

    // Native code: ROL EAX, CL; RET
    uint8_t native_code[] = { 0xD3, 0xC0, 0xC3 };

    // Allocate executable memory
    #include <sys/mman.h>
    void *exec_mem = mmap(NULL, 4096, PROT_READ|PROT_WRITE|PROT_EXEC,
                          MAP_PRIVATE|MAP_ANONYMOUS, -1, 0);
    memcpy(exec_mem, native_code, sizeof(native_code));
    const uint8_t *code_ptr = (const uint8_t *)exec_mem;

    printf("Before: RAX=0x%lX RCX=0x%lX\n", buf[0], buf[2]);

    __asm__ volatile(
        "push %[b]\n\t"
        "push %%rax\n\t"
        "mov 0(%[b]),  %%rax\n\t"
        "mov 8(%[b]),  %%rbx\n\t"
        "mov 16(%[b]), %%rcx\n\t"
        "mov 24(%[b]), %%rdx\n\t"
        "mov 32(%[b]), %%rsi\n\t"
        "mov 40(%[b]), %%rdi\n\t"
        "mov 48(%[b]), %%r8\n\t"
        "mov 56(%[b]), %%r9\n\t"
        "call *%[c]\n\t"
        "pop %%r11\n\t"
        "pop %%r11\n\t"
        "mov %%rax, 0(%%r11)\n\t"
        "mov %%rbx, 8(%%r11)\n\t"
        "mov %%rcx, 16(%%r11)\n\t"
        "mov %%rdx, 24(%%r11)\n\t"
        "mov %%rsi, 32(%%r11)\n\t"
        "mov %%rdi, 40(%%r11)\n\t"
        "mov %%r8,  48(%%r11)\n\t"
        "mov %%r9,  56(%%r11)\n\t"
        "pushfq\n\t"
        "pop %%rax\n\t"
        "mov %%rax, 64(%%r11)\n\t"
        :
        : [b] "r" (buf), [c] "r" (code_ptr)
        : "memory", "cc", "rax", "rbx", "rcx", "rdx", "rsi", "rdi",
          "r8", "r9", "r10", "r11"
    );

    printf("After:  RAX=0x%lX RCX=0x%lX RFLAGS=0x%lX\n", buf[0], buf[2], buf[8]);

    // Expected: ROL(0xA5A6, 5)
    uint64_t expected = ((0xA5A6ULL << 5) | (0xA5A6ULL >> (32-5))) & 0xFFFFFFFF;
    printf("Expected RAX: 0x%lX\n", expected);
    printf("Match: %s\n", buf[0] == expected ? "YES" : "NO");

    return 0;
}
