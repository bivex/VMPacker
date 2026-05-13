#include <stdio.h>

__attribute__((noinline)) int check1(int a) {
    __asm__ volatile ("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");
    return a + 10;
}

__attribute__((noinline)) int check2(int a) {
    __asm__ volatile ("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");
    return a + 20;
}

__attribute__((noinline)) int check3(int a) {
    __asm__ volatile ("nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop; nop;");
    return a + 30;
}

int main() {
    int v1 = check1(1);
    int v2 = check2(1);
    int v3 = check3(1);
    printf("R1: %d\n", v1);
    printf("R2: %d\n", v2);
    printf("R3: %d\n", v3);
    if (v1 == 11 && v2 == 21 && v3 == 31) {
        printf("Verification: SUCCESS\n");
        return 0;
    }
    printf("Verification: FAILED\n");
    return 1;
}
