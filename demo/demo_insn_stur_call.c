/*
 * demo_insn_stur_call.c — Test STUR negative offset + BL cross-function call
 *
 * Compile: aarch64-linux-gnu-gcc -O1 -static -o demo_stur_call
 * demo_insn_stur_call.c Run: ./demo_stur_call → "STUR_CALL PASS"
 */
#include <stdio.h>
#include <string.h>

/* External helper function (BL call target) */
__attribute__((noinline)) int helper_add(int a, int b) { return a + b; }

__attribute__((noinline)) int helper_mul(int a, int b) { return a * b; }

/* Test STUR negative offset: write bytes backwards */
__attribute__((noinline)) void test_stur_neg(unsigned char *buf, int len) {
  /* The compiler may use STUR/STURB to implement operations like buf[i-1] */
  for (int i = len; i > 0; i--) {
    buf[i] = buf[i - 1]; /* Shift right by one position */
  }
  buf[0] = 0xFF;
}

/* Test cross-function BL call + STUR */
__attribute__((noinline)) int test_call_stur(int x) {
  int a = helper_add(x, 10);
  int b = helper_mul(a, 3);
  unsigned char tmp[8] = {1, 2, 3, 4, 5, 6, 7, 8};
  test_stur_neg(tmp, 7);
  int sum = 0;
  for (int i = 0; i < 8; i++)
    sum += tmp[i];
  return b + sum;
}

int main(void) {
  int r = test_call_stur(5);
  /*
   * x=5 → a=5+10=15 → b=15*3=45
   * tmp initial = {1,2,3,4,5,6,7,8}
   * test_stur_neg: shift right by 1 position → {0xFF,1,2,3,4,5,6,7}
   * sum = 0xFF+1+2+3+4+5+6+7 = 255+28 = 283
   * result = 45 + 283 = 328
   */
  if (r == 328) {
    printf("STUR_CALL PASS\n");
  } else {
    printf("STUR_CALL FAIL: got %d, expect 328\n", r);
  }
  return 0;
}
