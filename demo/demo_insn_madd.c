// demo/demo_insn_madd.c
// Test all variants of MADD/MSUB instructions
// MADD: Rd = Ra + Rn * Rm
// MSUB: Rd = Ra - Rn * Rm
// MUL:  Rd = Rn * Rm (MADD with Ra=XZR)
// MNEG: Rd = -(Rn * Rm) (MSUB with Ra=XZR)
#include <stdio.h>

// 64-bit MADD: X = Ra + Rn * Rm
int __attribute__((noinline)) test_madd64(long a, long b, long c) {
  long result;
  // MADD X0, X1, X2, X0  => result = a + b * c
  __asm__ volatile("madd %0, %1, %2, %3"
                   : "=r"(result)
                   : "r"(b), "r"(c), "r"(a));
  return (int)(result == a + b * c);
}

// 32-bit MADD: W = Wa + Wn * Wm
int __attribute__((noinline)) test_madd32(int a, int b, int c) {
  int result;
  __asm__ volatile("madd %w0, %w1, %w2, %w3"
                   : "=r"(result)
                   : "r"(b), "r"(c), "r"(a));
  return result == (int)((long)a + (long)b * (long)c);
}

// 64-bit MSUB: X = Ra - Rn * Rm
int __attribute__((noinline)) test_msub64(long a, long b, long c) {
  long result;
  __asm__ volatile("msub %0, %1, %2, %3"
                   : "=r"(result)
                   : "r"(b), "r"(c), "r"(a));
  return (int)(result == a - b * c);
}

// 32-bit MSUB: W = Wa - Wn * Wm
int __attribute__((noinline)) test_msub32(int a, int b, int c) {
  int result;
  __asm__ volatile("msub %w0, %w1, %w2, %w3"
                   : "=r"(result)
                   : "r"(b), "r"(c), "r"(a));
  return result == (int)((long)a - (long)b * (long)c);
}

// MUL = MADD with Ra=XZR (Rd = Rn * Rm)
int __attribute__((noinline)) test_mul64(long a, long b) {
  long result;
  __asm__ volatile("mul %0, %1, %2" : "=r"(result) : "r"(a), "r"(b));
  return (int)(result == a * b);
}

// MNEG = MSUB with Ra=XZR (Rd = -(Rn * Rm))
int __attribute__((noinline)) test_mneg64(long a, long b) {
  long result;
  __asm__ volatile("mneg %0, %1, %2" : "=r"(result) : "r"(a), "r"(b));
  return (int)(result == -(a * b));
}

// Comprehensive test function (ensure > 72 bytes to support standard mode)
int __attribute__((noinline)) test_madd_all(void) {
  int pass = 0;
  int total = 0;

  // MADD 64-bit
  total++;
  if (test_madd64(100, 7, 13))
    pass++; // 100 + 7*13 = 191
  else
    printf("  FAIL: madd64\n");

  // MADD 32-bit
  total++;
  if (test_madd32(50, 6, 8))
    pass++; // 50 + 6*8 = 98
  else
    printf("  FAIL: madd32\n");

  // MSUB 64-bit
  total++;
  if (test_msub64(200, 5, 10))
    pass++; // 200 - 5*10 = 150
  else
    printf("  FAIL: msub64\n");

  // MSUB 32-bit
  total++;
  if (test_msub32(100, 3, 9))
    pass++; // 100 - 3*9 = 73
  else
    printf("  FAIL: msub32\n");

  // MUL 64-bit
  total++;
  if (test_mul64(12345, 67890))
    pass++; // 12345 * 67890
  else
    printf("  FAIL: mul64\n");

  // MNEG 64-bit
  total++;
  if (test_mneg64(11, 22))
    pass++; // -(11*22) = -242
  else
    printf("  FAIL: mneg64\n");

  // Edge cases
  total++;
  if (test_madd64(0, 0, 0))
    pass++; // 0 + 0*0 = 0
  else
    printf("  FAIL: madd64 zero\n");

  total++;
  if (test_madd64(-1, -1, -1))
    pass++; // -1 + (-1)*(-1) = 0
  else
    printf("  FAIL: madd64 neg\n");

  total++;
  if (test_msub64(0, 100, 100))
    pass++; // 0 - 100*100 = -10000
  else
    printf("  FAIL: msub64 neg result\n");

  if (pass == total)
    return 0;
  printf("  %d/%d passed\n", pass, total);
  return 1;
}

int main(void) {
  int ret = test_madd_all();
  if (ret == 0) {
    printf("MADD PASS\n");
    return 0;
  }
  printf("MADD FAIL\n");
  return 1;
}
