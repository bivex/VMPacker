/*
 * h_fpu.h — Floating-point / SIMD instruction handlers
 */
#ifndef H_FPU_H
#define H_FPU_H

#include "../vm_types.h"

/* SFADD d, n, m, type: d = n + m [5B] */
static inline u32 h_fadd(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], m = vm->bc[vm->pc + 3];
  u8 type = vm->bc[vm->pc + 4];
  if (type == 0) { /* float */
    float fa, fb, fr;
    __builtin_memcpy(&fa, &vm->V[n & 31][0], 4);
    __builtin_memcpy(&fb, &vm->V[m & 31][0], 4);
    fr = fa + fb;
    __builtin_memcpy(&vm->V[d & 31][0], &fr, 4);
  } else { /* double */
    double da, db, dr;
    __builtin_memcpy(&da, &vm->V[n & 31][0], 8);
    __builtin_memcpy(&db, &vm->V[m & 31][0], 8);
    dr = da + db;
    __builtin_memcpy(&vm->V[d & 31][0], &dr, 8);
  }
  return 5;
}

/* SFSUB d, n, m, type [5B] */
static inline u32 h_fsub(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], m = vm->bc[vm->pc + 3];
  u8 type = vm->bc[vm->pc + 4];
  if (type == 0) {
    float fa, fb, fr;
    __builtin_memcpy(&fa, &vm->V[n & 31][0], 4);
    __builtin_memcpy(&fb, &vm->V[m & 31][0], 4);
    fr = fa - fb;
    __builtin_memcpy(&vm->V[d & 31][0], &fr, 4);
  } else {
    double da, db, dr;
    __builtin_memcpy(&da, &vm->V[n & 31][0], 8);
    __builtin_memcpy(&db, &vm->V[m & 31][0], 8);
    dr = da - db;
    __builtin_memcpy(&vm->V[d & 31][0], &dr, 8);
  }
  return 5;
}

/* SFMUL d, n, m, type [5B] */
static inline u32 h_fmul(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], m = vm->bc[vm->pc + 3];
  u8 type = vm->bc[vm->pc + 4];
  if (type == 0) {
    float fa, fb, fr;
    __builtin_memcpy(&fa, &vm->V[n & 31][0], 4);
    __builtin_memcpy(&fb, &vm->V[m & 31][0], 4);
    fr = fa * fb;
    __builtin_memcpy(&vm->V[d & 31][0], &fr, 4);
  } else {
    double da, db, dr;
    __builtin_memcpy(&da, &vm->V[n & 31][0], 8);
    __builtin_memcpy(&db, &vm->V[m & 31][0], 8);
    dr = da * db;
    __builtin_memcpy(&vm->V[d & 31][0], &dr, 8);
  }
  return 5;
}

/* SFDIV d, n, m, type [5B] */
static inline u32 h_fdiv(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], m = vm->bc[vm->pc + 3];
  u8 type = vm->bc[vm->pc + 4];
  if (type == 0) {
    float fa, fb, fr;
    __builtin_memcpy(&fa, &vm->V[n & 31][0], 4);
    __builtin_memcpy(&fb, &vm->V[m & 31][0], 4);
    fr = (fb == 0.0f) ? 0.0f : fa / fb;
    __builtin_memcpy(&vm->V[d & 31][0], &fr, 4);
  } else {
    double da, db, dr;
    __builtin_memcpy(&da, &vm->V[n & 31][0], 8);
    __builtin_memcpy(&db, &vm->V[m & 31][0], 8);
    dr = (db == 0.0) ? 0.0 : da / db;
    __builtin_memcpy(&vm->V[d & 31][0], &dr, 8);
  }
  return 5;
}

/* SFMOV d, n, type: d = n [4B] */
static inline u32 h_fmov(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  u32 width = (type == 0) ? 4 : 8;
  __builtin_memcpy(&vm->V[d & 31][0], &vm->V[n & 31][0], width);
  return 4;
}

/* SFCMP n, m, type: update flags [4B] */
static inline u32 h_fcmp(vm_ctx_t *vm) {
  u8 n = vm->bc[vm->pc + 1], m = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  vm->FL = 0;
  if (type == 0) {
    float fa, fb;
    __builtin_memcpy(&fa, &vm->V[n & 31][0], 4);
    __builtin_memcpy(&fb, &vm->V[m & 31][0], 4);
    if (fa == fb) vm->FL |= FL_ZERO;
    if (fa < fb) vm->FL |= FL_SIGN;
    if (fa < fb) vm->FL |= FL_CARRY;
  } else {
    double da, db;
    __builtin_memcpy(&da, &vm->V[n & 31][0], 8);
    __builtin_memcpy(&db, &vm->V[m & 31][0], 8);
    if (da == db) vm->FL |= FL_ZERO;
    if (da < db) vm->FL |= FL_SIGN;
    if (da < db) vm->FL |= FL_CARRY;
  }
  return 4;
}

#endif /* H_FPU_H */
