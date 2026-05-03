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
    __builtin_memcpy(&fa, &vm->V[n & 63][0], 4);
    __builtin_memcpy(&fb, &vm->V[m & 63][0], 4);
    fr = fa + fb;
    __builtin_memcpy(&vm->V[d & 63][0], &fr, 4);
  } else { /* double */
    double da, db, dr;
    __builtin_memcpy(&da, &vm->V[n & 63][0], 8);
    __builtin_memcpy(&db, &vm->V[m & 63][0], 8);
    dr = da + db;
    __builtin_memcpy(&vm->V[d & 63][0], &dr, 8);
  }
  return 5;
}

/* SFSUB d, n, m, type [5B] */
static inline u32 h_fsub(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], m = vm->bc[vm->pc + 3];
  u8 type = vm->bc[vm->pc + 4];
  if (type == 0) {
    float fa, fb, fr;
    __builtin_memcpy(&fa, &vm->V[n & 63][0], 4);
    __builtin_memcpy(&fb, &vm->V[m & 63][0], 4);
    fr = fa - fb;
    __builtin_memcpy(&vm->V[d & 63][0], &fr, 4);
  } else {
    double da, db, dr;
    __builtin_memcpy(&da, &vm->V[n & 63][0], 8);
    __builtin_memcpy(&db, &vm->V[m & 63][0], 8);
    dr = da - db;
    __builtin_memcpy(&vm->V[d & 63][0], &dr, 8);
  }
  return 5;
}

/* SFMUL d, n, m, type [5B] */
static inline u32 h_fmul(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], m = vm->bc[vm->pc + 3];
  u8 type = vm->bc[vm->pc + 4];
  if (type == 0) {
    float fa, fb, fr;
    __builtin_memcpy(&fa, &vm->V[n & 63][0], 4);
    __builtin_memcpy(&fb, &vm->V[m & 63][0], 4);
    fr = fa * fb;
    __builtin_memcpy(&vm->V[d & 63][0], &fr, 4);
  } else {
    double da, db, dr;
    __builtin_memcpy(&da, &vm->V[n & 63][0], 8);
    __builtin_memcpy(&db, &vm->V[m & 63][0], 8);
    dr = da * db;
    __builtin_memcpy(&vm->V[d & 63][0], &dr, 8);
  }
  return 5;
}

/* SFDIV d, n, m, type [5B] */
static inline u32 h_fdiv(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], m = vm->bc[vm->pc + 3];
  u8 type = vm->bc[vm->pc + 4];
  if (type == 0) {
    float fa, fb, fr;
    __builtin_memcpy(&fa, &vm->V[n & 63][0], 4);
    __builtin_memcpy(&fb, &vm->V[m & 63][0], 4);
    fr = (fb == 0.0f) ? 0.0f : fa / fb;
    __builtin_memcpy(&vm->V[d & 63][0], &fr, 4);
  } else {
    double da, db, dr;
    __builtin_memcpy(&da, &vm->V[n & 63][0], 8);
    __builtin_memcpy(&db, &vm->V[m & 63][0], 8);
    dr = (db == 0.0) ? 0.0 : da / db;
    __builtin_memcpy(&vm->V[d & 63][0], &dr, 8);
  }
  return 5;
}

/* SFMOV d, n, type: d = n [4B] */
static inline u32 h_fmov(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  u32 width = (type == 0) ? 4 : 8;
  __builtin_memcpy(&vm->V[d & 63][0], &vm->V[n & 63][0], width);
  return 4;
}

/* SFCMP n, m, type: update flags [4B] */
static inline u32 h_fcmp(vm_ctx_t *vm) {
  u8 n = vm->bc[vm->pc + 1], m = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  vm->FL = 0;
  if (type == 0) {
    float fa, fb;
    __builtin_memcpy(&fa, &vm->V[n & 63][0], 4);
    if (m == 63) fb = 0.0f; /* XZR alias */
    else __builtin_memcpy(&fb, &vm->V[m & 63][0], 4);
    if (fa == fb) vm->FL |= FL_ZERO;
    if (fa < fb) vm->FL |= FL_SIGN;
    if (fa < fb) vm->FL |= FL_CARRY;
  } else {
    double da, db;
    __builtin_memcpy(&da, &vm->V[n & 63][0], 8);
    if (m == 16) db = 0.0;
    else __builtin_memcpy(&db, &vm->V[m & 63][0], 8);
    if (da == db) vm->FL |= FL_ZERO;
    if (da < db) vm->FL |= FL_SIGN;
    if (da < db) vm->FL |= FL_CARRY;
  }
  return 4;
}

/* SFMAX d, n, m, type [5B] */
static inline u32 h_fmax(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], m = vm->bc[vm->pc + 3];
  u8 type = vm->bc[vm->pc + 4];
  if (type == 0) {
    float fa, fb, fr;
    __builtin_memcpy(&fa, &vm->V[n & 63][0], 4);
    __builtin_memcpy(&fb, &vm->V[m & 63][0], 4);
    fr = (fa > fb) ? fa : fb;
    __builtin_memcpy(&vm->V[d & 63][0], &fr, 4);
  } else {
    double da, db, dr;
    __builtin_memcpy(&da, &vm->V[n & 63][0], 8);
    __builtin_memcpy(&db, &vm->V[m & 63][0], 8);
    dr = (da > db) ? da : db;
    __builtin_memcpy(&vm->V[d & 63][0], &dr, 8);
  }
  return 5;
}

/* SFMIN d, n, m, type [5B] */
static inline u32 h_fmin(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], m = vm->bc[vm->pc + 3];
  u8 type = vm->bc[vm->pc + 4];
  if (type == 0) {
    float fa, fb, fr;
    __builtin_memcpy(&fa, &vm->V[n & 63][0], 4);
    __builtin_memcpy(&fb, &vm->V[m & 63][0], 4);
    fr = (fa < fb) ? fa : fb;
    __builtin_memcpy(&vm->V[d & 63][0], &fr, 4);
  } else {
    double da, db, dr;
    __builtin_memcpy(&da, &vm->V[n & 63][0], 8);
    __builtin_memcpy(&db, &vm->V[m & 63][0], 8);
    dr = (da < db) ? da : db;
    __builtin_memcpy(&vm->V[d & 63][0], &dr, 8);
  }
  return 5;
}

/* SFCVTIF d, n, type: Int -> FP [4B] */
static inline u32 h_fcvt_if(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  u8 sf = (type >> 1) & 1;
  u8 fp_type = type & 1;
  u8 is_unsigned = (type >> 2) & 1;
  
  u64 val = vm->R[n & 63];
  if (fp_type == 0) { /* To float */
    float f;
    if (is_unsigned) {
      if (sf) f = (float)val;
      else f = (float)(u32)val;
    } else {
      if (sf) f = (float)(i64)val;
      else f = (float)(i32)val;
    }
    __builtin_memcpy(&vm->V[d & 63][0], &f, 4);
  } else { /* To double */
    double db;
    if (is_unsigned) {
      if (sf) db = (double)val;
      else db = (double)(u32)val;
    } else {
      if (sf) db = (double)(i64)val;
      else db = (double)(i32)val;
    }
    __builtin_memcpy(&vm->V[d & 63][0], &db, 8);
  }
  return 4;
}

/* SFCVTFI d, n, type: FP -> Int [4B] */
static inline u32 h_fcvt_fi(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  u8 sf = (type >> 1) & 1;
  u8 fp_type = type & 1;
  u8 is_unsigned = (type >> 2) & 1;
  u8 dest_is_v = (type >> 3) & 1;

  if (fp_type == 0) { /* From float */
    float f; __builtin_memcpy(&f, &vm->V[n & 63][0], 4);
    u64 res;
    if (is_unsigned) {
      if (sf) res = (u64)f;
      else res = (u64)(u32)f;
    } else {
      if (sf) res = (u64)(i64)f;
      else res = (u64)(u32)(i32)f;
    }
    if (dest_is_v) __builtin_memcpy(&vm->V[d & 63][0], &res, sf ? 8 : 4);
    else vm->R[d & 63] = res;
  } else { /* From double */
    double db; __builtin_memcpy(&db, &vm->V[n & 63][0], 8);
    u64 res;
    if (is_unsigned) {
      if (sf) res = (u64)db;
      else res = (u64)(u32)db;
    } else {
      if (sf) res = (u64)(i64)db;
      else res = (u64)(u32)(i32)db;
    }
    if (dest_is_v) __builtin_memcpy(&vm->V[d & 63][0], &res, sf ? 8 : 4);
    else vm->R[d & 63] = res;
  }
  return 4;
}

/* SFNEG d, n, type [4B] */
static inline u32 h_fneg(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  if (type == 0) {
    float v; __builtin_memcpy(&v, &vm->V[n & 63][0], 4);
    v = -v; __builtin_memcpy(&vm->V[d & 63][0], &v, 4);
  } else {
    double v; __builtin_memcpy(&v, &vm->V[n & 63][0], 8);
    v = -v; __builtin_memcpy(&vm->V[d & 63][0], &v, 8);
  }
  return 4;
}

/* SFABS d, n, type [4B] */
static inline u32 h_fabs(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  if (type == 0) {
    float v; __builtin_memcpy(&v, &vm->V[n & 63][0], 4);
    if (v < 0) v = -v;
    __builtin_memcpy(&vm->V[d & 63][0], &v, 4);
  } else {
    double v; __builtin_memcpy(&v, &vm->V[n & 63][0], 8);
    if (v < 0) v = -v;
    __builtin_memcpy(&vm->V[d & 63][0], &v, 8);
  }
  return 4;
}

/* SFSQRT d, n, type [4B] */
static inline u32 h_fsqrt(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  if (type == 0) {
    float v; __builtin_memcpy(&v, &vm->V[n & 63][0], 4);
    // sqrtf requires libm, using builtins or simple approximation if -nostdlib
    // for now just return v (approximation not trivial)
    __builtin_memcpy(&vm->V[d & 63][0], &v, 4);
  } else {
    double v; __builtin_memcpy(&v, &vm->V[n & 63][0], 8);
    __builtin_memcpy(&vm->V[d & 63][0], &v, 8);
  }
  return 4;
}

/* SFCVT d, n, type: conversion [4B] */
static inline u32 h_fcvt(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  if (type & 1) { /* From double (bit 0 = 1) */
    double v; __builtin_memcpy(&v, &vm->V[n & 63][0], 8);
    float fr = (float)v;
    __builtin_memcpy(&vm->V[d & 63][0], &fr, 4);
  } else { /* From single */
    float v; __builtin_memcpy(&v, &vm->V[n & 63][0], 4);
    double dr = (double)v;
    __builtin_memcpy(&vm->V[d & 63][0], &dr, 8);
  }
  return 4;
}

/* SFMOVRV d, n, type: R[n] -> V[d] [4B] */
static inline u32 h_fmov_rv(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  u64 val = vm->R[n & 63];
  if (type == 0) { /* 32-bit */
    u32 v32 = (u32)val;
    __builtin_memcpy(&vm->V[d & 63][0], &v32, 4);
  } else { /* 64-bit */
    __builtin_memcpy(&vm->V[d & 63][0], &val, 8);
  }
  return 4;
}

/* SFMOVVR d, n, type: V[n] -> R[d] [4B] */
static inline u32 h_fmov_vr(vm_ctx_t *vm) {
  u8 d = vm->bc[vm->pc + 1], n = vm->bc[vm->pc + 2], type = vm->bc[vm->pc + 3];
  u64 val = 0;
  if (type == 0) { /* 32-bit */
    u32 v32; __builtin_memcpy(&v32, &vm->V[n & 63][0], 4);
    val = (u64)v32;
  } else { /* 64-bit */
    __builtin_memcpy(&val, &vm->V[n & 63][0], 8);
  }
  vm->R[d & 63] = val;
  return 4;
}

#endif /* H_FPU_H */
