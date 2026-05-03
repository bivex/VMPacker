/**
 * simple_app.cpp - VMPacker test program
 *
 * Contains calculation logic of common instructions and log printing, used to verify if the program can run normally after VMP protection.
 * The core function log2Console will serve as the protection target.
 *
 * Two compilation modes (see test/Makefile):
 *   1) Static mode: gcc simple_app.cpp -o simple_app (only testing executable file protection)
 *   2) SO mode:  gcc -DUSE_SHARED_LIB simple_app.cpp -lvmptest (testing .so protection as well)
 *
 * Protection: go run ./cmd/vmpacker/ -func log2Console -v -debug -o /tmp/simple_app_protected simple_app
 * Running: ./simple_app_protected
 */

#include <stdio.h>
#include <string.h>

#ifdef USE_SHARED_LIB
#include "libvmptest.h"
#endif

/**
 * log2Console - Core function simulating log output to the console
 *
 * Uses common instructions: arithmetic (ADD/SUB/MUL), logic (AND/OR/XOR),
 * comparison and branching, loops, memory access, etc.
 * This function will be protected by VMP.
 * extern "C" Ensures the symbol name is log2Console, making it easy for vmpacker -func to match.
 */
extern "C" int log2Console(const char* tag, int level, int value) {
  if (tag == 0)
    return -1;

  /* Simple hash: performing operations on each byte of the tag */
  unsigned int hash = 0x811c9dc5;
  int len = 0;
  const char* p = tag;
  while (*p != 0 && len < 64) {
    hash = (hash * 31) ^ (unsigned char)(*p);
    len++;
    p++;
  }

  /* Arithmetic operations: value * 7 + hash % 100 */
  int a = value * 7;
  int b = (int)(hash % 100);
  int result = a + b;

  /* Conditional branching */
  if (level > 0) {
    result += 10;
  } else {
    result -= 5;
  }

  /* Loop: accumulating 0..4 */
  int sum = 0;
  for (int i = 0; i < 5; i++) {
    sum += i;
  }
  result += sum;

  /* Bitwise operations */
  result = (result & 0xFFF) | ((result >> 12) << 12);

  return result;
}

/**
 * computeAndLog - Calculate and format log string (for main to call)
 */
void computeAndLog(const char* msg, int x, int y) {
  int z = log2Console(msg, 1, x + y);
  printf("[LOG] %s: x=%d, y=%d => result=%d\n", msg, x, y, z);
}

int main(int argc, char* argv[]) {
  printf("=== VMPacker Simple App Test ===\n");

  /* Call the protected log2Console (within the executable) */
  computeAndLog("init", 10, 20);
  computeAndLog("step1", 5, 15);
  computeAndLog("step2", 100, 200);

  int final_result = log2Console("done", 1, 42);
  printf("[LOG] Final result: %d\n", final_result);

#ifdef USE_SHARED_LIB
  /* Call the protected function in the shared library */
  printf("\n--- Shared library (.so) tests ---\n");

  int r1 = vmp_compute("hello", 0, 42);
  int r2 = vmp_compute("hello", 1, 42);
  int r3 = vmp_compute("hello", 2, 42);
  int r4 = vmp_compute("world", 0, 42);
  int r5 = vmp_compute((const char*)0, 0, 0);
  printf("[SO] vmp_compute(\"hello\",0,42) = %d\n", r1);
  printf("[SO] vmp_compute(\"hello\",1,42) = %d\n", r2);
  printf("[SO] vmp_compute(\"hello\",2,42) = %d\n", r3);
  printf("[SO] vmp_compute(\"world\",0,42) = %d\n", r4);
  printf("[SO] vmp_compute(NULL,0,0)      = %d\n", r5);

  int v1 = vmp_verify_key("ABCD-1234-EFGH", 100);
  int v2 = vmp_verify_key("short", 100);
  int v3 = vmp_verify_key((const char*)0, 100);
  printf("[SO] vmp_verify_key(\"ABCD-1234-EFGH\",100) = %d\n", v1);
  printf("[SO] vmp_verify_key(\"short\",100)           = %d\n", v2);
  printf("[SO] vmp_verify_key(NULL,100)               = %d\n", v3);

  /* --- Functions with external libc calls (PLT) --- */
  printf("\n--- External call tests (PLT) ---\n");

  char md5buf[64];
  int md5ret;

  md5ret = vmp_md5_hex("hello", md5buf, sizeof(md5buf));
  printf("[SO] vmp_md5_hex(\"hello\")     = %s  (rc=%d)\n", md5buf, md5ret);

  md5ret = vmp_md5_hex("", md5buf, sizeof(md5buf));
  printf("[SO] vmp_md5_hex(\"\")          = %s  (rc=%d)\n", md5buf, md5ret);

  md5ret = vmp_md5_hex("The quick brown fox jumps over the lazy dog", md5buf, sizeof(md5buf));
  printf("[SO] vmp_md5_hex(\"The q...\")  = %s  (rc=%d)\n", md5buf, md5ret);

  md5ret = vmp_md5_hex((const char*)0, md5buf, sizeof(md5buf));
  printf("[SO] vmp_md5_hex(NULL)         rc=%d\n", md5ret);

  char procname[128];
  int pnret = vmp_get_process_name(procname, sizeof(procname));
  printf("[SO] vmp_get_process_name()   = \"%s\"  (len=%d)\n", procname, pnret);
#endif

  printf("=== Test completed successfully ===\n");
  return 0;
}
