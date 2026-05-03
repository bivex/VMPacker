/*
 * demo_chain_crypt.c — SectionCryptor 链式加密 demo
 *
 * 设计思想 (对标 VMProtect SectionCryptor):
 *   - 字节码被划分为固定大小的 segment (如 64 字节)
 *   - 每个 segment 有独立的 XOR key (密钥链: key[i+1] = crc32(key[i]))
 *   - 运行时只解密当前 segment, 同时回加密上一个 segment
 *   - At most 1 segment is in plaintext in memory at any time → prevents memory dumps
 *
 * 数据格式 (packer 生成):
 *   [seg_count:u32]            segment 数量
 *   [seg_size:u32]             每段大小 (最后一段可能不足)
 *   [init_key:u32]             初始密钥 (种子)
 *   [encrypted_data...]        分段加密的字节码
 *
 * 密钥链:
 *   key[0] = init_key
 *   key[i] = crc32(&key[i-1], 4)   每段密钥由前一段密钥的 CRC 派生
 *
 * 加密方式: 每段内按 u32 对齐做 XOR:
 *   for j in 0..seg_words: data[j] ^= rotate(key, j)
 *
 * 编译: aarch64-linux-gnu-gcc -static -O2 demo/demo_chain_crypt.c -o
 * build/demo_chain_crypt
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>


typedef unsigned char u8;
typedef unsigned int u32;

/* ---- CRC32 无表位运算 (与 stub vm_crc.h 一致) ---- */
static u32 crc32_calc(const u8 *data, u32 len) {
  u32 crc = 0xFFFFFFFFu, i, j;
  for (i = 0; i < len; i++) {
    crc ^= data[i];
    for (j = 0; j < 8; j++)
      crc = (crc & 1) ? (0xEDB88320u ^ (crc >> 1)) : (crc >> 1);
  }
  return crc ^ 0xFFFFFFFFu;
}

/* ---- 密钥链：CRC32 派生下一段密钥 ---- */
static u32 next_key(u32 key) {
  u8 kb[4] = {key, key >> 8, key >> 16, key >> 24};
  return crc32_calc(kb, 4);
}

/* ---- 段加密/解密 (XOR with rotating key) ---- */
static void seg_crypt(u8 *data, u32 len, u32 key) {
  u32 *words = (u32 *)data;
  u32 nwords = len / 4;
  for (u32 i = 0; i < nwords; i++) {
    /* 旋转密钥增加混淆度 */
    u32 rk = (key << (i % 32)) | (key >> (32 - (i % 32)));
    words[i] ^= rk;
  }
  /* 处理尾部不对齐字节 */
  u32 tail = len % 4;
  if (tail > 0) {
    u8 *p = data + nwords * 4;
    u8 kb[4] = {key, key >> 8, key >> 16, key >> 24};
    for (u32 i = 0; i < tail; i++)
      p[i] ^= kb[i];
  }
}

/* ---- 分段加密 (模拟 packer) ---- */
static void chain_encrypt(u8 *data, u32 total_len, u32 seg_size, u32 init_key) {
  u32 seg_count = (total_len + seg_size - 1) / seg_size;
  u32 key = init_key;

  printf("  [encrypt] total=%u seg_size=%u seg_count=%u init_key=0x%08X\n",
         total_len, seg_size, seg_count, init_key);

  for (u32 i = 0; i < seg_count; i++) {
    u32 off = i * seg_size;
    u32 len = (off + seg_size <= total_len) ? seg_size : (total_len - off);

    printf("  [encrypt] seg[%u] off=%u len=%u key=0x%08X\n", i, off, len, key);
    seg_crypt(data + off, len, key);
    key = next_key(key);
  }
}

/* ---- 按需解密 + 回加密 (模拟 stub 解释器) ---- */
typedef struct {
  u8 *data;
  u32 total_len;
  u32 seg_size;
  u32 init_key;
  int cur_seg;  /* 当前解密的 segment, -1 表示都是密文 */
  u32 keys[64]; /* 预计算的密钥链 (最多 64 段) */
  u32 seg_count;
} chain_ctx_t;

static void chain_init(chain_ctx_t *ctx, u8 *data, u32 total_len, u32 seg_size,
                       u32 init_key) {
  ctx->data = data;
  ctx->total_len = total_len;
  ctx->seg_size = seg_size;
  ctx->init_key = init_key;
  ctx->cur_seg = -1;
  ctx->seg_count = (total_len + seg_size - 1) / seg_size;

  /* 预计算密钥链 */
  ctx->keys[0] = init_key;
  for (u32 i = 1; i < ctx->seg_count; i++)
    ctx->keys[i] = next_key(ctx->keys[i - 1]);
}

/* 确保 segment[idx] 是明文, 同时回加密之前的 segment */
static void chain_ensure_decrypted(chain_ctx_t *ctx, u32 idx) {
  if (idx >= ctx->seg_count)
    return;
  if ((int)idx == ctx->cur_seg)
    return; /* 已经是明文 */

  /* 回加密当前明文 segment */
  if (ctx->cur_seg >= 0) {
    u32 off = ctx->cur_seg * ctx->seg_size;
    u32 len = (off + ctx->seg_size <= ctx->total_len) ? ctx->seg_size
                                                      : (ctx->total_len - off);
    seg_crypt(ctx->data + off, len, ctx->keys[ctx->cur_seg]);
    /* printf("  [re-encrypt] seg[%d]\n", ctx->cur_seg); */
  }

  /* 解密目标 segment */
  u32 off = idx * ctx->seg_size;
  u32 len = (off + ctx->seg_size <= ctx->total_len) ? ctx->seg_size
                                                    : (ctx->total_len - off);
  seg_crypt(ctx->data + off, len, ctx->keys[idx]);
  ctx->cur_seg = idx;
  /* printf("  [decrypt] seg[%u]\n", idx); */
}

/* 通过 PC 偏移获取所需 segment 并读字节 */
static u8 chain_read_byte(chain_ctx_t *ctx, u32 pc) {
  u32 seg_idx = pc / ctx->seg_size;
  chain_ensure_decrypted(ctx, seg_idx);
  return ctx->data[pc];
}

/* ============ 测试 ============ */

/* 生成测试字节码 (0x00..0xFF 循环) */
static void fill_test_data(u8 *data, u32 len) {
  for (u32 i = 0; i < len; i++)
    data[i] = (u8)(i & 0xFF);
}

/* 验证读出的数据与原始数据匹配 */
static int verify_chain_read(chain_ctx_t *ctx, const u8 *original, u32 len) {
  for (u32 i = 0; i < len; i++) {
    u8 got = chain_read_byte(ctx, i);
    if (got != original[i]) {
      printf("  MISMATCH at pc=%u: expected=0x%02X got=0x%02X\n", i,
             original[i], got);
      return 0;
    }
  }
  return 1;
}

/* 检查密文状态: 确保非当前段都不是明文 */
static int verify_only_one_cleartext(chain_ctx_t *ctx, const u8 *original) {
  int cleartext_count = 0;
  for (u32 i = 0; i < ctx->seg_count; i++) {
    u32 off = i * ctx->seg_size;
    u32 len = (off + ctx->seg_size <= ctx->total_len) ? ctx->seg_size
                                                      : (ctx->total_len - off);
    int is_clear = 1;
    for (u32 j = 0; j < len; j++) {
      if (ctx->data[off + j] != original[off + j]) {
        is_clear = 0;
        break;
      }
    }
    if (is_clear)
      cleartext_count++;
  }
  return cleartext_count; /* 应该 <= 1 */
}

int main(void) {
  printf("=== SectionCryptor Chain Encryption Demo ===\n\n");

  int failures = 0;
  u32 init_key = 0xBEEFCAFE;

  /* ---- Test 1: 基本分段加密/解密 ---- */
  {
    printf("[Test 1] Basic chain encrypt/decrypt (200B, seg=64)\n");
    u32 total = 200, seg = 64;
    u8 original[200], data[200];
    fill_test_data(original, total);
    memcpy(data, original, total);

    chain_encrypt(data, total, seg, init_key);

    /* 验证加密后数据已改变 */
    int changed = 0;
    for (u32 i = 0; i < total; i++)
      if (data[i] != original[i])
        changed++;
    printf("  encrypted: %d/%u bytes changed\n", changed, total);

    /* 用 chain_ctx 逐字节读取 */
    chain_ctx_t ctx;
    chain_init(&ctx, data, total, seg, init_key);
    int ok = verify_chain_read(&ctx, original, total);
    printf("  [Test 1] %s\n\n", ok ? "PASS" : "FAIL");
    if (!ok)
      failures++;
  }

  /* ---- Test 2: 只有一个段是明文 ---- */
  {
    printf("[Test 2] Only 1 segment cleartext at a time\n");
    u32 total = 256, seg = 64;
    u8 original[256], data[256];
    fill_test_data(original, total);
    memcpy(data, original, total);
    chain_encrypt(data, total, seg, init_key);

    chain_ctx_t ctx;
    chain_init(&ctx, data, total, seg, init_key);

    /* 解密 seg[2] */
    chain_read_byte(&ctx, seg * 2 + 10);
    int ct = verify_only_one_cleartext(&ctx, original);
    printf("  after reading seg[2]: cleartext_count=%d (expect 1)\n", ct);
    int ok = (ct == 1);

    /* 切换到 seg[0] */
    chain_read_byte(&ctx, 5);
    ct = verify_only_one_cleartext(&ctx, original);
    printf("  after reading seg[0]: cleartext_count=%d (expect 1)\n", ct);
    ok = ok && (ct == 1);

    printf("  [Test 2] %s\n\n", ok ? "PASS" : "FAIL");
    if (!ok)
      failures++;
  }

  /* ---- Test 3: 跨段顺序遍历 ---- */
  {
    printf("[Test 3] Sequential traversal across all segments\n");
    u32 total = 300, seg = 32;
    u8 original[300], data[300];
    fill_test_data(original, total);
    memcpy(data, original, total);
    chain_encrypt(data, total, seg, init_key);

    chain_ctx_t ctx;
    chain_init(&ctx, data, total, seg, init_key);
    int ok = verify_chain_read(&ctx, original, total);
    printf("  [Test 3] %s (300B, seg=32, %u segments)\n\n",
           ok ? "PASS" : "FAIL", ctx.seg_count);
    if (!ok)
      failures++;
  }

  /* ---- Test 4: 随机跳转访问 ---- */
  {
    printf("[Test 4] Random jump access pattern\n");
    u32 total = 512, seg = 64;
    u8 original[512], data[512];
    fill_test_data(original, total);
    memcpy(data, original, total);
    chain_encrypt(data, total, seg, init_key);

    chain_ctx_t ctx;
    chain_init(&ctx, data, total, seg, init_key);

    /* 模拟 VM 的跳转: 随机访问不同段 */
    u32 jumps[] = {400, 100, 300, 50, 450, 200, 10, 500, 250, 0};
    int ok = 1;
    for (int i = 0; i < 10; i++) {
      u32 pc = jumps[i];
      u8 got = chain_read_byte(&ctx, pc);
      if (got != original[pc]) {
        printf("  FAIL at jump pc=%u: expected=0x%02X got=0x%02X\n", pc,
               original[pc], got);
        ok = 0;
      }
    }
    printf("  [Test 4] %s (10 random jumps)\n\n", ok ? "PASS" : "FAIL");
    if (!ok)
      failures++;
  }

  /* ---- Test 5: 密钥链一致性 ---- */
  {
    printf("[Test 5] Key chain derivation\n");
    u32 k = init_key;
    printf("  key[0] = 0x%08X\n", k);
    for (int i = 1; i <= 4; i++) {
      k = next_key(k);
      printf("  key[%d] = 0x%08X\n", i, k);
    }
    /* 验证确定性 */
    u32 k2 = init_key;
    for (int i = 0; i < 4; i++)
      k2 = next_key(k2);
    int ok = (k == k2);
    printf("  [Test 5] %s (deterministic key chain)\n\n", ok ? "PASS" : "FAIL");
    if (!ok)
      failures++;
  }

  printf("=== Result: %s (%d failures) ===\n",
         failures == 0 ? "ALL PASS" : "SOME FAILED", failures);
  return failures;
}
