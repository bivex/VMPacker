/*
 * h_snprintf.h — S_PRINTF handler for variadic snprintf calls
 *
 * OpSnprintf (0xAC): 9B opcode (1B op + 8B target addr).
 * Reads args from vm->R[0]-R[18] (19 args) and calls target.
 */
#ifndef H_SNPRINTF_H
#define H_SNPRINTF_H

#include "../vm_decode.h"
#include "../vm_types.h"

typedef int (*snprintf_func_t)(char *, unsigned long, const char *, ...);

static inline u32 h_snprintf(vm_ctx_t *vm) {
  u64 target = rd64(&vm->bc[vm->pc + 1]);
  snprintf_func_t fn = (snprintf_func_t)target;

  int result = fn((char*)vm->R[0], (unsigned long)vm->R[1], (const char*)vm->R[2],
                  vm->R[3], vm->R[4], vm->R[5], vm->R[6], vm->R[7],
                  vm->R[8], vm->R[9], vm->R[10], vm->R[11], vm->R[12],
                  vm->R[13], vm->R[14], vm->R[15], vm->R[16], vm->R[17],
                  vm->R[18]);

  vm->R[0] = (u64)result;
  return 9; /* 1-byte opcode + 8-byte target */
}

#endif /* H_SNPRINTF_H */
