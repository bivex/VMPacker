#include <stdio.h>

__attribute__((noinline))
int just_return(int x) {
    int r = x ^ 0x12345678;
    r += 0xABCDEF;
    r ^= (r >> 8);
    r *= 0x9E3779B9;
    return r;
}

int main() {
    printf("just_return(41) = 0x%08X\n", just_return(41));
    return 0;
}
