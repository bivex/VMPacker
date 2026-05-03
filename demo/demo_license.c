/*
 * demo_license.c — VMP end-to-end test demo
 *
 * Compile: aarch64-linux-gnu-gcc -O1 -o demo_license demo_license.c
 * Protect: vmpacker -func check_license -v -o demo_license.vmp demo_license
 * Run:     ./demo_license.vmp 12345678
 *          ./demo_license.vmp 00000000
 */

#include <stdio.h>
#include <string.h>

/*
 * check_license — Simple license verification function
 *
 * This function will be protected by VMP. It uses instructions already supported by the translator:
 *   - MOV (MOVZ/MOVK)
 *   - ADD/SUB/MUL/XOR/AND/OR/LSL/LSR
 *   - CMP + B.cond
 *   - LDR/STR (byte load)
 *   - Loops
 *   - RET
 *
 * Algorithm: Hash each byte of the input key and compare with the expected value.
 *   hash = 0
 *   for each byte b in key:
 *       hash = (hash * 31 + b) ^ 0x5A
 *   return hash == EXPECTED
 */
int check_license(const char *key) {
  if (key == 0)
    return 0;

  unsigned long hash = 0;
  int len = 0;

  /* Calculate length (avoid strlen to prevent external calls) */
  const char *p = key;
  while (*p != 0) {
    len++;
    p++;
  }

  /* Key length must be 8 */
  if (len != 8)
    return 0;

  /* Calculate hash */
  for (int i = 0; i < 8; i++) {
    unsigned char b = (unsigned char)key[i];
    hash = (hash * 31 + b) ^ 0x5A;
  }

  /* Expected value: check_license("12345678") == 1 */
  unsigned long expected = 0;
  const char *valid = "12345678";
  for (int i = 0; i < 8; i++) {
    unsigned char b = (unsigned char)valid[i];
    expected = (expected * 31 + b) ^ 0x5A;
  }

  return (hash == expected) ? 1 : 0;
}

int main(int argc, char *argv[]) {
  if (argc < 2) {
    printf("Usage: %s <license-key>\n", argv[0]);
    return 1;
  }

  int result = check_license(argv[1]);

  if (result) {
    printf("[+] License valid!\n");
  } else {
    printf("[-] License invalid.\n");
  }

  return result ? 0 : 1;
}
