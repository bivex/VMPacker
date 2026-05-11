#include <stdio.h>

int check_logic(int a, int b) {
    int res = 0;
    if (a > b) {
        res = a * b + 10;
    } else {
        res = (a + b) ^ 0x55;
    }
    
    for (int i = 0; i < 5; i++) {
        res += i;
    }
    
    return res;
}

int main() {
    int x = 20;
    int y = 10;
    int result = check_logic(x, y);
    
    printf("Input: %d, %d\n", x, y);
    printf("Result: %d\n", result);
    
    if (result == 210+10) { // 20*10 + 10 + (0+1+2+3+4) = 210 + 10 = 220? wait.
        // 20 > 10: 20*10+10 = 210.
        // Loop: +0+1+2+3+4 = +10.
        // Total: 220.
        printf("Verification: SUCCESS\n");
        return 0;
    } else {
        printf("Verification: FAILED (expected 220, got %d)\n", result);
        return 1;
    }
}
