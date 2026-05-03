/*
 * vm_crc.h — CRC32 完整性校验 (无外部依赖)
 *
 * CRC-32/ISO polynomial 0xEDB88320, compatible with Go crc32.ChecksumIEEE.
 * Used for integrity checking of bytecode and stub code internally.
 *
 * 尾部 CRC 段格式 (在 BR 映射表之前，可选):
 *   [stub_va:u64]     stub code virtual address in memory
 *   [stub_size:u32]   stub 代码大小
 *   [stub_crc:u32]    stub 代码的 CRC32
 *   [bc_crc:u32]      字节码 (不含 CRC 段和 BR 表) 的 CRC32
 *   [CRC_MAGIC:u32]   0x43524332 ("CRC2")
 *   共 24 字节
 */
#ifndef VM_CRC_H
#define VM_CRC_H

#include "vm_decode.h"
#include "vm_types.h"

#define CRC_MAGIC 0x43524332u /* "CRC2" little-endian */
#define CRC_SECTION_SIZE 24   /* 8+4+4+4+4 bytes      */

/* ---- CRC32 无表位运算实现 ---- */
/* 不使用查表: stub 是 RX-only flat binary, 不能写 .bss/.data */

static inline u32 crc32_calc(const u8 *data, u32 len) {
  u32 crc = 0xFFFFFFFFu, i, j;
  for (i = 0; i < len; i++) {
    crc ^= data[i];
    for (j = 0; j < 8; j++)
      crc = (crc & 1) ? (0xEDB88320u ^ (crc >> 1)) : (crc >> 1);
  }
  return crc ^ 0xFFFFFFFFu;
}
