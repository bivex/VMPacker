# ============================================================
# VMP Toolchain Makefile (Portable: Linux/macOS/Windows)
# ============================================================

# Detect OS
ifeq ($(OS),Windows_NT)
    IS_WINDOWS := 1
else
    UNAME_S := $(shell uname -s)
    ifeq ($(UNAME_S),Darwin)
        IS_MACOS := 1
    else
        IS_LINUX := 1
    endif
endif

# Cross-compilation toolchain
ifdef ANDROID_NDK
    # Android NDK (macOS: darwin-x86_64, Linux: linux-x86_64)
    NDK_TOOLCHAIN = $(ANDROID_NDK)/toolchains/llvm/prebuilt/darwin-x86_64/bin
    CROSS   = $(NDK_TOOLCHAIN)/aarch64-linux-android
    CC      = $(CROSS)21-clang
    # NDK uses clang as linker with -nostdlib -nostartfiles for freestanding blobs
    LD      = $(CROSS)21-clang -nostdlib -nostartfiles -static
    NM      = $(NDK_TOOLCHAIN)/llvm-nm
    OBJCOPY = $(NDK_TOOLCHAIN)/llvm-objcopy
else
    CROSS   ?= aarch64-linux-gnu-
    CC       = $(CROSS)gcc
    LD       = $(CROSS)ld
    NM       = $(CROSS)nm
    OBJCOPY  = $(CROSS)objcopy
endif
GO       = go

# Directories
STUB_DIR   = stub/linux/arm64
CMD_DIR    = cmd/vmpacker
DEMO_DIR   = demo
BUILD_DIR  = build

# ------ VM Interpreter blob ------
STUB_SRC   = $(STUB_DIR)/vm_interp.c
STUB_ASM   = $(STUB_DIR)/vm_entry.S
STUB_LDS   = $(STUB_DIR)/vm_interp.lds
STUB_O     = $(BUILD_DIR)/stub/vm_interp.o
STUB_O_ASM = $(BUILD_DIR)/stub/vm_entry.o
STUB_ELF   = $(BUILD_DIR)/stub/vm_interp.elf
STUB_BIN   = $(CMD_DIR)/vm_interp.bin

# ------ VM Interpreter x86_64 blob ------
STUB64_DIR   = stub/linux/x86_64
STUB64_SRC   = $(STUB64_DIR)/vm_interp.c
STUB64_ASM   = $(STUB64_DIR)/vm_entry.S
STUB64_LDS   = $(STUB64_DIR)/vm_interp.lds
STUB64_O     = $(BUILD_DIR)/stub64/vm_interp.o
STUB64_O_ASM = $(BUILD_DIR)/stub64/vm_entry.o
STUB64_ELF   = $(BUILD_DIR)/stub64/vm_interp.elf
STUB64_BIN   = $(CMD_DIR)/vm_interp_x86_64.bin

# ------ Go packer ------
ifdef IS_WINDOWS
    PACKER = $(BUILD_DIR)/vmpacker.exe
else
    PACKER = $(BUILD_DIR)/vmpacker
endif

# Compilation options
STUB_CFLAGS = -g -c -O1 -mcmodel=tiny -fno-stack-protector \
              -fno-builtin -fno-builtin-memcpy -nostdlib -march=armv8-a \
              -DVM_FUNC_SPLIT -DVM_TOKEN_ENTRY -DVM_INDIRECT_DISPATCH

STUB64_CFLAGS = -g -c -O1 -fno-stack-protector \
              -fno-builtin -fno-builtin-memcpy -nostdlib -fPIE -fvisibility=hidden -mcmodel=small -fno-jump-tables \
              -DVM_FUNC_SPLIT -DVM_TOKEN_ENTRY -DVM_INDIRECT_DISPATCH

DEMO_CFLAGS = -static -O0 -march=armv8-a

# ============================================================
.PHONY: all stub stub64 packer demo test clean help

all: stub stub64 packer
	@echo "[+] Build complete: $(BUILD_DIR)/"

# ------ VM Interpreter blob ------
stub: $(STUB_BIN)

stub64: $(STUB64_BIN)

$(BUILD_DIR) $(BUILD_DIR)/stub $(BUILD_DIR)/stub64:
ifdef IS_WINDOWS
	@powershell -Command "New-Item -ItemType Directory -Force -Path '$@' | Out-Null"
else
	@mkdir -p $@
endif

$(STUB_O): $(STUB_SRC) | $(BUILD_DIR)/stub
	$(CC) $(STUB_CFLAGS) $< -o $@

$(STUB_O_ASM): $(STUB_ASM) | $(BUILD_DIR)/stub
	$(CC) $(STUB_CFLAGS) $< -o $@

$(STUB_ELF): $(STUB_O) $(STUB_O_ASM) $(STUB_LDS)
	$(LD) -T $(STUB_LDS) -o $@ $(STUB_O) $(STUB_O_ASM)

$(STUB_BIN): $(STUB_ELF) | $(BUILD_DIR)
	$(OBJCOPY) -O binary $< $(BUILD_DIR)/vm_interp_raw.bin
ifdef IS_WINDOWS
	@powershell -Command "\
		$$nmOut = & '$(NM)' '$<';\
		$$l1 = $$nmOut | Select-String '\bvm_entry$$';\
		$$l2 = $$nmOut | Select-String '\bvm_entry_token$$';\
		$$l3 = $$nmOut | Select-String '\b_token_table_va$$';\
		if (!$$l1) { Write-Error 'vm_entry not found'; exit 1 };\
		if (!$$l2) { Write-Error 'vm_entry_token not found'; exit 1 };\
		if (!$$l3) { Write-Error '_token_table_va not found'; exit 1 };\
		$$off1 = [Convert]::ToUInt64($$l1.ToString().Split(' ')[0], 16);\
		$$off2 = [Convert]::ToUInt64($$l2.ToString().Split(' ')[0], 16);\
		$$off3 = [Convert]::ToUInt64($$l3.ToString().Split(' ')[0], 16);\
		$$hdr = [BitConverter]::GetBytes([UInt64]$$off1) + [BitConverter]::GetBytes([UInt64]$$off2) + [BitConverter]::GetBytes([UInt64]$$off3);\
		$$raw = [IO.File]::ReadAllBytes('$(BUILD_DIR)/vm_interp_raw.bin');\
		$$blob = $$hdr + $$raw;\
		[IO.File]::WriteAllBytes('$(STUB_BIN)', $$blob);\
		Write-Host ('[+] vm_interp.bin: ' + $$blob.Length + ' bytes (vm_entry=0x' + $$off1.ToString('X') + ' vm_entry_token=0x' + $$off2.ToString('X') + ' _token_table_va=0x' + $$off3.ToString('X') + ')')\
	"
	@copy /Y "$(subst /,\,$(STUB_BIN))" "$(subst /,\,$(BUILD_DIR))\vm_interp.bin" > nul
else
	@OFF1=$$($(NM) $< | grep "\bvm_entry$$" | cut -d' ' -f1); \
	OFF2=$$($(NM) $< | grep "\bvm_entry_token$$" | cut -d' ' -f1); \
	OFF3=$$($(NM) $< | grep "\b_token_table_va$$" | cut -d' ' -f1); \
	if [ -z "$$OFF1" ] || [ -z "$$OFF2" ] || [ -z "$$OFF3" ]; then \
		echo "Error: Symbols not found"; exit 1; \
	fi; \
	env -i PATH=/usr/bin:/bin /usr/bin/python3 -c "import struct; h = struct.pack('<QQQ', int('$$OFF1', 16), int('$$OFF2', 16), int('$$OFF3', 16)); r = open('$(BUILD_DIR)/vm_interp_raw.bin', 'rb').read(); open('$(STUB_BIN)', 'wb').write(h + r)"
	@cp $(STUB_BIN) $(BUILD_DIR)/vm_interp.bin
	@echo "[+] vm_interp.bin created"
endif

# x86_64 stub build
CC64 = gcc
LD64 = ld
NM64 = nm
OBJCOPY64 = objcopy

$(STUB64_O): $(STUB64_SRC) | $(BUILD_DIR)/stub64
	$(CC64) $(STUB64_CFLAGS) $< -o $@

$(STUB64_O_ASM): $(STUB64_ASM) | $(BUILD_DIR)/stub64
	$(CC64) $(STUB64_CFLAGS) $< -o $@

$(STUB64_ELF): $(STUB64_O) $(STUB64_O_ASM) $(STUB64_LDS)
	$(LD64) -T $(STUB64_LDS) -o $@ $(STUB64_O) $(STUB64_O_ASM)

$(STUB64_BIN): $(STUB64_ELF) | $(BUILD_DIR)
	$(OBJCOPY64) -O binary $< $(BUILD_DIR)/vm_interp_x86_64_raw.bin
	@OFF1=$$($(NM64) $< | grep "\bvm_entry$$" | cut -d' ' -f1); \
	OFF2=$$($(NM64) $< | grep "\bvm_entry_token$$" | cut -d' ' -f1); \
	OFF3=$$($(NM64) $< | grep "\b_token_table_va$$" | cut -d' ' -f1); \
	if [ -z "$$OFF1" ] || [ -z "$$OFF2" ] || [ -z "$$OFF3" ]; then \
		echo "Error: Symbols not found for x86_64 stub"; exit 1; \
	fi; \
	python3 -c "import struct; h = struct.pack('<QQQ', int('$$OFF1', 16), int('$$OFF2', 16), int('$$OFF3', 16)); r = open('$(BUILD_DIR)/vm_interp_x86_64_raw.bin', 'rb').read(); open('$(STUB64_BIN)', 'wb').write(h + r)"
	@cp $(STUB64_BIN) $(BUILD_DIR)/vm_interp_x86_64.bin
	@echo "[+] vm_interp_x86_64.bin created"

# ------ Go packer ------
packer: $(STUB_BIN) $(STUB64_BIN) | $(BUILD_DIR)
	$(GO) build -buildvcs=false -o $(PACKER) ./$(CMD_DIR)/
	@echo "[+] packer: $(PACKER)"

# ------ Demo ------
demo: $(BUILD_DIR)/demo_simple $(BUILD_DIR)/demo_x86_64 $(BUILD_DIR)/demo_hybrid_license $(BUILD_DIR)/demo_hybrid_license_x86_64

$(BUILD_DIR)/demo_x86_64: $(DEMO_DIR)/demo_x86_64.c | $(BUILD_DIR)
	$(CC64) -O1 $< -o $@
	@echo "[+] x86_64 demo: $@"

$(BUILD_DIR)/demo_simple: $(DEMO_DIR)/demo_simple.c | $(BUILD_DIR)
	$(CC) -static -O1 -nostdlib -march=armv8-a $< -o $@
	@echo "[+] ARM64 demo: $@"

$(BUILD_DIR)/demo_hybrid_license_x86_64: $(DEMO_DIR)/demo_hybrid_license.c | $(BUILD_DIR)
	$(CC64) -O1 $< -o $@
	@echo "[+] x86_64 hybrid demo: $@"

$(BUILD_DIR)/demo_hybrid_license: $(DEMO_DIR)/demo_hybrid_license.c | $(BUILD_DIR)
	$(CC) -static -O1 -nostdlib -march=armv8-a $< -o $@
	@echo "[+] ARM64 hybrid demo: $@"

# ------ GUI (Wails) ------
GUI_DIR = vmp-gui

gui: stub
ifdef IS_WINDOWS
	@copy /Y "$(subst /,\,$(STUB_BIN))" "$(subst /,\,$(GUI_DIR))\backend\api\vm_interp.bin" > nul
	@cd $(GUI_DIR) && wails build
else
	@cp $(STUB_BIN) $(GUI_DIR)/backend/api/vm_interp.bin
	@cd $(GUI_DIR) && wails build
endif
	@echo "[+] GUI build complete"

# ------ Clean ------
clean:
ifdef IS_WINDOWS
	@powershell -Command "Remove-Item -Recurse -Force -ErrorAction SilentlyContinue '$(BUILD_DIR)', '$(STUB_BIN)'"
else
	@rm -rf $(BUILD_DIR) $(STUB_BIN)
endif
	@echo "[+] cleaned"

# ------ Hybrid Mode Demo Test ------
# Test x86_64 hybrid demo (native host execution)
test-hybrid-x86_64: $(BUILD_DIR)/demo_hybrid_license_x86_64
	@echo "=== Testing Hybrid Mode on x86_64 ==="
	./vmpacker -func verify_license_key -hybrid -v -o $(BUILD_DIR)/demo_hybrid_x86_64.vmp $<
	@echo ""
	@echo "--- Running protected binary ---"
	@$(BUILD_DIR)/demo_hybrid_license_x86_64
	@echo ""

# Test ARM64 hybrid demo (requires qemu-aarch64 or real ARM64)
test-hybrid-arm64: $(BUILD_DIR)/demo_hybrid_license
	@echo "=== Testing Hybrid Mode on ARM64 ==="
	@echo "Note: Requires aarch64-linux-gnu-ld (cross-toolchain) and QEMU or real ARM64"
	./vmpacker -func verify_license_key -hybrid -v -o $(BUILD_DIR)/demo_hybrid_arm64.vmp $<
	@echo ""
	@echo "--- Running on QEMU (if available) ---"
	@qemu-aarch64 -L /usr/aarch64-linux-gnu/ $(BUILD_DIR)/demo_hybrid_license 2>/dev/null || \
		echo "QEMU not available, binary built but not executed"
	@echo ""

help:
	@echo "make all    - Build stub + packer"
	@echo "make stub   - Build VM interpreter blob"
	@echo "make packer - Build Go packer"
	@echo "make demo   - Build demo programs"
	@echo "make test-hybrid-x86_64 - Test Hybrid Mode on x86_64 (native)"
	@echo "make test-hybrid-arm64  - Test Hybrid Mode on ARM64 (via QEMU)"
	@echo "make clean  - Clean build artifacts"
