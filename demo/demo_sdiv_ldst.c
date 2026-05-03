/*
 * demo_sdiv_ldst.c — Test SDIV + load/store variants
 *
 * Coverage:
 *   - SDIV (signed division)
 *   - LDRH/STRH (reg offset)
 *   - LDRSB/LDRSH/LDRSW (reg offset)
 *   - LDURH/STURH (unscaled)
 *   - LDURSB/LDURSH/LDURSW (unscaled)
 *
 * Compile: aarch64-linux-gnu-gcc -O2 -o demo_sdiv_ldst demo_sdiv_ldst.c
 */
#include <stdint.h>
#include <stdio.h>
#include <string.h>


__attribute__((noinline)) int64_t test_sdiv(int64_t a, int64_t b) {
  if (b == 0)
    return 0;
  return a / b; // SDIV
}

__attribute__((noinline)) int32_t test_sdiv_w(int32_t a, int32_t b) {
  if (b == 0)
    return 0;
  return a / b; // SDIV (32-bit)
}

__attribute__((noinline)) uint16_t test_ldrh_reg(uint16_t *arr, int64_t idx) {
  return arr[idx]; // LDRH (reg offset, LSL #1)
}

__attribute__((noinline)) void test_strh_reg(uint16_t *arr, int64_t idx,
                                             uint16_t val) {
  arr[idx] = val; // STRH (reg offset, LSL #1)
}

__attribute__((noinline)) int64_t test_ldrsb_reg(int8_t *arr, int64_t idx) {
  return (int64_t)arr[idx]; // LDRSB (reg offset)
}

__attribute__((noinline)) int64_t test_ldrsh_reg(int16_t *arr, int64_t idx) {
  return (int64_t)arr[idx]; // LDRSH (reg offset)
}

__attribute__((noinline)) int64_t test_ldrsw_reg(int32_t *arr, int64_t idx) {
  return (int64_t)arr[idx]; // LDRSW (reg offset)
}

__attribute__((noinline)) uint16_t test_ldurh(uint16_t *p) {
  // The compiler may use LDURH to handle unaligned or small offsets
  char *base = (char *)p;
  return *(uint16_t *)(base + 3); // unaligned offset → LDURH
}

__attribute__((noinline)) void test_sturh(uint16_t *p, uint16_t val) {
  char *base = (char *)p;
  *(uint16_t *)(base + 3) = val; // unaligned offset → STURH
}

__attribute__((noinline)) int64_t test_ldursb(int8_t *p) {
  char *base = (char *)p;
  return (int64_t)*(int8_t *)(base - 1); // negative offset → LDURSB
}

__attribute__((noinline)) int64_t test_ldursh(int16_t *p) {
  char *base = (char *)p;
  return (int64_t)*(int16_t *)(base + 2); // small offset → LDURSH
}

__attribute__((noinline)) int64_t test_ldursw(int32_t *p) {
  char *base = (char *)p;
  return (int64_t)*(int32_t *)(base + 4); // small offset → LDURSW
}

int main(void) {
  int pass = 0, fail = 0;

#define CHECK(name, got, expect)                                               \
  do {                                                                         \
    if ((got) == (expect)) {                                                   \
      pass++;                                                                  \
    } else {                                                                   \
      printf("FAIL: %s got=%ld expect=%ld\n", name, (long)(got),               \
             (long)(expect));                                                  \
      fail++;                                                                  \
    }                                                                          \
  } while (0)

  // SDIV tests
  CHECK("sdiv(100,7)", test_sdiv(100, 7), 14);
  CHECK("sdiv(-100,7)", test_sdiv(-100, 7), -14);
  CHECK("sdiv(100,-7)", test_sdiv(100, -7), -14);
  CHECK("sdiv(-100,-7)", test_sdiv(-100, -7), 14);
  CHECK("sdiv(0,5)", test_sdiv(0, 5), 0);
  CHECK("sdiv(5,0)", test_sdiv(5, 0), 0);

  // SDIV 32-bit
  CHECK("sdiv_w(100,7)", test_sdiv_w(100, 7), 14);
  CHECK("sdiv_w(-99,10)", test_sdiv_w(-99, 10), -9);

  // LDRH/STRH register offset
  uint16_t arr16[4] = {0x1234, 0x5678, 0xABCD, 0xEF01};
  CHECK("ldrh_reg[0]", test_ldrh_reg(arr16, 0), 0x1234);
  CHECK("ldrh_reg[2]", test_ldrh_reg(arr16, 2), 0xABCD);
  test_strh_reg(arr16, 1, 0xBEEF);
  CHECK("strh_reg[1]", arr16[1], 0xBEEF);

  // LDRSB register offset
  int8_t arr8[] = {-128, 0, 127, -1};
  CHECK("ldrsb_reg[0]", test_ldrsb_reg(arr8, 0), -128);
  CHECK("ldrsb_reg[2]", test_ldrsb_reg(arr8, 2), 127);
  CHECK("ldrsb_reg[3]", test_ldrsb_reg(arr8, 3), -1);

  // LDRSH register offset
  int16_t arr16s[] = {-32768, 0, 32767, -1};
  CHECK("ldrsh_reg[0]", test_ldrsh_reg(arr16s, 0), -32768);
  CHECK("ldrsh_reg[2]", test_ldrsh_reg(arr16s, 2), 32767);

  // LDRSW register offset
  int32_t arr32s[] = {-2147483647, 0, 2147483647, -1};
  CHECK("ldrsw_reg[0]", test_ldrsw_reg(arr32s, 0), -2147483647);
  CHECK("ldrsw_reg[3]", test_ldrsw_reg(arr32s, 3), -1);

  printf("Results: %d passed, %d failed\n", pass, fail);
  return fail;
}
