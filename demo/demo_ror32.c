/*
 * demo_ror32.c — 验证 ROR 32-bit bug
 *
 * Contains rotl32() + chained XOR decrypt logic—identical pattern to
 * decrypt_crc_info in check_stub_crc.
 *
 * ARM64 编译后 rotl32(v,7) 会生成:
 *   EOR Wd, Wn, Wm, ROR #25
 * This is a 32-bit ROR. If the VM's h_ror performs a 64-bit ROR, the result
 * will be incorrect.
 *
 * 编译: aarch64-linux-gnu-gcc -O2 -march=armv8-a -static -o demo_ror32
 * demo_ror32.c 反汇编验证: aarch64-linux-gnu-objdump -d demo_ror32 | grep -A30
 * '<my_decrypt>'
 */

/* 避免 libc 依赖的 syscall 包装 */
static long my_write(int fd, const void *buf, long count) {
  register long x8 __asm__("x8") = 64;
  register long x0 __asm__("x0") = fd;
  register long x1 __asm__("x1") = (long)buf;
  register long x2 __asm__("x2") = count;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2) : "memory");
  return x0;
}

static void my_exit(int code) {
  register long x8 __asm__("x8") = 93;
  register long x0 __asm__("x0") = code;
  __asm__ volatile("svc #0" : : "r"(x8), "r"(x0));
  __builtin_unreachable();
}

typedef unsigned int u32;
typedef unsigned char u8;

/* Exactly the same rotl32 as in integrity.c */
static u32 rotl32(u32 v, int n) { return (v << n) | (v >> (32 - n)); }

/* 与 decrypt_crc_info 相同的解密链 */
static void my_decrypt(u32 *key, u32 *addr, u32 *size, u32 *hash) {
  u32 dec;
  dec = *addr ^ *key;
  *key = rotl32(*key, 7) ^ dec;
  *addr = dec;

  dec = *size ^ *key;
  *key = rotl32(*key, 7) ^ dec;
  *size = dec;

  dec = *hash ^ *key;
  *key = rotl32(*key, 7) ^ dec;
  *hash = dec;
}

/* 十六进制输出 */
static void print_hex(u32 v) {
  char buf[11];
  buf[0] = '0';
  buf[1] = 'x';
  for (int i = 9; i >= 2; i--) {
    int d = v & 0xF;
    buf[i] = d < 10 ? '0' + d : 'A' + d - 10;
    v >>= 4;
  }
  buf[10] = ' ';
  my_write(1, buf, 11);
}

static void print_str(const char *s) {
  int len = 0;
  while (s[len])
    len++;
  my_write(1, s, len);
}

/* 测试入口 */
__attribute__((noinline)) int my_test(void) {
  /* 使用 check_stub_crc 实际使用的 salt 值 */
  u32 key = 0x59EEE963;

  /* 模拟加密的 CRC_INFO 条目（addr, size, hash 各 4 字节） */
  /* 先用原始值加密，然后解密验证 */
  u32 orig_addr = 0x00001000;
  u32 orig_size = 0x00000612;
  u32 orig_hash = 0xDEADBEEF;

  /* 加密：encrypt = value ^ key, key = rotl32(key,7) ^ value */
  u32 ekey = key;
  u32 enc_addr = orig_addr ^ ekey;
  ekey = rotl32(ekey, 7) ^ orig_addr;
  u32 enc_size = orig_size ^ ekey;
  ekey = rotl32(ekey, 7) ^ orig_size;
  u32 enc_hash = orig_hash ^ ekey;

  /* 解密 */
  u32 dkey = key;
  u32 dec_addr = enc_addr;
  u32 dec_size = enc_size;
  u32 dec_hash = enc_hash;
  my_decrypt(&dkey, &dec_addr, &dec_size, &dec_hash);

  /* 验证 */
  print_str("key=");
  print_hex(key);
  print_str("\norig: ");
  print_hex(orig_addr);
  print_hex(orig_size);
  print_hex(orig_hash);
  print_str("\ndec:  ");
  print_hex(dec_addr);
  print_hex(dec_size);
  print_hex(dec_hash);
  print_str("\n");

  int pass = (dec_addr == orig_addr) && (dec_size == orig_size) &&
             (dec_hash == orig_hash);
  if (pass) {
    print_str("RESULT: PASS\n");
  } else {
    print_str("RESULT: FAIL\n");
    print_str("addr ");
    print_str(dec_addr == orig_addr ? "OK" : "MISMATCH");
    print_str(" size ");
    print_str(dec_size == orig_size ? "OK" : "MISMATCH");
    print_str(" hash ");
    print_str(dec_hash == orig_hash ? "OK" : "MISMATCH");
    print_str("\n");
  }
  return pass ? 0 : 1;
}

void _start(void) {
  int rc = my_test();
  my_exit(rc);
}
