#ifndef VM_DECODE_H
#define VM_DECODE_H

#include "vm_types.h"
#include "vm_opcodes_dynamic.h"

/* Instruction size lookup based on logical opcode ID */
static u32 vm_logical_insn_size(u8 op_id) {
  switch (op_id) {
  case OP_ID_NOP:
  case OP_ID_HALT:
  case OP_ID_SLOADSLIDE:
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

  case OP_ID_PUSH:
  case OP_ID_POP:
  case OP_ID_CALLREG:
  case OP_ID_BRREG:
  case OP_ID_RET:
  case OP_ID_SVLOAD:
  case OP_ID_SVSTORE:
  case OP_ID_SVLDV:
  case OP_ID_SVSTV:
    return 2;

  case OP_ID_MOVREG:
  case OP_ID_CMP:
  case OP_ID_NOT:
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
  case OP_ID_SNPRINTF:
    return 9;

  case OP_ID_MOVIMM:
    return 10;

  default:
    return 0;
  }
}

/* Helper to read 16-bit LE value */
static u16 rd16(const u8 *p) {
  return (u16)p[0] | ((u16)p[1] << 8);
}

/* Helper to read 32-bit LE value */
static u32 rd32(const u8 *p) {
  return (u32)p[0] | ((u32)p[1] << 8) | ((u32)p[2] << 16) | ((u32)p[3] << 24);
}

/* Helper to read 64-bit LE value */
static u64 rd64(const u8 *p) {
  return (u64)rd32(p) | ((u64)rd32(p + 4) << 32);
}

/* Helper to write 32-bit LE value */
static void wr32(u8 *p, u32 val) {
  p[0] = (u8)val;
  p[1] = (u8)(val >> 8);
  p[2] = (u8)(val >> 16);
  p[3] = (u8)(val >> 24);
}

/* Helper to write 64-bit LE value */
static void wr64(u8 *p, u64 val) {
  wr32(p, (u32)val);
  wr32(p + 4, (u32)(val >> 32));
}

#endif /* VM_DECODE_H */
