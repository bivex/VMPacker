.section .text.entry
.global vm_entry_token
.type vm_entry_token, %function

vm_entry_token:
    /* Stack frame 304B: FP/LR(16) + X0-X7(64) + callerFP/LR(16) + V0-V7(128) + X19-X28(80) */
    mov x9, x29               /* Save caller FP */
    mov x10, x30              /* Save caller LR */
    stp x29, x30, [sp, #-304]! /* Save FP/LR + Allocate 304B frame */
    mov x29, sp               /* Establish frame */
    
    stp x0, x1, [sp, #16]     /* args[0..1] = X0, X1 */
    stp x2, x3, [sp, #32]     /* args[2..3] = X2, X3 */
    stp x4, x5, [sp, #48]     /* args[4..5] = X4, X5 */
    stp x6, x7, [sp, #64]     /* args[6..7] = X6, X7 */
    stp x9, x10, [sp, #80]    /* args[8..9] = callerFP, callerLR */
    
    stp q0, q1, [sp, #96]     /* args[10..13] = V0, V1 (128-bit each) */
    stp q2, q3, [sp, #128]    /* args[14..17] = V2, V3 */
    stp q4, q5, [sp, #160]    /* args[18..21] = V4, V5 */
    stp q6, q7, [sp, #192]    /* args[22..25] = V6, V7 */
    
    stp x19, x20, [sp, #224]  /* args[26..27] = X19, X20 */
    stp x21, x22, [sp, #240]  /* args[28..29] = X21, X22 */
    stp x23, x24, [sp, #256]  /* args[30..31] = X23, X24 */
    stp x25, x26, [sp, #272]  /* args[32..33] = X25, X26 */
    stp x27, x28, [sp, #288]  /* args[34..35] = X27, X28 */
    
    add x0, sp, #16           /* X0 = args pointer */
    mov w1, w16               /* X1 = token (from X16/IP0) */
    bl vm_entry_token_inner   /* Call C internal function, result in X0 */
    
    ldp x19, x20, [sp, #224]  /* Restore callee-saved X19, X20 */
    ldp x21, x22, [sp, #240]  /* Restore X21, X22 */
    ldp x23, x24, [sp, #256]  /* Restore X23, X24 */
    ldp x25, x26, [sp, #272]  /* Restore X25, X26 */
    ldp x27, x28, [sp, #288]  /* Restore X27, X28 */
    ldp x29, x30, [sp], #304  /* Restore FP/LR + Free frame */
    ret                       /* Return (result in X0) */
