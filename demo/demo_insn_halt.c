#include <stdio.h>

/*
 * HALT instruction verification description:
 * The vmp translator will automatically append OP_HALT at the end of each function's bytecode.
 * This demo keeps the function logic minimal to verify that the protected function can halt normally and return after execution.
 */

__attribute__((noinline)) int check_halt(int x) {
    int y = x + 1;
    __asm__ volatile(
        "nop; nop; nop; nop; nop; nop; nop; nop; "
        "nop; nop; nop; nop; nop; nop; nop; nop; "
        "nop; nop; nop; nop; nop; nop; nop; nop;");
    return y;
}

int main(void) {
    int v = check_halt(41);
    if (v == 42) {
        printf("PASS:HALT:%d\n", v);
        return 0;
    }
    printf("FAIL:HALT:%d\n", v);
    return 1;
}

