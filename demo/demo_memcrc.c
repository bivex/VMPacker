/*
 * demo_memcrc.c — Memory CRC self-check demo (ARM64)
 *
 * Verify: Perform CRC on its own code segment at runtime to detect if it has been patched.
 * Compile: aarch64-linux-gnu-gcc -static -O2 demo_memcrc.c -o demo_memcrc
 *
 * This demo simulates two types of CRC for the stub:
 *   1. Bytecode CRC - Verify if the VM bytecode has been tampered with
 *   2. Memory CRC - Verify if the stub code itself has been patched (anti-IDA modification)
 */
#include <stdio.h>
#include <string.h>

/* ==== CRC32 (Same as demo_crc.c, no external dependencies) ==== */

static unsigned int crc32_tab[256];
static int crc32_ready = 0;

static void crc32_init(void) {
  unsigned int i, j, c;
  for (i = 0; i < 256; i++) {
    c = i;
    for (j = 0; j < 8; j++)
      c = (c & 1) ? (0xEDB88320u ^ (c >> 1)) : (c >> 1);
    crc32_tab[i] = c;
  }
  crc32_ready = 1;
}

static unsigned int crc32_calc(const unsigned char *data, unsigned int len) {
  unsigned int crc = 0xFFFFFFFFu, i;
  if (!crc32_ready)
    crc32_init();
  for (i = 0; i < len; i++)
    crc = crc32_tab[(crc ^ data[i]) & 0xFF] ^ (crc >> 8);
  return crc ^ 0xFFFFFFFFu;
}

/* ==== Simulate stub code area ==== */

/*
 * Use a global array to simulate the .text segment of the stub.
 * In a real scenario, the stub obtains its own PC via the ADR instruction,
 * then back-calculates to the blob start address for CRC.
 */
static unsigned char fake_stub_code[] = {
    /* Simulate 64 bytes of stub code */
    0xFD, 0x7B, 0xBF, 0xA9, /* stp x29, x30, [sp, #-16]! */
    0xFD, 0x03, 0x00, 0x91, /* mov x29, sp               */
    0x00, 0x00, 0x80, 0xD2, /* mov x0, #0                */
    0xE8, 0x03, 0x1F, 0xAA, /* mov x8, xzr               */
    0x01, 0x00, 0x00, 0xD4, /* svc #0                    */
    0xFD, 0x7B, 0xC1, 0xA8, /* ldp x29, x30, [sp], #16   */
    0xC0, 0x03, 0x5F, 0xD6, /* ret                       */
    0x00, 0x00, 0x00, 0x00, /* padding                   */
    0xAA, 0xBB, 0xCC, 0xDD, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
    0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE,
    0xBA, 0xBE, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
};

/* ==== Test 1: Normal memory CRC check ==== */

static int test_memcrc_normal(void) {
  /* Simulate: Packer calculates the CRC of the stub code after compilation and embeds it */
  unsigned int expected_crc =
      crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  /* Simulate: Stub performs CRC on its own code at runtime */
  unsigned int actual_crc = crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  if (actual_crc == expected_crc) {
    printf("[MemCRC] normal: PASS (crc=0x%08X)\n", actual_crc);
    return 0;
  }
  printf("[MemCRC] normal: FAIL\n");
  return 1;
}

/* ==== Test 2: Detect IDA patch (0xCC / BRK breakpoint) ==== */

static int test_memcrc_patch(void) {
  unsigned int expected_crc =
      crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  /* Simulate: IDA/debugger patches an instruction at the entry (change to BRK #0) */
  unsigned char saved[4];
  memcpy(saved, fake_stub_code, 4);
  fake_stub_code[0] = 0x00;
  fake_stub_code[1] = 0x00;
  fake_stub_code[2] = 0x20;
  fake_stub_code[3] = 0xD4; /* BRK #0 = 0xD4200000 */

  unsigned int actual_crc = crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  /* Restore */
  memcpy(fake_stub_code, saved, 4);

  if (actual_crc != expected_crc) {
    printf("[MemCRC] patch detect: PASS (expected=0x%08X, got=0x%08X)\n",
           expected_crc, actual_crc);
    return 0;
  }
  printf("[MemCRC] patch detect: FAIL\n");
  return 1;
}

/* ==== Test 3: ADR self-location (ARM64 gets the current PC) ==== */

/*
 * On ARM64, the stub can use ADR to get its own address:
 *   adr x0, .       => x0 = current instruction address
 *   sub x0, x0, #offset_from_blob_start => x0 = blob start
 *
 * Here, a function pointer is used to simulate and verify this mechanism:
 * Take the address of the test_func_to_check function and calculate the CRC for the first N bytes.
 */

/* Mark a small function so we can perform a CRC on it */
__attribute__((noinline)) static unsigned int
test_func_to_check(unsigned int a, unsigned int b) {
  return a + b + 42;
}

static int test_self_locate(void) {
  /*
   * Perform CRC on the first 16 bytes of test_func_to_check
   * (In the actual stub, the entire blob is CRC-checked)
   *
   * Note: This only proves "can get code address from function pointer and calculate CRC",
   * does not verify the specific CRC value (as the compiler may change the code).
   */
  const unsigned char *code_ptr = (const unsigned char *)test_func_to_check;
  unsigned int crc1 = crc32_calc(code_ptr, 16);
  unsigned int crc2 = crc32_calc(code_ptr, 16);

  if (crc1 == crc2 && crc1 != 0) {
    printf("[MemCRC] self-locate: PASS (func@%p, crc=0x%08X)\n",
           (void *)code_ptr, crc1);
    return 0;
  }
  printf("[MemCRC] self-locate: FAIL\n");
  return 1;
}

/* ==== Test 4: Combined dual-layer CRC check ==== */

static int test_dual_crc(void) {
  /* Simulate bytecode */
  unsigned char bytecode[] = {0x5A, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
                              0x42, 0x00, 0x2F, 0x02, 0x01, 0x37, 0x03, 0x01};

  /* Pre-calculation on the packer side: stub CRC + bytecode CRC */
  unsigned int stub_crc = crc32_calc(fake_stub_code, sizeof(fake_stub_code));
  unsigned int bc_crc = crc32_calc(bytecode, sizeof(bytecode));

  /* Runtime check: first check memory CRC, then check bytecode CRC */
  unsigned int check1 = crc32_calc(fake_stub_code, sizeof(fake_stub_code));
  unsigned int check2 = crc32_calc(bytecode, sizeof(bytecode));

  int pass = (check1 == stub_crc) && (check2 == bc_crc);

  if (pass) {
    printf("[MemCRC] dual CRC: PASS (stub=0x%08X, bc=0x%08X)\n", check1,
           check2);
    return 0;
  }
  printf("[MemCRC] dual CRC: FAIL\n");
  return 1;
}

/* ==== Test 5: Check failed → Do not run ==== */

static int test_fail_abort(void) {
  unsigned int expected_crc =
      crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  /* Tamper with one byte */
  unsigned char saved = fake_stub_code[10];
  fake_stub_code[10] ^= 0xFF;

  unsigned int actual_crc = crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  /* Restore */
  fake_stub_code[10] = saved;

  /* Simulate stub behavior: CRC mismatch → return 0, do not execute any bytecode */
  int would_run = (actual_crc == expected_crc);

  if (!would_run) {
    printf("[MemCRC] fail-abort: PASS (tampered → refused to run)\n");
    return 0;
  }
  printf("[MemCRC] fail-abort: FAIL (should have refused)\n");
  return 1;
}

int main(void) {
  printf("=== Memory CRC Self-Check Demo ===\n\n");

  int failures = 0;
  failures += test_memcrc_normal();
  failures += test_memcrc_patch();
  failures += test_self_locate();
  failures += test_dual_crc();
  failures += test_fail_abort();

  printf("\n=== Result: %s (%d failures) ===\n",
         failures == 0 ? "ALL PASS" : "SOME FAILED", failures);
  return failures;
}
