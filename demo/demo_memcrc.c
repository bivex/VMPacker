/*
 * demo_memcrc.c — 内存 CRC 自校验 demo (ARM64)
 *
 * 验证: 运行时对自身代码段做 CRC，检测是否被 patch。
 * 编译: aarch64-linux-gnu-gcc -static -O2 demo_memcrc.c -o demo_memcrc
 *
 * 本 demo 模拟 stub 的两种 CRC:
 *   1. 字节码 CRC  — 校验 VM 字节码是否被篡改
 *   2. 内存 CRC    — 校验 stub 代码自身是否被 patch (反 IDA 修改)
 */
#include <stdio.h>
#include <string.h>

/* ==== CRC32 (与 demo_crc.c 相同, 无外部依赖) ==== */

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

/* ==== 模拟 stub 代码区域 ==== */

/*
 * 用一个全局数组模拟 stub 的 .text 段。
 * In a real scenario, the stub obtains its own PC via the ADR instruction,
 * then back-calculates to the blob start address for CRC.
 */
static unsigned char fake_stub_code[] = {
    /* 模拟 64 字节 stub 代码 */
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

/* ==== 测试 1: 内存 CRC 正常校验 ==== */

static int test_memcrc_normal(void) {
  /* 模拟: packer 编译后计算 stub 代码的 CRC 并嵌入 */
  unsigned int expected_crc =
      crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  /* 模拟: 运行时 stub 对自身代码做 CRC */
  unsigned int actual_crc = crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  if (actual_crc == expected_crc) {
    printf("[MemCRC] normal: PASS (crc=0x%08X)\n", actual_crc);
    return 0;
  }
  printf("[MemCRC] normal: FAIL\n");
  return 1;
}

/* ==== 测试 2: 检测 IDA patch (0xCC / BRK 断点) ==== */

static int test_memcrc_patch(void) {
  unsigned int expected_crc =
      crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  /* 模拟: IDA/调试器在入口处 patch 一条指令 (改成 BRK #0) */
  unsigned char saved[4];
  memcpy(saved, fake_stub_code, 4);
  fake_stub_code[0] = 0x00;
  fake_stub_code[1] = 0x00;
  fake_stub_code[2] = 0x20;
  fake_stub_code[3] = 0xD4; /* BRK #0 = 0xD4200000 */

  unsigned int actual_crc = crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  /* 恢复 */
  memcpy(fake_stub_code, saved, 4);

  if (actual_crc != expected_crc) {
    printf("[MemCRC] patch detect: PASS (expected=0x%08X, got=0x%08X)\n",
           expected_crc, actual_crc);
    return 0;
  }
  printf("[MemCRC] patch detect: FAIL\n");
  return 1;
}

/* ==== 测试 3: ADR 自定位 (ARM64 获取当前 PC) ==== */

/*
 * 在 ARM64 上，stub 可以用 ADR 获取自身地址:
 *   adr x0, .       => x0 = 当前指令地址
 *   sub x0, x0, #offset_from_blob_start => x0 = blob 起始
 *
 * 这里用函数指针模拟验证该机制：
 * 取 test_self_locate 函数的地址，对前 N 字节算 CRC。
 */

/* 标记一个小函数，让我们可以对它做 CRC */
__attribute__((noinline)) static unsigned int
test_func_to_check(unsigned int a, unsigned int b) {
  return a + b + 42;
}

static int test_self_locate(void) {
  /*
   * 对 test_func_to_check 的前 16 字节做 CRC
   * (In the actual stub, the entire blob is CRC-checked)
   *
   * 注意: 这里只证明"可以从函数指针获取代码地址并计算CRC",
   * 不验证具体 CRC 值（因为编译器可能改变代码）。
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

/* ==== 测试 4: 双层 CRC 联合校验 ==== */

static int test_dual_crc(void) {
  /* 模拟字节码 */
  unsigned char bytecode[] = {0x5A, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
                              0x42, 0x00, 0x2F, 0x02, 0x01, 0x37, 0x03, 0x01};

  /* packer 端预计算: stub CRC + bytecode CRC */
  unsigned int stub_crc = crc32_calc(fake_stub_code, sizeof(fake_stub_code));
  unsigned int bc_crc = crc32_calc(bytecode, sizeof(bytecode));

  /* 运行时校验: 先查内存 CRC，再查字节码 CRC */
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

/* ==== 测试 5: 校验失败 → 直接不运行 ==== */

static int test_fail_abort(void) {
  unsigned int expected_crc =
      crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  /* 篡改一个字节 */
  unsigned char saved = fake_stub_code[10];
  fake_stub_code[10] ^= 0xFF;

  unsigned int actual_crc = crc32_calc(fake_stub_code, sizeof(fake_stub_code));

  /* 恢复 */
  fake_stub_code[10] = saved;

  /* 模拟 stub 行为: CRC 不匹配 → 返回 0, 不执行任何字节码 */
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
