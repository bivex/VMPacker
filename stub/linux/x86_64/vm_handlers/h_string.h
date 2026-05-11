/*
 * h_string.h — String Decryption Handlers
 */
#ifndef H_STRING_H
#define H_STRING_H

#include "../vm_types.h"

/*
 * OP_S_DECRYPT_STR — pop key, len, addr -> decrypt string and push ptr
 * 
 * Logic:
 *   1. Pop XOR key (u64, used as u8)
 *   2. Pop string length (u64, used as u32)
 *   3. Pop encrypted string address (u64)
 *   4. Copy to circular string_pool and XOR decrypt
 *   5. Push string_pool + offset back to eval_stk
 */
static __attribute__((always_inline)) u32 h_s_decrypt_str(vm_ctx_t *vm) {
  VM_DEBUG("[VM] h_s_decrypt_str fired!\n");
  u64 key = vm->eval_stk[--vm->eval_sp];
  u32 len = (u32)vm->eval_stk[--vm->eval_sp];
  u64 addr = vm->eval_stk[--vm->eval_sp];

  /* Ensure enough space in circular buffer (max string len 1024) */
  if (len > 1024) len = 1024;
  
  /* Check if we need to wrap around */
  if (vm->str_ptr + len + 1 > 4096) {
    vm->str_ptr = 0;
  }

  u8 *dst = &vm->string_pool[vm->str_ptr];
  u8 *src = (u8 *)addr;
  u8 k = (u8)key;

  for (u32 i = 0; i < len; i++) {
    dst[i] = src[i] ^ k;
  }
  dst[len] = '\0'; /* Null terminate for safety */

  /* Push the result pointer onto the evaluation stack */
  vm->eval_stk[vm->eval_sp++] = (u64)dst;

  /* Advance the circular buffer pointer */
  vm->str_ptr += (len + 1);
  
  return 1; /* Opcode is 1 byte */
}

#endif /* H_STRING_H */
