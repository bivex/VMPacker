/*
 * vm_interp.c — 模块化 VM 解释器 (Linux/ARM64 PIC blob)
 *
 * 架构:
 *   vm_types.h       → 类型 + vm_ctx_t 结构体
 *   vm_opcodes.h     → 操作码定义
 *   vm_decode.h      → 字节码读取工具
 *   vm_handlers/*.h  → 分模块指令 handler
 *
 * 编译 (交叉编译为 blob):
 *   aarch64-linux-gnu-gcc -c -Os -mcmodel=tiny -fno-stack-protector \
 *     -fno-builtin -nostdlib -march=armv8-a vm_interp.c -o vm_interp.o
 */

/* ---- 基础设施 ---- */

#include "vm_decode.h"
#include "vm_opcodes.h"
#include "vm_types.h"

/* ---- 指令 Handler 模块 ---- */
#include "vm_handlers/h_alu.h" /* ADD/SUB/MUL/XOR/AND/OR/SHL/SHR/ASR/NOT/ROR + _IMM */
#include "vm_handlers/h_branch.h" /* JMP/JE/JNE/JL/JGE/JGT/JLE/JB/JAE */
#include "vm_handlers/h_cmp.h"    /* CMP, CMP_IMM */
#include "vm_handlers/h_mem.h"    /* LOAD/STORE 8/32/64 */
#include "vm_handlers/h_mov.h"    /* MOV_IMM, MOV_IMM32, MOV_REG */
#include "vm_handlers/h_stack.h"  /* PUSH, POP */
#include "vm_handlers/h_stack_ops.h" /* 栈机器操作 handler (VLOAD/VSTORE/VADD...) */
#include "vm_handlers/h_system.h" /* NOP, CALL_NAT, BR_REG, VLD16, VST16 */


/* ---- 间接 Dispatch 跳转表 (条件编译) ---- */
#ifdef VM_INDIRECT_DISPATCH
#include "vm_dispatch.h"
#endif

/* ---- Token 化入口 (条件编译) ---- */
/* TOKEN_ONLY: Token 入口始终编译 */
#include "vm_token.h"

/* ---- syscall: mmap (无 libc 依赖) ---- */
static inline void *sys_mmap(unsigned long size) {
  register long x8 __asm__("x8") = 222; /* __NR_mmap */
  register long x0 __asm__("x0") = 0;   /* addr = NULL */
  register long x1 __asm__("x1") = (long)size;
  register long x2 __asm__("x2") = 3;    /* PROT_READ | PROT_WRITE */
  register long x3 __asm__("x3") = 0x22; /* MAP_PRIVATE | MAP_ANONYMOUS */
  register long x4 __asm__("x4") = -1;   /* fd = -1 */
  register long x5 __asm__("x5") = 0;    /* offset = 0 */
  __asm__ volatile("svc #0"
                   : "+r"(x0)
                   : "r"(x8), "r"(x1), "r"(x2), "r"(x3), "r"(x4), "r"(x5)
                   : "memory");
  return (void *)x0;
}

/* ---- syscall: munmap ---- */
static inline void sys_munmap(void *addr, unsigned long size) {
  register long x8 __asm__("x8") = 215; /* __NR_munmap */
  register long x0 __asm__("x0") = (long)addr;
  register long x1 __asm__("x1") = (long)size;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1) : "memory");
}

/* ---- syscall: openat ---- */
static inline int sys_openat(int dirfd, const char *pathname, int flags) {
  register long x8 __asm__("x8") = 56; /* __NR_openat */
  register long x0 __asm__("x0") = (long)dirfd;
  register long x1 __asm__("x1") = (long)pathname;
  register long x2 __asm__("x2") = (long)flags;
  register long x3 __asm__("x3") = 0;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2), "r"(x3) : "memory");
  return (int)x0;
}

/* ---- syscall: read ---- */
static inline long sys_read(int fd, void *buf, unsigned long count) {
  register long x8 __asm__("x8") = 63; /* __NR_read */
  register long x0 __asm__("x0") = (long)fd;
  register long x1 __asm__("x1") = (long)buf;
  register long x2 __asm__("x2") = (long)count;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2) : "memory");
  return x0;
}

/* ---- syscall: close ---- */
static inline int sys_close(int fd) {
  register long x8 __asm__("x8") = 57; /* __NR_close */
  register long x0 __asm__("x0") = (long)fd;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8) : "memory");
  return (int)x0;
}

/* ---- syscall: mprotect ---- */
static inline int sys_mprotect(void *addr, unsigned long len, int prot) {
  register long x8 __asm__("x8") = 226; /* __NR_mprotect */
  register long x0 __asm__("x0") = (long)addr;
  register long x1 __asm__("x1") = (long)len;
  register long x2 __asm__("x2") = prot;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2) : "memory");
  return (int)x0;
}

/* ---- Text Write Helpers ---- */
static inline void enable_text_write(void *addr) {
  u64 page_start = (u64)addr & ~4095u;
  sys_mprotect((void *)page_start, 4096, 7); /* PROT_READ | PROT_WRITE | PROT_EXEC */
}

static inline void disable_text_write(void *addr) {
  u64 page_start = (u64)addr & ~4095u;
  sys_mprotect((void *)page_start, 4096, 5); /* PROT_READ | PROT_EXEC */
}

/* ---- syscall: munmap ---- */
__attribute__((section(".data.entry"), used)) volatile u64 _so_base = 0;

/*
 * vm_entry — VM 解释器入口
 *
 * 参数:
 *   args           : 指向保存的 X0-X7, callerFP, callerLR (共10个u64)
 *   table_addr     : token 描述符表地址
 *   table_num      : token 描述符表项数
 *   current_func_id: 当前函数 ID
 *   enc_bc         : XOR 加密的字节码
 *   bc_len         : 字节码长度
 *   xor_key        : XOR 解密密钥
 *
 * 返回: R[0] (模拟 X0 返回值)
 */
__attribute__((section(".text.entry"))) u64 vm_entry(u64 *args,
             u64 table_addr, u64 table_num,
             u32 current_func_id,
             u8 *enc_bc, u32 bc_len, u8 xor_key);

/* ================================================================
 * Token 化入口 (条件编译)
 *
 * Token trampoline (3 条指令):
 *   MOV  W16, #token_lo16
 *   MOVK W16, #token_hi16, LSL#16
 *   B    vm_entry_token
 *
 * X16 (IP0) 传递 token，X0-X7 保持调用方原始参数不变。
 * vm_entry_token_asm 负责保存寄存器并调用 vm_entry_token_inner。
 * ================================================================ */
/* TOKEN_ONLY: Token 入口始终编译 */

/* Packer 在 payload 中 patch 此变量为 token 描述符表的 VA */
__attribute__((section(".data.entry"), used)) volatile u64 _token_table_va = 0;

/* 内部 C 函数: 解码 token 并调用 vm_entry */
__attribute__((noinline, section(".text.entry"))) u64
vm_entry_token_inner(u64 *args, u32 token) {
  u8 xor_key = (u8)TOKEN_XOR_KEY(token);
  u32 func_id = TOKEN_FUNC_ID(token);

  /* PIE 兼容: _token_table_va 存储的是相对于自身地址的偏移
   * 用 ADR 获取 _token_table_va 的运行时地址 (PC-relative, ±1MB)
   * 然后加上偏移得到 token 描述符表的实际地址 */
  u64 self_va;
  __asm__ volatile("adr %0, _token_table_va" : "=r"(self_va));
  u64 tbl_off = *(volatile u64 *)&_token_table_va;
  if (__builtin_expect(tbl_off == 0, 0))
    return 0; /* 表未初始化, 安全退出 */

  token_desc_t *table = (token_desc_t *)(self_va + tbl_off);
  /* bc_off 也是相对于 _token_table_va 的偏移 */
  u8 *enc_bc = (u8 *)(self_va + table[func_id].bc_off);
  u32 bc_len = table[func_id].bc_len;

  if (__builtin_expect(enc_bc == (u8 *)self_va || bc_len == 0, 0))
    return 0; /* 无效条目, 安全退出 */

  u64 table_addr = (u64)table;
  u64 table_num = *((u64 *)(((u8 *)(table)) - 8));
  return vm_entry(args, table_addr, table_num, func_id, enc_bc, bc_len, xor_key);
}

/* Naked 汇编入口: 保存调用方寄存器, 调用 C 内部函数 */
__attribute__((naked, section(".text.entry"), used)) void vm_entry_token(void) {
  __asm__ volatile(
      "mov x9, x29\n"               /* 暂存 caller FP */
      "mov x10, x30\n"              /* 暂存 caller LR */
      "stp x29, x30, [sp, #-96]!\n" /* 保存 FP/LR + 分配 96B 栈帧 */
      "mov x29, sp\n"               /* 建立栈帧 */
      "stp x0, x1, [sp, #16]\n"     /* args[0..1] */
      "stp x2, x3, [sp, #32]\n"     /* args[2..3] */
      "stp x4, x5, [sp, #48]\n"     /* args[4..5] */
      "stp x6, x7, [sp, #64]\n"     /* args[6..7] */
      "stp x9, x10, [sp, #80]\n"    /* args[8]=callerFP, args[9]=callerLR */
      "add x0, sp, #16\n"           /* X0 = args 指针 (10 个 u64) */
      "mov w1, w16\n"               /* X1 = token (从 X16/IP0 传入) */
      "bl vm_entry_token_inner\n"   /* 调用 C 内部函数 */
      "ldp x29, x30, [sp], #96\n"   /* 恢复 FP/LR + 释放栈帧 */
      "ret\n"                       /* 返回 (结果在 X0) */
  );
}

/* end TOKEN_ONLY */

static u64 get_so_base_by_name(const char *name) {
  int fd = sys_openat(-100, "/proc/self/maps", 0); /* AT_FDCWD = -100 */
  if (fd < 0) return 0;

  char buf[512];
  char line[512];
  int line_ptr = 0;
  u64 base = 0;

  for (;;) {
    long n = sys_read(fd, buf, sizeof(buf));
    if (n <= 0) break;

    for (int i = 0; i < n; i++) {
      if (buf[i] == '\n') {
        line[line_ptr] = 0;
        /* Minimal string search to avoid libc */
        int found_name = 0;
        for (int j = 0; line[j]; j++) {
          int match = 1;
          for (int k = 0; name[k]; k++) {
            if (line[j + k] != name[k]) {
              match = 0;
              break;
            }
          }
          if (match) {
            found_name = 1;
            break;
          }
        }

        if (found_name) {
          int found_rxp = 0;
          for (int j = 0; line[j]; j++) {
            if (line[j] == 'r' && line[j+1] == '-' && line[j+2] == 'x' && line[j+3] == 'p') {
              found_rxp = 1;
              break;
            }
          }
          if (found_rxp) {
            /* Parse base address (hex) */
            u64 addr = 0;
            for (int j = 0; line[j]; j++) {
              char c = line[j];
              if (c >= '0' && c <= '9') addr = (addr << 4) | (c - '0');
              else if (c >= 'a' && c <= 'f') addr = (addr << 4) | (c - 'a' + 10);
              else if (c >= 'A' && c <= 'F') addr = (addr << 4) | (c - 'A' + 10);
              else break;
            }
            base = addr;
            goto found;
          }
        }
        line_ptr = 0;
      } else if (line_ptr < 511) {
        line[line_ptr++] = buf[i];
      }
    }
  }

found:
  sys_close(fd);
  return base;
}

/* ---- vm_entry 实现 ---- */
__attribute__((section(".text.entry"))) u64 vm_entry(u64 *args,
             u64 table_addr, u64 table_num,
             u32 current_func_id,
             u8 *enc_bc, u32 bc_len, u8 xor_key) {
  u64 ret = 0;

  /* ---- 1. 动态分配字节码缓冲区 (mmap, 替代栈上 64KB) ---- */
  if (bc_len > VM_BYTECODE_MAX)
    bc_len = VM_BYTECODE_MAX;
  u32 alloc_size = (bc_len + 4095u) & ~4095u; /* 页对齐向上取整 */
  u8 *bc_buf = (u8 *)sys_mmap(alloc_size);
  if ((long)bc_buf < 0)
    return 0; /* mmap 失败, 安全退出 */

  /* ---- 1b. XOR 解密 (8 字节加宽, ~8x 加速) ---- */
  u64 xk8 = (u64)xor_key;
  xk8 |= xk8 << 8;
  xk8 |= xk8 << 16;
  xk8 |= xk8 << 32;
  {
    u32 n8 = bc_len >> 3;
    u64 *d8 = (u64 *)bc_buf;
    const u64 *s8 = (const u64 *)enc_bc;
    for (u32 i = 0; i < n8; i++)
      d8[i] = s8[i] ^ xk8;
    for (u32 i = n8 << 3; i < bc_len; i++)
      bc_buf[i] = enc_bc[i] ^ xor_key;
  }

  /* ---- 2b. 初始化 VM 上下文 (mmap 堆分配, 避免 Go/Rust 协程栈溢出) ---- */
  u32 ctx_alloc = (sizeof(vm_ctx_t) + 4095u) & ~4095u;
  vm_ctx_t *vm = (vm_ctx_t *)sys_mmap(ctx_alloc);
  if ((long)vm < 0) {
    sys_munmap(bc_buf, alloc_size);
    return 0;
  }
  vm_ctx_init(vm, args, bc_buf, bc_len);

  /* ---- 2c. 解析字节码尾部 trailer ---- */
  /* 尾部格式 (从末尾向前剥离):
   *   [...bytecode...][BR map entries][reverse(1B)][oc_key(4B)]
   *                    [map_count:u32][func_addr:u64][func_size:u32]
   *
   * 剥离顺序: func_size(4B) → func_addr(8B) → map_count(4B)
   *           → oc_key(4B) → reverse(1B) → BR map entries
   * 固定 trailer 大小: 4+8+4+4+1 = 21B
   */
  if (bc_len >= 21) { /* 最小 trailer: 21B */
    u32 trail_func_size = rd32(&bc_buf[bc_len - 4]);
    u64 trail_func_addr = rd64(&bc_buf[bc_len - 12]);
    u32 trail_map_count = rd32(&bc_buf[bc_len - 16]);
    u32 trail_oc_key = rd32(&bc_buf[bc_len - 20]);
    u8 trail_reverse = bc_buf[bc_len - 21];
    u32 map_data_size =
        trail_map_count * 8 +
        21; /* +21 for reverse+oc_key+map_count+func_addr+func_size */

    /* 设置 OpcodeCryptor 密钥 + reverse 标志 */
    vm->oc_key = trail_oc_key;
    vm->reverse = trail_reverse;

    if (trail_func_addr != 0 && trail_map_count > 0 &&
        map_data_size <= bc_len) {
      vm->func_addr = trail_func_addr;
      vm->func_size = trail_func_size;
      vm->map_count = trail_map_count;
      vm->addr_map = (addr_map_entry_t *)&bc_buf[bc_len - map_data_size];
      vm->bc_len = bc_len - map_data_size; /* 实际字节码不含 trailer */

      /* 插入排序 addr_map (按 arm64_off 升序, 为二分查找准备) */
      /* 注: 使用字段级拷贝避免编译器生成隐式 memcpy (-nostdlib) */
      for (u32 j = 1; j < vm->map_count; j++) {
        u32 t_arm = vm->addr_map[j].arm64_off;
        u32 t_vm = vm->addr_map[j].vm_off;
        int k = (int)j - 1;
        while (k >= 0 && vm->addr_map[k].arm64_off > t_arm) {
          vm->addr_map[k + 1].arm64_off = vm->addr_map[k].arm64_off;
          vm->addr_map[k + 1].vm_off = vm->addr_map[k].vm_off;
          k--;
        }
        vm->addr_map[k + 1].arm64_off = t_arm;
        vm->addr_map[k + 1].vm_off = t_vm;
      }
    } else {
      /* 无 BR map: 只剥离 21B 固定 trailer */
      vm->bc_len = bc_len - 21;
    }
  }

  /* ---- 2d. 运行时重定位 (Relocation) ---- */
  if (table_addr != 0) {
    /* 1. Get so name from token_desc_t table */
    u8 *so_name_info = (u8 *)(table_addr + sizeof(token_desc_t) * table_num);
    u8 so_name_len = so_name_info[0];
    u8 *so_name = so_name_info + 1;

    /* 2. Find so base if not cached */
    if (_so_base == 0) {
      enable_text_write((void *)&_so_base);
      _so_base = get_so_base_by_name((char *)so_name);
      disable_text_write((void *)&_so_base);
    }

    /* 3. Parse relocation table (RTLR magic) */
    u8 *reloc_table_start = (u8 *)((u64)so_name_info + so_name_len + 2);
    u32 magic = rd32(reloc_table_start);
    if (magic == 0x524C5452) { /* "RTLR" */
      u32 reloc_count = rd32(reloc_table_start + 4);
      u8 *reloc_entry = reloc_table_start + 8;
      for (u32 i = 0; i < reloc_count; i++) {
        u64 func_id = rd64(reloc_entry);
        if (func_id == current_func_id) {
          u64 *write_pos = (u64 *)(bc_buf + rd64(reloc_entry + 8));
          u64 offset = rd64(reloc_entry + 16);
          *write_pos = _so_base + offset;
        }
        if (func_id > current_func_id)
          break;
        reloc_entry += 24;
      }
    }
  }

/* ---- OpcodeCryptor 解密宏 (两种模式共用) ---- */
#define OC_DECRYPT(pc, key) ((u8)((key) ^ ((pc) * 0x9E3779B9u)))

#ifdef VM_INDIRECT_DISPATCH
  /* ================================================================
   * 间接 Dispatch 模式: 相对偏移跳转表 + 函数指针间接调用
   *
   * 替代 computed goto，使 IDA Pro 等静态分析工具
   * 无法追踪所有 handler 目标地址。
   * ================================================================ */

  /* ---- 运行时初始化跳转表 (栈上分配, RX blob 不可写 BSS) ---- */
  vm_handler_fn vm_jump_table[256];
  vm_init_jump_table(vm_jump_table);

  /* ---- PC 初始化: reverse 模式从 bc_len 开始 ---- */
  if (vm->reverse) {
    vm->pc = vm->bc_len;
  }

  /* ---- 间接 Dispatch 主循环 ---- */
  for (;;) {
    /* -- 反向/正向 PC 定位 -- */
    if (vm->reverse) {
      if (__builtin_expect((i64)vm->pc <= 0, 0))
        break;
      vm->pc--;
      if (__builtin_expect(vm->pc >= vm->bc_len, 0))
        break;
      u8 _sz = vm->bc[vm->pc];
      if (__builtin_expect(_sz > vm->pc, 0))
        break;
      vm->pc -= _sz;
    } else {
      if (__builtin_expect(vm->pc >= vm->bc_len, 0))
        break;
    }

    /* -- OpcodeCryptor 解密 -- */
    u8 _raw_op = vm->bc[vm->pc];
    u8 _dec_op = _raw_op ^ OC_DECRYPT(vm->pc, vm->oc_key);

    /* -- 指令大小校验 -- */
    u8 _isz = vm_insn_size(_dec_op);
    if (__builtin_expect(_isz == 0 || vm->pc + _isz > vm->bc_len, 0))
      break;

    /* -- 特殊处理: HALT / RET (不经过跳转表) -- */
    if (_dec_op == OP_HALT) {
      ret = vm->R[0];
      goto cleanup;
    }
    if (_dec_op == OP_RET) {
      u8 _r = vm->bc[vm->pc + 1];
      ret = vm->R[_r & 31];
      goto cleanup;
    }

    /* -- 间接 Dispatch: 直接从跳转表取函数指针调用 -- */
#ifdef VM_DEBUG_TRACE
    /* -- Debug trace: 输出 pc+op 到 stderr -- */
    {
      u8 _tbuf[16];
/* 内联计算十六进制字符 (避免 static 数据引用) */
#define _HX(n) ((u8)((n) < 10 ? '0' + (n) : 'A' + (n) - 10))
      _tbuf[0] = _HX((vm->pc >> 12) & 0xF);
      _tbuf[1] = _HX((vm->pc >> 8) & 0xF);
      _tbuf[2] = _HX((vm->pc >> 4) & 0xF);
      _tbuf[3] = _HX(vm->pc & 0xF);
      _tbuf[4] = ':';
      _tbuf[5] = _HX((_dec_op >> 4) & 0xF);
      _tbuf[6] = _HX(_dec_op & 0xF);
      _tbuf[7] = '\n';
#undef _HX
      register long _x8 __asm__("x8") = 64; /* __NR_write */
      register long _x0 __asm__("x0") = 2;  /* stderr */
      register long _x1 __asm__("x1") = (long)_tbuf;
      register long _x2 __asm__("x2") = 8;
      __asm__ volatile("svc #0"
                       : "+r"(_x0)
                       : "r"(_x8), "r"(_x1), "r"(_x2)
                       : "memory");
    }
#endif
    vm_handler_fn _handler = vm_jump_table[_dec_op];
    u32 _step = _handler(vm);

    /* -- 检查 HALT 哨兵 (wrap_unknown 等返回) -- */
    if (__builtin_expect(_step == VM_STEP_HALT, 0)) {
      ret = vm->R[0];
      goto cleanup;
    }

    /* -- 推进 PC -- */
    /* _step == 0: 分支 handler 已直接设置 pc, 不推进 */
    /* _step > 0 且非 reverse: 正常推进 */
    if (_step > 0 && !vm->reverse) {
      vm->pc += _step;
    }
  }

#else /* !VM_INDIRECT_DISPATCH */

  /* ================================================================
   * 原始 Computed Goto 模式 (保持不变)
   * ================================================================ */

  /* ---- 3. Computed goto 分发表 (替代 switch-case, ~20-30% 加速) ---- */
  /* GCC 扩展: &&label 获取标签地址, goto *ptr 跳转 */
  /* 注: 使用循环填充默认值避免 [0...255] 范围初始化生成隐式 memcpy */
  const void *dtab[256];
  for (int _i = 0; _i < 256; _i++)
    dtab[_i] = &&L_UNKNOWN;
  /* 系统 */
  dtab[OP_NOP] = &&L_NOP;
  dtab[OP_HALT] = &&L_HALT;
  dtab[OP_RET] = &&L_RET;
  /* 数据移动 */
  dtab[OP_MOV_IMM] = &&L_MOV_IMM;
  dtab[OP_MOV_IMM32] = &&L_MOV_IMM32;
  dtab[OP_MOV_REG] = &&L_MOV_REG;
  /* 内存 */
  dtab[OP_LOAD8] = &&L_LOAD8;
  dtab[OP_LOAD32] = &&L_LOAD32;
  dtab[OP_LOAD64] = &&L_LOAD64;
  dtab[OP_STORE8] = &&L_STORE8;
  dtab[OP_STORE32] = &&L_STORE32;
  dtab[OP_STORE64] = &&L_STORE64;
  dtab[OP_LOAD16] = &&L_LOAD16;
  dtab[OP_STORE16] = &&L_STORE16;
  /* ALU 三寄存器 */
  dtab[OP_ADD] = &&L_ADD;
  dtab[OP_SUB] = &&L_SUB;
  dtab[OP_MUL] = &&L_MUL;
  dtab[OP_XOR] = &&L_XOR;
  dtab[OP_AND] = &&L_AND;
  dtab[OP_OR] = &&L_OR;
  dtab[OP_SHL] = &&L_SHL;
  dtab[OP_SHR] = &&L_SHR;
  dtab[OP_ASR] = &&L_ASR;
  dtab[OP_NOT] = &&L_NOT;
  dtab[OP_ROR] = &&L_ROR;
  dtab[OP_UMULH] = &&L_UMULH;
  /* ALU 立即数 */
  dtab[OP_ADD_IMM] = &&L_ADD_IMM;
  dtab[OP_SUB_IMM] = &&L_SUB_IMM;
  dtab[OP_XOR_IMM] = &&L_XOR_IMM;
  dtab[OP_AND_IMM] = &&L_AND_IMM;
  dtab[OP_OR_IMM] = &&L_OR_IMM;
  dtab[OP_MUL_IMM] = &&L_MUL_IMM;
  dtab[OP_SHL_IMM] = &&L_SHL_IMM;
  dtab[OP_SHR_IMM] = &&L_SHR_IMM;
  dtab[OP_ASR_IMM] = &&L_ASR_IMM;
  /* 比较 */
  dtab[OP_CMP] = &&L_CMP;
  dtab[OP_CMP_IMM] = &&L_CMP_IMM;
  /* 分支 */
  dtab[OP_JMP] = &&L_JMP;
  dtab[OP_JE] = &&L_JE;
  dtab[OP_JNE] = &&L_JNE;
  dtab[OP_JL] = &&L_JL;
  dtab[OP_JGE] = &&L_JGE;
  dtab[OP_JGT] = &&L_JGT;
  dtab[OP_JLE] = &&L_JLE;
  dtab[OP_JB] = &&L_JB;
  dtab[OP_JAE] = &&L_JAE;
  dtab[OP_JBE] = &&L_JBE;
  dtab[OP_JA] = &&L_JA;
  /* 栈操作 */
  dtab[OP_PUSH] = &&L_PUSH;
  dtab[OP_POP] = &&L_POP;
  /* 原生调用 */
  dtab[OP_CALL_NAT] = &&L_CALL_NAT;
  dtab[OP_CALL_REG] = &&L_CALL_REG;
  dtab[OP_BR_REG] = &&L_BR_REG;
  /* SIMD */
  dtab[OP_VLD16] = &&L_VLD16;
  dtab[OP_VST16] = &&L_VST16;
  /* TBZ/TBNZ */
  dtab[OP_TBZ] = &&L_TBZ;
  dtab[OP_TBNZ] = &&L_TBNZ;
  /* CCMP/CCMN */
  dtab[OP_CCMP_REG] = &&L_CCMP_REG;
  dtab[OP_CCMP_IMM] = &&L_CCMP_IMM;
  dtab[OP_CCMN_REG] = &&L_CCMN_REG;
  dtab[OP_CCMN_IMM] = &&L_CCMN_IMM;
  /* SVC */
  dtab[OP_SVC] = &&L_SVC;
  /* UDIV/SDIV */
  dtab[OP_UDIV] = &&L_UDIV;
  dtab[OP_SDIV] = &&L_SDIV;
  /* MRS */
  dtab[OP_MRS] = &&L_MRS;

/* 反向模式: pc 指向指令末尾的 size 标记之后
 * 步骤: pc--; size = bc[pc]; pc -= size; 现在 pc 指向指令起始 */
#define DISPATCH()                                                             \
  do {                                                                         \
    if (vm->reverse) {                                                         \
      if (__builtin_expect((i64)vm->pc <= 0, 0))                               \
        goto cleanup;                                                          \
      vm->pc--;                                                                \
      if (__builtin_expect(vm->pc >= vm->bc_len, 0))                           \
        goto cleanup;                                                          \
      u8 _sz = vm->bc[vm->pc];                                                 \
      if (__builtin_expect(_sz > vm->pc, 0))                                   \
        goto cleanup;                                                          \
      vm->pc -= _sz;                                                           \
    } else {                                                                   \
      if (__builtin_expect(vm->pc >= vm->bc_len, 0))                           \
        goto cleanup;                                                          \
    }                                                                          \
    u8 _raw_op = vm->bc[vm->pc];                                               \
    u8 _dec_op = _raw_op ^ OC_DECRYPT(vm->pc, vm->oc_key);                     \
    u8 _isz = vm_insn_size(_dec_op);                                           \
    if (__builtin_expect(_isz == 0 || vm->pc + _isz > vm->bc_len, 0))          \
      goto cleanup;                                                            \
    goto *dtab[_dec_op];                                                       \
  } while (0)

/* NEXT: handler 必须总是执行; 正向 pc += n, 反向忽略 advance */
#define NEXT(n)                                                                \
  do {                                                                         \
    u32 _adv = (n);                                                            \
    __asm__ volatile("" ::: "memory");                                         \
    if (!vm->reverse)                                                          \
      vm->pc += _adv;                                                          \
    DISPATCH();                                                                \
  } while (0)
#define NEXT0() DISPATCH() /* handler 已设置 pc */

  /* ---- PC 初始化: reverse 模式从 bc_len 开始 ---- */
  if (vm->reverse) {
    vm->pc = vm->bc_len; /* DISPATCH 会先递减定位到最后一条指令 */
  }

  /* ---- 开始执行 ---- */
  DISPATCH();

/* ---- 系统 ---- */
L_NOP:
  NEXT(h_nop(vm));
L_HALT:
  ret = vm->R[0];
  goto cleanup;
L_RET: {
  u8 r = vm->bc[vm->pc + 1];
  ret = vm->R[r & 31];
  goto cleanup;
}

/* ---- 数据移动 ---- */
L_MOV_IMM:
  NEXT(h_mov_imm(vm));
L_MOV_IMM32:
  NEXT(h_mov_imm32(vm));
L_MOV_REG:
  NEXT(h_mov_reg(vm));

/* ---- 内存访问 ---- */
L_LOAD8:
  NEXT(h_load8(vm));
L_LOAD32:
  NEXT(h_load32(vm));
L_LOAD64:
  NEXT(h_load64(vm));
L_STORE8:
  NEXT(h_store8(vm));
L_STORE32:
  NEXT(h_store32(vm));
L_STORE64:
  NEXT(h_store64(vm));
L_LOAD16:
  NEXT(h_load16(vm));
L_STORE16:
  NEXT(h_store16(vm));

/* ---- ALU 三寄存器 ---- */
L_ADD:
  NEXT(h_add(vm));
L_SUB:
  NEXT(h_sub(vm));
L_MUL:
  NEXT(h_mul(vm));
L_XOR:
  NEXT(h_xor(vm));
L_AND:
  NEXT(h_and(vm));
L_OR:
  NEXT(h_or(vm));
L_SHL:
  NEXT(h_shl(vm));
L_SHR:
  NEXT(h_shr(vm));
L_ASR:
  NEXT(h_asr(vm));
L_NOT:
  NEXT(h_not(vm));
L_ROR:
  NEXT(h_ror(vm));
L_UMULH:
  NEXT(h_umulh(vm));

/* ---- ALU 立即数 ---- */
L_ADD_IMM:
  NEXT(h_add_imm(vm));
L_SUB_IMM:
  NEXT(h_sub_imm(vm));
L_XOR_IMM:
  NEXT(h_xor_imm(vm));
L_AND_IMM:
  NEXT(h_and_imm(vm));
L_OR_IMM:
  NEXT(h_or_imm(vm));
L_MUL_IMM:
  NEXT(h_mul_imm(vm));
L_SHL_IMM:
  NEXT(h_shl_imm(vm));
L_SHR_IMM:
  NEXT(h_shr_imm(vm));
L_ASR_IMM:
  NEXT(h_asr_imm(vm));

/* ---- 比较 ---- */
L_CMP:
  NEXT(h_cmp(vm));
L_CMP_IMM:
  NEXT(h_cmp_imm(vm));

/* ---- 分支 (handler 返回 0, 已设置 pc) ---- */
L_JMP:
  h_jmp(vm);
  NEXT0();
L_JE:
  h_je(vm);
  NEXT0();
L_JNE:
  h_jne(vm);
  NEXT0();
L_JL:
  h_jl(vm);
  NEXT0();
L_JGE:
  h_jge(vm);
  NEXT0();
L_JGT:
  h_jgt(vm);
  NEXT0();
L_JLE:
  h_jle(vm);
  NEXT0();
L_JB:
  h_jb(vm);
  NEXT0();
L_JAE:
  h_jae(vm);
  NEXT0();
L_JBE:
  h_jbe(vm);
  NEXT0();
L_JA:
  h_ja(vm);
  NEXT0();

/* ---- 栈操作 ---- */
L_PUSH:
  NEXT(h_push(vm));
L_POP:
  NEXT(h_pop(vm));

/* ---- 原生调用 ---- */
L_CALL_NAT:
  NEXT(h_call_nat(vm));
L_CALL_REG:
  NEXT(h_call_reg(vm));
L_BR_REG: {
  u32 a = h_br_reg(vm);
  if (a)
    NEXT(a);
  else
    NEXT0();
}

/* ---- SIMD ---- */
L_VLD16:
  NEXT(h_vld16(vm));
L_VST16:
  NEXT(h_vst16(vm));

/* ---- TBZ/TBNZ (分支, handler 返回 0, 已设置 pc) ---- */
L_TBZ:
  h_tbz(vm);
  NEXT0();
L_TBNZ:
  h_tbnz(vm);
  NEXT0();

/* ---- CCMP/CCMN ---- */
L_CCMP_REG:
  NEXT(h_ccmp_reg(vm));
L_CCMP_IMM:
  NEXT(h_ccmp_imm(vm));
L_CCMN_REG:
  NEXT(h_ccmn_reg(vm));
L_CCMN_IMM:
  NEXT(h_ccmn_imm(vm));

/* ---- SVC ---- */
L_SVC:
  NEXT(h_svc(vm));

/* ---- UDIV/SDIV ---- */
L_UDIV:
  NEXT(h_udiv(vm));
L_SDIV:
  NEXT(h_sdiv(vm));

/* ---- MRS ---- */
L_MRS:
  NEXT(h_mrs(vm));

/* ---- 未知指令 ---- */
L_UNKNOWN:
  ret = vm->R[0]; /* fall through to cleanup */

#undef DISPATCH
#undef NEXT
#undef NEXT0

#endif /* VM_INDIRECT_DISPATCH */

  /* ---- 统一退出: 释放 mmap 防止泄漏 ---- */
cleanup:
  sys_munmap(vm, ctx_alloc);
  sys_munmap(bc_buf, alloc_size);
  return ret;
}
