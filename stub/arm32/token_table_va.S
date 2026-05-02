/* token_table_va.S — _token_table_va + _link_time_self_va in .text.entry
 * ADR requires same-file symbol; packer patches both words at runtime.
 * 使用 .arm 与 stub 保持一致，避免 ARM↔Thumb interworking 潜在问题。 */
.section .text.entry, "ax", %progbits
.arm
.syntax unified

/* Returns runtime address of _token_table_va (for PIE). */
.global get_self_va
get_self_va:
  adr r0, _token_table_va
  bx lr

.align 2
.global _token_table_va
_token_table_va:
  .word 0               /* patched by packer: relative offset to token table */

.global _link_time_self_va
_link_time_self_va:
  .word 0               /* patched by packer: link-time VA of _token_table_va */

.section .note.GNU-stack, "", %progbits
