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
    ret
