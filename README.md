<p align="center">
  <img src="docs/1.png" width="120" alt="VMPacker">
  <h1 align="center">VMPacker</h1>
  <p align="center"><strong>ARM64/ARM32 ELF Virtual Machine Protection</strong></p>
  <p align="center">
    Translate native ARM instructions into custom VM bytecode — function-level code obfuscation for ELF binaries
  </p>
  <p align="center">
    <a href="#quick-start">Quick Start</a> •
    <a href="#features">Features</a> •
    <a href="#usage">Usage</a> •
    <a href="#architecture">Architecture</a> •
    <a href="docs/ISA.md">VM ISA</a> •
    <a href="README_CN.md">中文文档</a>
  </p>
</p>

---

## Quick Start

```bash
git clone https://github.com/LeoChen-CoreMind/VMPacker.git
cd VMPacker
make all
```

Protect a function:

```bash
./vmpacker -func check_license -v -o protected.so original.so
```

Run the demo with Docker (works on macOS/Windows/Linux):

```bash
cd demo && make -f Makefile-demo all protect run-docker
```

## Features

**Instruction Coverage** — 121 ARM64 instructions, 63 VM opcodes. Full base A64 ISA: ALU, multiply-extend, data movement, memory (LDR/STR/LDP/STP/LDPSW/LDADD/CAS), branch, conditional select, bitfield, SIMD LD1/ST1, SVC/MRS, barriers. ARM32 Thumb + ARM mode also supported.

**Multi-Layer Protection**

| Layer | What |
|-------|------|
| Custom VM ISA | Randomly mapped opcodes — no standard disassembler can decode |
| OpcodeCryptor | Per-instruction XOR encryption with position-dependent key |
| Bytecode Reversal | Instructions stored in reverse execution order |
| Token Entry | Original function replaced with 3-instruction trampoline |
| RTLR | Runtime relocation table for PIE/ASLR address fixup |
| Symbol Strip | Optional removal of .symtab/.strtab sections |

**GUI** — Cross-platform desktop app (Wails v2 + Vue 3 + Element Plus) with function selection, address-range input, and real-time log output.

<table>
  <tr>
    <td><img src="docs/1.png" width="400"></td>
    <td><img src="docs/2.png" width="400"></td>
    <td><img src="docs/3.png" width="400"></td>
  </tr>
</table>

## Usage

```bash
# Protect by function name (comma-separated)
./vmpacker -func "check_license,verify_token" -v -o out.so in.so

# Protect by address range
./vmpacker -addr "0x4006AC-0x400790:my_func" -o out.so in.so

# Mixed mode
./vmpacker -addr "0x4006AC-0x400790:sub_1" -func verify -o out.so in.so

# Print ELF info
./vmpacker -info input.so
```

| Flag | Description |
|------|-------------|
| `-func` | Function name(s) to protect |
| `-addr` | Address range `0xSTART-0xEND[:name]` |
| `-o` | Output path (default: `input.vmp`) |
| `-v` | Verbose — show disassembly |
| `-strip` | Strip symbol table (default: true) |
| `-debug` | Generate ARM→VM debug mapping |
| `-info` | Print ELF info and exit |

## Architecture

```
ARM64 Native ──► Decode ──► Translate ──► VM Bytecode
                                                 │
                 ELF ◄── Inject ◄── Encrypt ◄────┘
```

```
VMPacker/
├── cmd/vmpacker/         CLI entry point
├── pkg/
│   ├── arch/arm64/        ARM64 decoder + stack-based translator
│   ├── arch/arm32/        ARM32 decoder + translator (Thumb + ARM)
│   ├── vm/                VM ISA definitions + disassembler
│   └── binary/elf/        ELF injection (PT_NOTE hijack, trampoline)
├── stub/linux/arm64/      C VM interpreter (compiled to PIC binary)
├── vmp-gui/               Wails v2 GUI (Go + Vue 3)
└── demo/                  Sample target + Makefile
```

The stack-based translator eliminates register conflicts entirely — all ARM64 instructions become `VLOAD → PUSH → S_OP → VSTORE` sequences, removing the need for temporary register allocation.

The interpreter stub is a self-contained C program compiled to a flat binary (~15KB). It's injected into the target ELF by hijacking the `PT_NOTE` program header into a `PT_LOAD` segment.

### Protection Pipeline

```
Input ELF → Locate Function → Decode ARM → Translate to VM Bytecode
    → Reverse Bytecode → Encrypt Opcodes → XOR Chain
    → Inject VM Interpreter + Trampoline → Strip Symbols → Output ELF
```

## Android (NDK)

```bash
ANDROID_NDK=/path/to/ndk make all

# Push to device
adb push build/libnative_test_protected_arm64.so /data/local/tmp/
```

See [docs/Android_Test.md](docs/Android_Test.md) for the full Android testing guide.

## Building

| Component | Command |
|-----------|---------|
| All | `make all` |
| Stub only | `make stub` |
| Packer only | `make packer` |
| GUI | `cd vmp-gui && make gui` |
| Clean | `make clean` |

Requirements: Go 1.21+, GCC cross-compiler (aarch64-linux-gnu-gcc) or Android NDK.

## Documentation

| Document | Description |
|----------|-------------|
| [VM ISA Reference](docs/ISA.md) | Full opcode table and encoding details |
| [Architecture Notes](docs/notes/) | Design decisions, task history |
| [Android Testing](docs/Android_Test.md) | NDK build + emulator testing |
| [ARM32 Build](docs/BUILD_ARM32.md) | ARM32 cross-compilation guide |
| [中文文档](README_CN.md) | Full Chinese documentation |

## Roadmap

- [ ] Dynamic opcode mapping — unique ISA per protection run
- [ ] Hybrid mode — partial native + partial VM execution
- [ ] x86_64 architecture support

## License

[AGPL-3.0](LICENSE) — strong copyleft. Network use requires source disclosure. Commercial licensing available on request.

## Author

**LeoChen** — [@LeoChen-CoreMind](https://github.com/LeoChen-CoreMind)

Copyright © 2026 LeoChen. All rights reserved.
