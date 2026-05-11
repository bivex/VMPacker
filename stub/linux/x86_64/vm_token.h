/*
 * vm_token.h — Token 编解码宏 + 描述符表类型
 *
 * Token 32位布局:
 *   bits[31:24] = XOR 密钥   (8  bits, 0-255)
 *   bits[23:12] = bc_offset  (12 bits, 0-4095, 保留)
 *   bits[11:0]  = func_id    (12 bits, 0-4095)
 *
 * 使用 u32 而非 uint32_t (nostdlib 环境)
 */
#ifndef VM_TOKEN_H
#define VM_TOKEN_H

#include "vm_types.h"

/* ---- 解码宏 ---- */
#define TOKEN_FUNC_ID(tok)    ((u32)(tok) & 0xFFF)
#define TOKEN_BC_OFFSET(tok)  (((u32)(tok) >> 12) & 0xFFF)
#define TOKEN_XOR_KEY(tok)    (((u32)(tok) >> 24) & 0xFF)

/* ---- 编码宏 ---- */
#define TOKEN_ENCODE(fid, off, key) \
    (((u32)(key) << 24) | ((u32)(off) << 12) | ((u32)(fid) & 0xFFF))

/* ---- Token descriptor table entries (written by packer in payload) ---- */
typedef struct {
    u64 bc_off;     /* 加密字节码相对于 _token_table_va 的偏移 (PIE 兼容) */
    u32 bc_len;     /* 字节码长度 */
    u32 reserved;   /* 对齐到 16B */
} token_desc_t;

/* ---- 最大被保护函数数 ---- */
#define TOKEN_MAX_FUNCS 4096

#endif /* VM_TOKEN_H */
