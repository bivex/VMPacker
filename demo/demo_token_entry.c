/*
 * demo_token_entry.c — Token 32-bit encoding/decoding roundtrip verification
 *
 * Layout:
 *   bits[31:24] = XOR key    (8  bits, 0-255)
 *   bits[23:12] = bc_offset  (12 bits, 0-4095)
 *   bits[11:0]  = func_id    (12 bits, 0-4095)
 */

#include <stdint.h>
#include <stdio.h>

/* ---- Decoding Macros ---- */
#define TOKEN_FUNC_ID(tok)    ((tok) & 0xFFF)
#define TOKEN_BC_OFFSET(tok)  (((tok) >> 12) & 0xFFF)
#define TOKEN_XOR_KEY(tok)    (((tok) >> 24) & 0xFF)

/* ---- Encoding Macros ---- */
#define TOKEN_ENCODE(func_id, bc_offset, xor_key) \
    (((uint32_t)(xor_key) << 24) | ((uint32_t)(bc_offset) << 12) | ((uint32_t)(func_id) & 0xFFF))

/* Single test case */
static int test_roundtrip(uint32_t func_id, uint32_t bc_offset, uint32_t xor_key)
{
    uint32_t tok = TOKEN_ENCODE(func_id, bc_offset, xor_key);
    uint32_t d_fid = TOKEN_FUNC_ID(tok);
    uint32_t d_off = TOKEN_BC_OFFSET(tok);
    uint32_t d_key = TOKEN_XOR_KEY(tok);

    int ok = (d_fid == func_id) && (d_off == bc_offset) && (d_key == xor_key);

    printf("  func_id=%-4u bc_offset=%-4u xor_key=0x%02X  =>  tok=0x%08X  =>  "
           "fid=%-4u off=%-4u key=0x%02X  [%s]\n",
           func_id, bc_offset, xor_key, tok,
           d_fid, d_off, d_key,
           ok ? "PASS" : "FAIL");
    return ok;
}

int main(void)
{
    int fail = 0;

    printf("=== Token 32-bit roundtrip verification ===\n\n");

    /* Boundary values */
    printf("[Boundary values]\n");
    fail += !test_roundtrip(0,    0,    0);
    fail += !test_roundtrip(4095, 4095, 255);
    fail += !test_roundtrip(4095, 0,    0);
    fail += !test_roundtrip(0,    4095, 0);
    fail += !test_roundtrip(0,    0,    255);

    /* Typical values */
    printf("\n[Typical values]\n");
    fail += !test_roundtrip(42,   100,  0xA5);
    fail += !test_roundtrip(1,    1,    1);
    fail += !test_roundtrip(256,  512,  128);

    /* Random values */
    printf("\n[Random values]\n");
    fail += !test_roundtrip(3333, 2048, 0xDE);
    fail += !test_roundtrip(1023, 3071, 0x7F);
    fail += !test_roundtrip(2222, 1111, 0x55);
    fail += !test_roundtrip(4000, 300,  0xAB);
    fail += !test_roundtrip(777,  888,  0x01);

    printf("\n=== Result: %s (%d items failed) ===\n",
           fail ? "FAIL" : "ALL PASS", fail);

    return fail ? 1 : 0;
}
