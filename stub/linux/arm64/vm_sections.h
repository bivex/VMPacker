/*
 * vm_sections.h — Handler Section Allocation Macros
 *
 * When VM_FUNC_SPLIT macro is defined, disperse handler wrapper functions into
 * in different ELF sections, making IDA Pro recognize each fragment as an
 * independent function.
 *
 * Section Grouping:
 *   .text.vm_alu    — ALU Operations (add/sub/mul/xor/and/or/shl/shr/asr/not/ror + _imm)
 *   .text.vm_mem    — Memory Access (load/store 8/32/64, mov_imm/imm32/reg)
 *   .text.vm_branch — Branch (jmp/je/jne/jl/jge/jgt/jle/jb/jae/jbe/ja)
 *   .text.vm_system — System (nop/halt/ret/call_nat/call_reg/br_reg/push/pop/vld16/vst16)
 *
 * When not enabled, the macro expands to empty, not affecting compilation.
 */
#ifndef VM_SECTIONS_H
#define VM_SECTIONS_H

#ifdef VM_FUNC_SPLIT
  #define VM_SECTION_ALU     __attribute__((section(".text.vm_alu")))
  #define VM_SECTION_MEM     __attribute__((section(".text.vm_mem")))
  #define VM_SECTION_BRANCH  __attribute__((section(".text.vm_branch")))
  #define VM_SECTION_SYSTEM  __attribute__((section(".text.vm_system")))
#else
  #define VM_SECTION_ALU
  #define VM_SECTION_MEM
  #define VM_SECTION_BRANCH
  #define VM_SECTION_SYSTEM
#endif

#endif /* VM_SECTIONS_H */
