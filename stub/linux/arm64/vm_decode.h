/*
 * vm_decode.h — 字节码读取工具函数
 *
 * Little-endian reads: read 16/32/64-bit values from bytecode stream.
 */
#ifndef VM_DECODE_H
#define VM_DECODE_H

#include "vm_types.h"

static inline u16 rd16(const u8 *p) { return (u16)p[0] | ((u16)p[1] << 8); }

static inline u32 rd32(const u8 *p) {
  return (u32)p[0] | ((u32)p[1] << 8) | ((u32)p[2] << 16) | ((u32)p[3] << 24);
}

static inline u64 rd64(const u8 *p) {
  return (u64)rd32(p) | ((u64)rd32(p + 4) << 32);
}

/* ---- 指令大小查找 (用于越界保护) ---- */
/* 返回解密后 opcode 对应的指令字节数, 0 = 未知 */
#include "vm_opcodes.h"
static inline u8 vm_insn_size(u8 op) {
  switch (op) {
  case OP_NOP:
  case OP_HALT:
    return 1;
  case OP_RET:
  case OP_PUSH:
  case OP_POP:
  case OP_CALL_REG:
  case OP_BR_REG:
    return 2;
  case OP_MOV_REG:
  case OP_NOT:
  case OP_CMP:
  case OP_VLD16:
  case OP_VST16:
  case OP_SVC:
  case OP_CLZ:
  case OP_CLS:
  case OP_RBIT:
  case OP_REV:
  case OP_REV16:
  case OP_REV32:
    return 3;
  case OP_ADD:
  case OP_SUB:
  case OP_MUL:
  case OP_XOR:
  case OP_AND:
  case OP_OR:
  case OP_SHL:
  case OP_SHR:
  case OP_ASR:
  case OP_ROR:
  case OP_UMULH:
  case OP_UDIV:
  case OP_SDIV:
  case OP_MRS:
  case OP_SMULH:
  case OP_ADC:
  case OP_SBC:
    return 4;
  case OP_LOAD8:
  case OP_LOAD16:
  case OP_LOAD32:
  case OP_LOAD64:
  case OP_STORE8:
  case OP_STORE16:
  case OP_STORE32:
  case OP_STORE64:
  case OP_JMP:
  case OP_JE:
  case OP_JNE:
  case OP_JL:
  case OP_JGE:
  case OP_JGT:
  case OP_JLE:
  case OP_JB:
  case OP_JAE:
  case OP_JBE:
  case OP_JA:
    return 5;
  case OP_MOV_IMM32:
  case OP_CMP_IMM:
  case OP_CCMP_REG:
  case OP_CCMP_IMM:
  case OP_CCMN_REG:
  case OP_CCMN_IMM:
    return 6;
  case OP_ADD_IMM:
  case OP_SUB_IMM:
  case OP_XOR_IMM:
  case OP_AND_IMM:
  case OP_OR_IMM:
  case OP_MUL_IMM:
  case OP_SHL_IMM:
  case OP_SHR_IMM:
  case OP_ASR_IMM:
  case OP_TBZ:
  case OP_TBNZ:
    return 7;
  case OP_CALL_NAT:
    return 9;
  case OP_MOV_IMM:
    return 10;
  /* ---- 栈机器操作码 ---- */
  case OP_S_DUP:
  case OP_S_SWAP:
  case OP_S_DROP:
  case OP_S_ADD:
  case OP_S_SUB:
  case OP_S_MUL:
  case OP_S_XOR:
  case OP_S_AND:
  case OP_S_OR:
  case OP_S_SHL:
  case OP_S_SHR:
  case OP_S_ASR:
  case OP_S_ROR:
  case OP_S_UMULH:
  case OP_S_SMULH:
  case OP_S_UDIV:
  case OP_S_SDIV:
  case OP_S_ADC:
  case OP_S_SBC:
  case OP_S_NOT:
  case OP_S_NEG:
  case OP_S_CLZ:
  case OP_S_CLS:
  case OP_S_RBIT:
  case OP_S_REV:
  case OP_S_REV16:
  case OP_S_REV32:
  case OP_S_TRUNC32:
  case OP_S_SEXT32:
  case OP_S_LOAD_SLIDE:
  case OP_S_CMP:
  case OP_S_LD8:
  case OP_S_LD16:
  case OP_S_LD32:
  case OP_S_LD64:
  case OP_S_ST8:
  case OP_S_ST16:
  case OP_S_ST32:
  case OP_S_ST64:
    return 1;
  case OP_S_VLOAD:
  case OP_S_VSTORE:
    return 2;
  case OP_S_PUSH_IMM32:
    return 5;
  case OP_S_PUSH_IMM64:
    return 9;
  case OP_SVLD:
  case OP_SVST:
    return 3;
  case OP_SFADD:
  case OP_SFSUB:
  case OP_SFMUL:
  case OP_SFDIV:
  case OP_SFMAX:
  case OP_SFMIN:
    return 5;
  case OP_SFMOV:
  case OP_SFCMP:
  case OP_SFNEG:
  case OP_SFABS:
  case OP_SFSQRT:
  case OP_SFCVTIF:
  case OP_SFCVTFI:
  case OP_SFMOVRV:
  case OP_SFMOVVR:
  case OP_SFCVT:
    return 4;
  default:
    return 0;
  }
}

/* ---- 逻辑指令大小查找 (用于动态码表) ---- */
#include "vm_opcodes_dynamic.h"
static inline u8 vm_logical_insn_size(u8 id) {
  switch (id) {
  case OP_ID_NOP:
  case OP_ID_HALT:
  case OP_ID_SDUP:
  case OP_ID_SSWAP:
  case OP_ID_SDROP:
  case OP_ID_SADD:
  case OP_ID_SSUB:
  case OP_ID_SMUL:
  case OP_ID_SXOR:
  case OP_ID_SAND:
  case OP_ID_SOR:
  case OP_ID_SSHL:
  case OP_ID_SSHR:
  case OP_ID_SASR:
  case OP_ID_SROR:
  case OP_ID_SUMULH:
  case OP_ID_SSMULH:
  case OP_ID_SUDIV:
  case OP_ID_SSDIV:
  case OP_ID_SADC:
  case OP_ID_SSBC:
  case OP_ID_SNOT:
  case OP_ID_SNEG:
  case OP_ID_SCLZ:
  case OP_ID_SCLS:
  case OP_ID_SRBIT:
  case OP_ID_SREV:
  case OP_ID_SREV16:
  case OP_ID_SREV32:
  case OP_ID_STRUNC32:
  case OP_ID_SSEXT32:
  case OP_ID_SLOADSLIDE:
  case OP_ID_SCMP:
  case OP_ID_SLD8:
  case OP_ID_SLD16:
  case OP_ID_SLD32:
  case OP_ID_SLD64:
  case OP_ID_SST8:
  case OP_ID_SST16:
  case OP_ID_SST32:
  case OP_ID_SST64:
  case OP_ID_SDECRYPTSTR:
    return 1;
  case OP_ID_RET:
  case OP_ID_PUSH:
  case OP_ID_POP:
  case OP_ID_CALLREG:
  case OP_ID_BRREG:
  case OP_ID_SVLOAD:
  case OP_ID_SVSTORE:
    return 2;
  case OP_ID_MOVREG:
  case OP_ID_NOT:
  case OP_ID_CMP:
  case OP_ID_VLD16:
  case OP_ID_VST16:
  case OP_ID_SVC:
  case OP_ID_CLZ:
  case OP_ID_CLS:
  case OP_ID_RBIT:
  case OP_ID_REV:
  case OP_ID_REV16:
  case OP_ID_REV32:
  case OP_ID_SVLD:
  case OP_ID_SVST:
    return 3;
  case OP_ID_ADD:
  case OP_ID_SUB:
  case OP_ID_MUL:
  case OP_ID_XOR:
  case OP_ID_AND:
  case OP_ID_OR:
  case OP_ID_SHL:
  case OP_ID_SHR:
  case OP_ID_ASR:
  case OP_ID_ROR:
  case OP_ID_UMULH:
  case OP_ID_UDIV:
  case OP_ID_SDIV:
  case OP_ID_MRS:
  case OP_ID_SMULH:
  case OP_ID_ADC:
  case OP_ID_SBC:
  case OP_ID_SFMOV:
  case OP_ID_SFCMP:
  case OP_ID_SFNEG:
  case OP_ID_SFABS:
  case OP_ID_SFSQRT:
  case OP_ID_SFCVTIF:
  case OP_ID_SFCVTFI:
  case OP_ID_SFMOVRV:
  case OP_ID_SFMOVVR:
  case OP_ID_SFCVT:
    return 4;
  case OP_ID_LOAD8:
  case OP_ID_LOAD16:
  case OP_ID_LOAD32:
  case OP_ID_LOAD64:
  case OP_ID_STORE8:
  case OP_ID_STORE16:
  case OP_ID_STORE32:
  case OP_ID_STORE64:
  case OP_ID_JMP:
  case OP_ID_JE:
  case OP_ID_JNE:
  case OP_ID_JL:
  case OP_ID_JGE:
  case OP_ID_JGT:
  case OP_ID_JLE:
  case OP_ID_JB:
  case OP_ID_JAE:
  case OP_ID_JBE:
  case OP_ID_JA:
  case OP_ID_JVS:
  case OP_ID_JVC:
  case OP_ID_SPUSHIMM32:
  case OP_ID_SFADD:
  case OP_ID_SFSUB:
  case OP_ID_SFMUL:
  case OP_ID_SFDIV:
  case OP_ID_SFMAX:
  case OP_ID_SFMIN:
    return 5;
  case OP_ID_MOVIMM32:
  case OP_ID_CMPIMM:
  case OP_ID_CCMPREG:
  case OP_ID_CCMPIMM:
  case OP_ID_CCMNREG:
  case OP_ID_CCMNIMM:
    return 6;
  case OP_ID_ADDIMM:
  case OP_ID_SUBIMM:
  case OP_ID_XORIMM:
  case OP_ID_ANDIMM:
  case OP_ID_ORIMM:
  case OP_ID_MULIMM:
  case OP_ID_SHLIMM:
  case OP_ID_SHRIMM:
  case OP_ID_ASRIMM:
  case OP_ID_TBZ:
  case OP_ID_TBNZ:
    return 7;
  case OP_ID_CALLNATIVE:
  case OP_ID_SPUSHIMM64:
  case OP_ID_SPRINTF:
    return 9;
  case OP_ID_MOVIMM:
    return 10;
  default:
    return 0;
  }
}

#endif /* VM_DECODE_H */
