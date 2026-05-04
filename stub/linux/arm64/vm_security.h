/*
 * vm_security.h — VM Runtime Security Protections
 *
 * Implements Anti-Tampering, Anti-Debug, and Anti-Hook mechanisms.
 * Designed to be zero-dependency (no libc).
 */
#ifndef VM_SECURITY_H
#define VM_SECURITY_H

#include "vm_types.h"

/* Syscall definitions for ARM64 */
#define __NR_madvise 233
#define __NR_ptrace  117
#define __NR_openat  56
#define __NR_read    63
#define __NR_close   57

#define MADV_DONTDUMP 16
#define PTRACE_TRACEME 0
#define AT_FDCWD -100
#define O_RDONLY 0

/* ---- Syscall Wrappers ---- */

static inline int sys_openat(int dirfd, const char *pathname, int flags) {
  register long x8 __asm__("x8") = __NR_openat;
  register long x0 __asm__("x0") = (long)dirfd;
  register long x1 __asm__("x1") = (long)pathname;
  register long x2 __asm__("x2") = (long)flags;
  register long x3 __asm__("x3") = 0;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2), "r"(x3) : "memory");
  return (int)x0;
}

static inline long sys_read(int fd, void *buf, unsigned long count) {
  register long x8 __asm__("x8") = __NR_read;
  register long x0 __asm__("x0") = (long)fd;
  register long x1 __asm__("x1") = (long)buf;
  register long x2 __asm__("x2") = (long)count;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2) : "memory");
  return x0;
}

static inline int sys_close(int fd) {
  register long x8 __asm__("x8") = __NR_close;
  register long x0 __asm__("x0") = (long)fd;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8) : "memory");
  return (int)x0;
}

static inline int sys_madvise(void *addr, unsigned long len, int advice) {
  register long x8 __asm__("x8") = __NR_madvise;
  register long x0 __asm__("x0") = (long)addr;
  register long x1 __asm__("x1") = (long)len;
  register long x2 __asm__("x2") = (long)advice;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2) : "memory");
  return (int)x0;
}

static inline long sys_ptrace(int request, long pid, void *addr, void *data) {
  register long x8 __asm__("x8") = __NR_ptrace;
  register long x0 __asm__("x0") = (long)request;
  register long x1 __asm__("x1") = (long)pid;
  register long x2 __asm__("x2") = (long)addr;
  register long x3 __asm__("x3") = (long)data;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2), "r"(x3) : "memory");
  return x0;
}

/* ---- Anti-Tampering: Memory Dump Protection ---- */

static inline void sec_protect_memory(void *addr, unsigned long len) {
  /* Prevent the region from appearing in core dumps */
  sys_madvise(addr, len, MADV_DONTDUMP);
}

/* ---- Anti-Tampering: Buffer Zeroing ---- */

static inline void sec_zero_memory(void *addr, unsigned long len) {
  /* Explicitly zero out memory to prevent lingering sensitive data */
  volatile char *ptr = (volatile char *)addr;
  while (len--) {
    *ptr++ = 0;
  }
}

/* ---- Anti-Debug: Ptrace Check ---- */

static inline int sec_check_ptrace(void) {
  /* Try to trace ourselves. If it fails, a debugger might already be attached. */
  long ret = sys_ptrace(PTRACE_TRACEME, 0, 0, 0);
  if (ret < 0) {
    return 1; /* Debugger detected */
  }
  return 0;
}

/* ---- Anti-Debug: Procfs TracerPid Check ---- */

static inline int sec_check_tracerpid(void) {
  char proc_path[18];
  proc_path[0] = '/'; proc_path[1] = 'p'; proc_path[2] = 'r'; proc_path[3] = 'o'; proc_path[4] = 'c';
  proc_path[5] = '/'; proc_path[6] = 's'; proc_path[7] = 'e'; proc_path[8] = 'l'; proc_path[9] = 'f';
  proc_path[10] = '/'; proc_path[11] = 's'; proc_path[12] = 't'; proc_path[13] = 'a'; proc_path[14] = 't';
  proc_path[15] = 'u'; proc_path[16] = 's'; proc_path[17] = '\0';

  int fd = sys_openat(AT_FDCWD, proc_path, O_RDONLY);
  if (fd < 0) return 0;

  char buf[512];
  long bytes = sys_read(fd, buf, sizeof(buf) - 1);
  sys_close(fd);
  
  if (bytes <= 0) return 0;
  buf[bytes] = '\0';

  /* Avoid string literals which might be placed in .rodata and stripped.
   * "TracerPid:" = 54 72 61 63 65 72 50 69 64 3a */
  char target[10];
  target[0] = 'T'; target[1] = 'r'; target[2] = 'a'; target[3] = 'c';
  target[4] = 'e'; target[5] = 'r'; target[6] = 'P'; target[7] = 'i';
  target[8] = 'd'; target[9] = ':';

  for (int i = 0; i < bytes - 10; i++) {
    int match = 1;
    for (int j = 0; j < 10; j++) {
      if (buf[i + j] != target[j]) {
        match = 0;
        break;
      }
    }
    if (match) {
      int k = i + 10;
      /* Skip whitespace */
      while (k < bytes && (buf[k] == ' ' || buf[k] == '\t')) {
        k++;
      }
      if (k < bytes && buf[k] != '0') {
        return 1; /* TracerPid is not 0 -> Debugger attached! */
      }
      break;
    }
  }
  return 0;
}

/* ---- Anti-Debug: Timing Check ---- */

/* Returns the current value of CNTVCT_EL0 (virtual timer count) */
static inline unsigned long long sec_get_timer(void) {
  unsigned long long ticks;
  __asm__ volatile("mrs %0, cntvct_el0" : "=r"(ticks));
  return ticks;
}

/* ---- Anti-Debug: Breakpoint Scanning ---- */

static inline int sec_scan_breakpoints(void *addr, unsigned long len) {
  /* Scan for AArch64 BRK instructions (0xD4200000 mask 0xFFE0001F) */
  unsigned int *ptr = (unsigned int *)addr;
  unsigned long count = len / 4;
  for (unsigned long i = 0; i < count; i++) {
    if ((ptr[i] & 0xFFE0001F) == 0xD4200000) {
      return 1; /* Breakpoint detected */
    }
  }
  return 0;
}

/* ---- Anti-Hook: Inline Hook Detection ---- */

static inline int sec_scan_inline_hook(void *func_addr) {
  /* Check first instruction for an unconditional branch (B) or LDR PC */
  if (!func_addr) return 0;
  unsigned int insn = *(volatile unsigned int *)func_addr;
  
  /* B or BL instruction: 000101xx or 100101xx */
  if ((insn & 0xFC000000) == 0x14000000 || /* B imm26 */
      (insn & 0xFC000000) == 0x94000000) { /* BL imm26 */
    return 1; /* Possible inline hook */
  }
  
  /* LDR Xt, [PC, #imm] */
  if ((insn & 0xFF000000) == 0x58000000) {
    return 1; /* Possible inline hook (often used for far jumps) */
  }
  
  return 0;
}

#include "vm_crc.h"

/* ---- Runtime Periodic Checks ---- */

#define VM_CHECK_INTERVAL 1024 /* Check every 1024 instructions */

__attribute__((always_inline)) static inline void sec_panic(int code) {
  /* Instead of a clean exit, we can make it more annoying for the researcher. */
  /* 1. Corrupt some registers or stack? 
   * 2. Trigger an illegal instruction. */
  (void)code;
  __asm__ volatile(".inst 0x00000000"); /* Trigger UDF (Undefined Instruction) */
  while(1) { __asm__ volatile("yield"); }
}

static inline int sec_runtime_check(vm_ctx_t *vm) {
  vm->insn_count++;
  if (__builtin_expect((vm->insn_count & (VM_CHECK_INTERVAL - 1)) == 0, 0)) {
    /* 1. Bytecode Integrity Check */
    if (vm->expected_bc_crc != 0) {
      if (crc32_calc(vm->bc, vm->bc_crc_len) != vm->expected_bc_crc) {
        return 110; /* Tampering detected during runtime */
      }
    }

    /* 2. Anti-Debug: Basic Ptrace/TracerPid */
    if (sec_check_ptrace()) sec_panic(111);
    if (sec_check_tracerpid()) sec_panic(112);
  }
  return 0;
}

#endif /* VM_SECURITY_H */
