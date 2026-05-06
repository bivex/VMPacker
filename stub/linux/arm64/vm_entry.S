.section .text.entry
.global _vmp_stub_base
_vmp_stub_base:

.global vm_entry_token
.type vm_entry_token, %function

vm_entry_token:
    /* Force 16-byte alignment of SP */
    mov x9, sp
    and x9, x9, #~15
    mov x10, sp               /* Save original SP */
    mov sp, x9
    
    /* Stack frame 320B: FP/LR(16) + args[0..36](296) + Padding(8) */
    stp x29, x30, [sp, #-320]!
    mov x29, sp
    
    /* Save original SP as args[37] (slot 37*8 = 296) */
    str x10, [sp, #312]

    /* Save X0-X7 (parameters) -> args[0-7] */
    stp x0, x1, [sp, #16]
    stp x2, x3, [sp, #32]
    stp x4, x5, [sp, #48]
    stp x6, x7, [sp, #64]

    /* Store caller's FP/LR (16B) -> args[8-9] */
    mov x11, x29
    mov x12, x30
    stp x11, x12, [sp, #80]

    /* Store V0-V7 (16B each) -> args[10-25] (128B) */
    stp q0, q1, [sp, #96]
    stp q2, q3, [sp, #128]
    stp q4, q5, [sp, #160]
    stp q6, q7, [sp, #192]

    /* Store X19-X28 (80B) -> args[26-35] */
    stp x19, x20, [sp, #224]
    stp x21, x22, [sp, #240]
    stp x23, x24, [sp, #256]
    stp x25, x26, [sp, #272]
    stp x27, x28, [sp, #288]
    
    /* Store X18 (8B) -> args[36] */
    str x18, [sp, #304]

    /* Pass args array (SP+16) and token (X16) to C function */
    add x0, sp, #16
    mov w1, w16
    bl vm_entry_token_inner

    /* Restore X19-X28 */
    ldp x19, x20, [sp, #224]
    ldp x21, x22, [sp, #240]
    ldp x23, x24, [sp, #256]
    ldp x25, x26, [sp, #272]
    ldp x27, x28, [sp, #288]
    ldr x18, [sp, #304]

    /* Restore original SP */
    ldr x10, [sp, #312]
    ldp x29, x30, [sp], #320
    mov sp, x10
    ret
