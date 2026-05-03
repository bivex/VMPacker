/*
 * vm_crc.h — CRC32 Integrity Check (no external dependencies)
 *
 * CRC-32/ISO polynomial 0xEDB88320, compatible with Go crc32.ChecksumIEEE.
 * Used for integrity checking of bytecode and stub code internally.
 *
 * Trailer CRC segment format (before BR mapping table, optional):
 *   [stub_va:u64]     stub code virtual address in memory
 *   [stub_size:u32]   stub code size
 *   [stub_crc:u32]    CRC32 of stub code
 *   [bc_crc:u32]      CRC32 of bytecode (excluding CRC segment and BR table)
 *   [CRC_MAGIC:u32]   0x43524332 ("CRC2")
 *   Total 24 bytes
 */
#ifndef VM_CRC_H
#define VM_CRC_H

#include "vm_decode.h"
#include "vm_types.h"

#define CRC_MAGIC 0x43524332u /* "CRC2" little-endian */
#define CRC_SECTION_SIZE 24   /* 8+4+4+4+4 bytes      */

/* ---- CRC32 Bitwise Implementation (no table) ---- */
/* Do not use lookup table: stub is an RX-only flat binary, cannot write to .bss/.data */

static inline u32 crc32_calc(const u8 *data, u32 len) {
  u32 crc = 0xFFFFFFFFu, i, j;
  for (i = 0; i < len; i++) {
    crc ^= data[i];
    for (j = 0; j < 8; j++)
      crc = (crc & 1) ? (0xEDB88320u ^ (crc >> 1)) : (crc >> 1);
  }
  return crc ^ 0xFFFFFFFFu;
}

#endif /* VM_CRC_H */
