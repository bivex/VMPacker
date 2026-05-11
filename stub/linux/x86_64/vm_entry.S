.section .text.entry
.global _vmp_stub_base
_vmp_stub_base:
    jmp _start_real            /* +0: Jump to actual code (2 bytes) */
    .byte 0, 0, 0, 0, 0, 0     /* +2: padding to +8 */
    .quad vm_entry_token       /* +8: Offset to vm_entry_token */
    .long 0x564D5031           /* +16: Magic "VMP1" */
    .long 0                    /* +20: unused */

_start_real:
    jmp vm_entry_token

.global vm_entry_token
.type vm_entry_token, @function
vm_entry_token:
    /* Token passed in R11 from trampoline */
    
    /* Save all registers to stack for VM context initialization */
    push %rsp        /* RSP at [RSP-8] (args[13]) */
    push %r15        /* args[12] */
    push %r14        /* args[11] */
    push %r13        /* args[10] */
    push %r12        /* args[9] */
    push %rbp        /* args[8] */
    push %rbx        /* args[7] */
    push %rax        /* args[6] */
    push %r9         /* args[5] */
    push %r8         /* args[4] */
    push %rcx        /* args[3] */
    push %rdx        /* args[2] */
    push %rsi        /* args[1] */
    push %rdi        /* args[0] */

    /* Pass args pointer (RSP) and token (R11) */
    mov %rsp, %rdi
    mov %r11, %rsi
    
    /* Ensure 16-byte stack alignment for C code */
    mov %rsp, %rbp
    and $-16, %rsp
    
    call vm_entry_token_inner
    
    /* Restore stack pointer before popping */
    mov %rbp, %rsp
    
    /* Restore registers in reverse order */
    pop %rdi
    pop %rsi
    pop %rdx
    pop %rcx
    pop %r8
    pop %r9
    pop %rax
    pop %rbx
    pop %rbp
    pop %r12
    pop %r13
    pop %r14
    pop %r15
    pop %rsp

    ret
