# VMPacker Project Layout

This document describes the current architecture, directory structure, and high-level workflow of the VMPacker project.

## Directory Structure

### `cmd/` (CLI Application)
* **`cmd/vmpacker/`**: The main entry point for the command-line interface.
  * `main.go`: Handles CLI arguments, flags, and orchestrates the overall packing process.

### `pkg/` (Core Go Libraries)
The core business logic of the packer is located here.
* **`pkg/arch/`**: Architecture-specific translation modules.
  * Contains the logic (e.g., `arm64`, `arm32`) to lift native instructions into the Virtual Machine's intermediate representation (IR) or bytecode.
* **`pkg/binary/`**: Binary parsing, manipulation, and injection.
  * **`pkg/binary/elf/`**: Code for parsing ELF files, locating target functions, modifying segments, injecting the VM blob, and patching original functions to redirect execution to the VM.
* **`pkg/vm/`**: Virtual Machine definitions and logic.
  * Defines the dynamic ISA (Instruction Set Architecture), opcodes, and bytecode utilities.
  * Includes bytecode obfuscation features (like opcode encryption and instruction reversal).

### `stub/` (VM Interpreter)
The C/Assembly source code for the runtime Virtual Machine interpreter.
* This code is compiled into a lightweight "blob" that is injected into the target executable. At runtime, it decrypts and interprets the protected bytecode.
* **`stub/linux/`**: The primary C interpreter loop, handlers, and dispatch logic.
* **`stub/arm32/`**: Specific stub implementations for 32-bit ARM.

### `demo/` (Test & Demo Programs)
A collection of C programs used for testing the packer's correctness.
* Each demo targets specific instructions (e.g., `demo_insn_add.c`), control flow structures, or edge cases.
* Includes scripts (`Makefile-demo`) and harnesses (`demo_go_test`, `demo_rust_test`) for automated validation.

### `vmp-gui/` (Graphical Interface)
A cross-platform GUI for the packer.
* Built using the [Wails](https://wails.io/) framework.
* Combines a Go backend (`app.go`) with a frontend web interface (`frontend/`).

### `scripts/` (Build & Utility Scripts)
Helper scripts for the development lifecycle.
* Contains shell scripts (`build_stub32_unix.sh`, `build_stub64_unix.sh`) used to compile the C VM stubs into the embeddable binary blobs used by the Go packer.

### `test/` & `scratch/` (Testing & Experimentation)
* **`test/`**: Integration testing framework (`libvmptest`) to verify that packed binaries execute correctly.
* **`scratch/`**: Sandbox directory for developers to test ideas, inspect disassembly, or write temporary validation scripts (e.g., `check_sizes.go`).

### `docs/` (Documentation)
Project documentation.
* Includes ISA specifications (`ISA.md`), build instructions (`BUILD_ARM32.md`), security models (`RUNTIME_SECURITY.md`), and developer notes.

---

## High-Level Workflow (How it works)

When VMPacker protects a binary, it follows these phases:

1. **Target Identification (`pkg/binary/elf`)**:
   The packer parses the target AArch64/ARM32 ELF file, resolves symbols or provided addresses, and locates the raw machine code of the function to be protected.

2. **Translation (`pkg/arch`)**:
   The native instructions are decoded and translated into the VMPacker's custom VM bytecode. 

3. **Obfuscation (`pkg/vm` & `pkg/binary/elf`)**:
   The bytecode may be subjected to transformations like Control Flow Flattening (CFF) or Mixed Boolean Arithmetic (MBA). Finally, the bytecode array is reversed and opcodes are encrypted to thwart static analysis.

4. **Injection (`pkg/binary/elf`)**:
   A new executable segment is added to the ELF file. The pre-compiled VM interpreter (`stub/`) and the encrypted bytecode payload are injected into this segment.

5. **Hooking**:
   The first few bytes of the original target function are overwritten with a jump (trampoline) to the injected VM entry point. A specific token/descriptor is passed so the VM knows which bytecode to decrypt and run.

6. **Execution (Runtime)**:
   When the protected application is launched and the hooked function is called, execution shifts to the VM. The VM initializes its context, decrypts the bytecode, and executes the equivalent logic natively, returning the result seamlessly to the original caller.
