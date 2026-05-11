/* vm_interp.lds — 将所有 sections 合并到一个可执行段 */
SECTIONS {
  . = 0;
  .text : {
    *vm_entry.o(.text.entry)
    *(.text.entry)
    *(.text.vm_alu)
    *(.text.vm_mem)
    *(.text.vm_branch)
    *(.text.vm_system)
    *(.text*)
    *(.rodata*)
    *(.data*)
    *(.bss*)
  }
  /DISCARD/ : {
    *(.comment)
    *(.note*)
    *(.eh_frame*)
    *(.ARM.*)
  }
}
