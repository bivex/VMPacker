/*
 * vm_security.h — VM Runtime Security Protections (x86_64 stub)
 *
 * This stub provides no-op implementations for x86_64; actual security
 * features are architecture-specific and currently only implemented for AArch64.
 */

#ifndef VM_SECURITY_H
#define VM_SECURITY_H

#include "vm_types.h"

/* ---- Anti-Tampering / Anti-Debug stubs (no-op on x86_64) ---- */

static inline void sec_protect_memory(void *addr, unsigned long len) { (void)addr; (void)len; }
static inline void sec_zero_memory(void *addr, unsigned long len) { (void)addr; (void)len; }
static inline int  sec_check_ptrace(void) { return 0; }
static inline int  sec_check_tracerpid(void) { return 0; }
static inline void sec_self_hash_check(void) {}

#endif /* VM_SECURITY_H */