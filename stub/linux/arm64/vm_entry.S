.section .text.entry
.global _vmp_stub_base
_vmp_stub_base:
    b _start_real             /* +0: Jump to actual code */
    .word 0                   /* +4: padding */
    .quad vm_entry_token      /* +8: Offset to vm_entry_token */
    .word 0x564D5031          /* +16: Magic "VMP1" */
    .word 0                   /* +20: unused */

_start_real:
    b vm_entry_token

.global vm_entry_token
.type vm_entry_token, %function
.align 4
vm_entry_token:
    /* Save caller FP/LR and X18 */
    mov x11, x29
    mov x12, x30
    mov x13, x18

    /* Force 16-byte alignment of SP */
    mov x9, sp
    and x9, x9, #~15
    mov x10, sp
    mov sp, x9
    
    /* Stack frame 496B (aligned 16, < 504 for LDP post-index) */
    stp x29, x30, [sp, #-496]!
    mov x29, sp
    
    /* Save original SP at end of frame */
    str x10, [sp, #488]

    /* args array starts at SP+16 */
    /* Save parameters X0-X7 (64B) -> args[0-7] (at SP+16..80) */
    stp x0, x1, [sp, #16]
    stp x2, x3, [sp, #32]
    stp x4, x5, [sp, #48]
    stp x6, x7, [sp, #64]

    /* Store caller's FP/LR/X18 (24B) -> args[8-10] (at SP+80..104) */
    stp x11, x12, [sp, #80]
    str x13, [sp, #96]

    /* Store V0-V7 (128B) -> args[12-27] (at SP+112..240) */
    stp q0, q1, [sp, #112]
    stp q2, q3, [sp, #144]
    stp q4, q5, [sp, #176]
    stp q6, q7, [sp, #208]

    /* Store Callee-Saved V8-V15 (128B) -> args[28-43] (at SP+240..368) */
    stp q8, q9, [sp, #240]
    stp q10, q11, [sp, #272]
    stp q12, q13, [sp, #304]
    stp q14, q15, [sp, #336]

    /* Store X19-X28 (80B) -> args[44-53] (at SP+368..448) */
    stp x19, x20, [sp, #368]
    stp x21, x22, [sp, #384]
    stp x23, x24, [sp, #400]
    stp x25, x26, [sp, #416]
    stp x27, x28, [sp, #432]

    /* Store FPSR/FPCR (16B) -> args[54-55] (at SP+448..464) */
    mrs x11, fpsr
    mrs x12, fpcr
    stp x11, x12, [sp, #448]

    /* Pass args pointer (SP+16) and token (X16) */
    add x0, sp, #16
    mov w1, w16
    bl vm_entry_token_inner

    /* Restore ALL callee-saved registers */
    /* Restore X19-X28 */
    ldp x19, x20, [sp, #368]
    ldp x21, x22, [sp, #384]
    ldp x23, x24, [sp, #400]
    ldp x25, x26, [sp, #416]
    ldp x27, x28, [sp, #432]
    
    /* Restore X18 */
    ldr x18, [sp, #96]

    /* Restore V8-V15 (Callee-saved) */
    ldp q8, q9, [sp, #240]
    ldp q10, q11, [sp, #272]
    ldp q12, q13, [sp, #304]
    ldp q14, q15, [sp, #336]

    /* Restore V0-V7 */
    ldp q0, q1, [sp, #112]
    ldp q2, q3, [sp, #144]
    ldp q4, q5, [sp, #176]
    ldp q6, q7, [sp, #208]

    /* Restore FPSR/FPCR */
    ldp x11, x12, [sp, #448]
    msr fpsr, x11
    msr fpcr, x12

    /* Restore FP/LR and SP */
    ldr x10, [sp, #488]
    ldp x29, x30, [sp], #496
    mov sp, x10
    ret
