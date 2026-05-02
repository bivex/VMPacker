/* vm_interp_arm32.lds — ARM32 linker script: merge all sections into one executable segment */
SECTIONS {
  . = 0;
  .text : {
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
