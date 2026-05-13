.section .text.entry
.global _vmp_stub_base
_vmp_stub_base:
    jmp vm_entry_token
    .byte 0, 0, 0, 0, 0, 0
    .quad vm_entry_token
    .long 0x564D5031
    .long 0

.global vm_entry_token
.type vm_entry_token, @function
vm_entry_token:
    pushq %rax
    pushq %rbx
    pushq %rbp
    pushq %r12
    pushq %r13
    pushq %r14
    pushq %r15

    movl %r11d, %ebx

    subq $112, %rsp

    movq %rdi, 0(%rsp)
    movq %rsi, 8(%rsp)
    movq %rdx, 16(%rsp)
    movq %rcx, 24(%rsp)
    movq %r8,  32(%rsp)
    movq %r9,  40(%rsp)

    movq 112+48(%rsp), %rax
    movq %rax, 48(%rsp)
    movq 112+40(%rsp), %rax
    movq %rax, 56(%rsp)
    movq 112+32(%rsp), %rax
    movq %rax, 64(%rsp)

    movq 112+24(%rsp), %rax
    movq %rax, 72(%rsp)
    movq 112+16(%rsp), %rax
    movq %rax, 80(%rsp)
    movq 112+8(%rsp), %rax
    movq %rax, 88(%rsp)
    movq 112+0(%rsp), %rax
    movq %rax, 96(%rsp)

    leaq 176(%rsp), %rax
    movq %rax, 104(%rsp)

    movq %rsp, %rdi
    movl %ebx, %esi
    callq vm_entry_token_inner

    addq $112, %rsp

    popq %r15
    popq %r14
    popq %r13
    popq %r12
    popq %rbp
    popq %rbx
    addq $8, %rsp
    retq
