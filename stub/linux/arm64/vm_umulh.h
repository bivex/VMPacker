/*
 * vm_umulh.h — 64x64 -> 高64位乘法 (UMULH/SMULH) + 64位除法 (UDIV/SDIV)
 * ARM32 无 __int128，且 -nostdlib 无 libgcc，提供软件实现
 */
#ifndef VM_UMULH_H
#define VM_UMULH_H

#include "vm_types.h"

/* 64-bit software div/rem — 避免 __aeabi_uldivmod/__aeabi_ldivmod (libgcc)
 * 32-bit ARM 必须用软件实现 (-nostdlib 无 libgcc) */
#if (defined(__arm__) || defined(__thumb__)) || !defined(__SIZEOF_INT128__) || __SIZEOF_INT128__ != 16
static u64 udiv64(u64 n, u64 d) {
  if (d == 0) return 0;
  u64 q = 0, r = 0;
  for (int i = 63; i >= 0; i--) {
    r = (r << 1) | ((n >> i) & 1);
    if (r >= d) { r -= d; q |= (u64)1 << i; }
  }
  return q;
}
static u64 urem64(u64 n, u64 d) {
  if (d == 0) return 0;
  u64 r = 0;
  for (int i = 63; i >= 0; i--) {
    r = (r << 1) | ((n >> i) & 1);
    if (r >= d) r -= d;
  }
  return r;
}
static u64 sdiv64(i64 a, i64 b) {
  if (b == 0) return 0;
  int neg = (a < 0) != (b < 0);
  u64 ua = (u64)(a < 0 ? -a : a);
  u64 ub = (u64)(b < 0 ? -b : b);
  u64 q = udiv64(ua, ub);
  return neg ? (u64)(-(i64)q) : q;
}
#define VM_USE_SOFT_DIV 1
#endif

#if defined(__SIZEOF_INT128__) && __SIZEOF_INT128__ == 16
/* 64-bit targets with __int128 */
static u64 umulh64(u64 a, u64 b) {
  __uint128_t r = (__uint128_t)a * (__uint128_t)b;
  return (u64)(r >> 64);
}
static u64 smulh64(u64 a, u64 b) {
  __int128 r = (__int128)(i64)a * (__int128)(i64)b;
  return (u64)((unsigned __int128)r >> 64);
}
#else
/* 32-bit fallback: 64x64 -> 128 bit product, return high 64 */
static u64 umulh64(u64 a, u64 b) {
  u32 a_lo = (u32)a, a_hi = (u32)(a >> 32);
  u32 b_lo = (u32)b, b_hi = (u32)(b >> 32);
  u64 p0 = (u64)a_lo * b_lo;
  u64 p1 = (u64)a_lo * b_hi;
  u64 p2 = (u64)a_hi * b_lo;
  u64 p3 = (u64)a_hi * b_hi;
  u64 t = (p1 & 0xFFFFFFFFULL) + (p2 & 0xFFFFFFFFULL) + (p0 >> 32);
  return p3 + (p1 >> 32) + (p2 >> 32) + (t >> 32);
}
static u64 smulh64(u64 a, u64 b) {
  i64 sa = (i64)a, sb = (i64)b;
  int neg = (sa < 0) != (sb < 0);
  u64 ua = sa < 0 ? (u64)-sa : (u64)sa;
  u64 ub = sb < 0 ? (u64)-sb : (u64)sb;
  u64 hi = umulh64(ua, ub);
  return neg ? ~hi + (ua * ub == 0 ? 0 : 1) : hi;
}
#endif

#if defined(VM_USE_SOFT_DIV)
#define UDIV64(a, b) udiv64((a), (b))
#define SDIV64(a, b) sdiv64((i64)(a), (i64)(b))
#else
#define UDIV64(a, b) ((a) / (b))
#define SDIV64(a, b) ((u64)((i64)(a) / (i64)(b)))
#endif

#endif /* VM_UMULH_H */
