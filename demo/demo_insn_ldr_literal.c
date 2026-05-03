/*
 * demo/demo_insn_ldr_literal.c
 * Test LDR literal (PC-relative) instructions
 *
 * ARM64 encoding: opc:011:V:00:imm19:Rt
 *   opc=01 → LDR X (64-bit)
 *   opc=00 → LDR W (32-bit)
 *   opc=10 → LDRSW  (32-bit sign-extended)
 *
 * Compile: aarch64-linux-gnu-gcc -O1 -static -o demo/demo_insn_ldr_literal
 * demo/demo_insn_ldr_literal.c
 */
#include <stdint.h>
#include <stdio.h>


/* Global constants - the compiler will load these using the LDR literal method */
static const uint64_t g_magic64 = 0xDEADBEEFCAFEBABEULL;
static const uint32_t g_magic32 = 0x12345678;
static const int32_t g_signed = -42;

/*
 * test_ldr_literal: Trigger LDR literal instructions
 * Function is large enough (>72B) to support VMP trampoline
 */
__attribute__((noinline)) int test_ldr_literal(void) {
  volatile int pass = 1;

  /* Test 1: LDR X (64-bit literal) */
  volatile uint64_t v64 = g_magic64; /* LDR Xt, [PC, #off] */
  if (v64 != 0xDEADBEEFCAFEBABEULL) {
    printf("FAIL 64: got 0x%llx\n", (unsigned long long)v64);
    pass = 0;
  }

  /* Test 2: LDR W (32-bit literal) */
  volatile uint32_t v32 = g_magic32; /* LDR Wt, [PC, #off] */
  if (v32 != 0x12345678) {
    printf("FAIL 32: got 0x%x\n", v32);
    pass = 0;
  }

  /* Test 3: LDRSW (sign-extended 32→64) */
  volatile int64_t vsw = (int64_t)g_signed; /* LDRSW Xt, [PC, #off] */
  if (vsw != -42) {
    printf("FAIL SW: got %lld\n", (long long)vsw);
    pass = 0;
  }

  /* Extra calculations to ensure the function is large enough */
  volatile uint64_t check = v64 ^ ((uint64_t)v32 << 32);
  volatile int64_t check2 = vsw * 100 + (int64_t)v32;
  if (check == 0 && check2 == 0) {
    /* Impossible to reach, but prevents compiler optimization */
    pass = 0;
  }

  return pass;
}

int main(void) {
  int ret = test_ldr_literal();
  if (ret) {
    printf("LDR_LITERAL PASS\n");
    return 0;
  }
  printf("LDR_LITERAL FAIL\n");
  return 1;
}
