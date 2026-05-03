# test/Makefile - Compile VMPacker test programs
#
# Usage:
#   make arm64        - Cross-compile ARM64 executable (requires aarch64-linux-gnu-gcc)
#   make arm32        - Cross-compile ARM32 executable (requires arm-linux-gnueabihf-gcc)
#   make protect      - Perform VMP protection on the ARM64 version (requires make arm64 first)
#   make run          - Run the protected program using qemu (requires qemu-user)
#
#   make so32         - Compile ARM32 shared library + dynamic link test program
#   make so64         - Compile ARM64 shared library + dynamic link test program
#   make so32-protect - Protect ARM32 .so (vmp_compute + vmp_verify_key)
#   make so64-protect - Protect ARM64 .so
#   make so32-run     - Run in Docker ARM32 environment (before vs. after protection)
#
# macOS cross-compilation for ARM32 optional:
#   brew install messense/macos-cross-toolchains/armv7-unknown-linux-gnueabihf
#   make arm32 CROSS_ARM32=scripts/toolchains/armv7-unknown-linux-gnueabihf/bin/armv7-unknown-linux-gnueabihf-

CROSS        ?= aarch64-linux-gnu-
CROSS_ARM32  ?= arm-linux-gnueabihf-
CC_ARM64     = $(CROSS)gcc
CC_ARM32     = $(CROSS_ARM32)gcc

BUILD_DIR = build

SRC       = simple_app.cpp
OUT_ARM64 = $(BUILD_DIR)/simple_app_arm64
OUT_ARM32 = $(BUILD_DIR)/simple_app_arm32
PROTECTED = $(BUILD_DIR)/simple_app_protected
VMPACKER  = go run ../cmd/vmpacker/

# Use -O1 to avoid excessive optimization causing function inlining (needed to locate log2Console during protection)
# Avoid link failure due to LTO plugin code signing on macOS
CFLAGS = -O1 -fno-inline-functions -fno-lto

# ARM64 SO additionally adds -mgeneral-regs-only to prevent the compiler from generating
# NEON/SIMD instructions for memset/memcpy (VMPacker does not yet support Advanced SIMD)
SO_CFLAGS_ARM64 = $(CFLAGS) -mgeneral-regs-only

# ---------- Shared library related ----------
SO_SRC        = libvmptest.c
SO_ARM32      = $(BUILD_DIR)/libvmptest_arm32.so
SO_ARM64      = $(BUILD_DIR)/libvmptest_arm64.so
SO_PROTECTED  = $(BUILD_DIR)/libvmptest_protected
APP_SO_ARM32  = $(BUILD_DIR)/simple_app_so_arm32
APP_SO_ARM64  = $(BUILD_DIR)/simple_app_so_arm64
SO_FUNCS      = vmp_compute,vmp_verify_key,vmp_md5_hex,vmp_get_process_name

.PHONY: arm64 arm32 protect protect32 run run32 clean \
        so32 so64 so32-protect so64-protect so32-run so64-run

$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

# ============================================================
# Static executables (existing)
# ============================================================

arm64: $(OUT_ARM64)
$(OUT_ARM64): $(SRC) | $(BUILD_DIR)
	$(CC_ARM64) $(CFLAGS) -o $@ $<
	@echo "[+] Built: $@"

arm32: $(OUT_ARM32)
$(OUT_ARM32): $(SRC) | $(BUILD_DIR)
	$(CC_ARM32) $(CFLAGS) -o $@ $<
	@echo "[+] Built: $@"

protect: $(OUT_ARM64)
	$(VMPACKER) -func log2Console -v -debug -o $(PROTECTED) $(OUT_ARM64)
	@echo "[+] Protected: $(PROTECTED)"

protect32: $(OUT_ARM32)
	$(VMPACKER) -func log2Console -v -debug -o $(PROTECTED)_arm32 $(OUT_ARM32)
	@echo "[+] Protected: $(PROTECTED)_arm32"

SYSROOT_ARM64 = $(shell cd $(dir $(MAKEFILE_LIST)).. && pwd)/scripts/toolchains/aarch64-unknown-linux-gnu/aarch64-unknown-linux-gnu/sysroot

run: protect
	@echo "[*] Running protected ARM64 binary with QEMU..."
	@if [ -d "$(SYSROOT_ARM64)" ]; then qemu-aarch64 -L "$(SYSROOT_ARM64)" $(PROTECTED); \
	else qemu-aarch64 -L /usr/aarch64-linux-gnu/ $(PROTECTED); fi

run-unprotected: $(OUT_ARM64)
	@if [ -d "$(SYSROOT_ARM64)" ]; then qemu-aarch64 -L "$(SYSROOT_ARM64)" $(OUT_ARM64); \
	else qemu-aarch64 -L /usr/aarch64-linux-gnu/ $(OUT_ARM64); fi

run32: protect32
	@echo "[*] Running protected ARM32 binary with QEMU..."
	qemu-arm -L /usr/arm-linux-gnueabihf/ $(PROTECTED)_arm32

# ============================================================
# Shared library tests (.so)
# ============================================================

# --- ARM32 ---
so32: $(SO_ARM32) $(APP_SO_ARM32)
$(SO_ARM32): $(SO_SRC) libvmptest.h | $(BUILD_DIR)
	$(CC_ARM32) -shared -fPIC $(CFLAGS) -o $@ $<
	@echo "[+] Built shared lib: $@"

$(APP_SO_ARM32): $(SRC) $(SO_ARM32) libvmptest.h | $(BUILD_DIR)
	$(CC_ARM32) $(CFLAGS) -DUSE_SHARED_LIB -I. -o $@ $< -L$(BUILD_DIR) -lvmptest_arm32 -Wl,-rpath,'$$ORIGIN'
	@echo "[+] Built app (dynamic): $@"

so32-protect: $(SO_ARM32)
	$(VMPACKER) -func "$(SO_FUNCS)" -v -debug -o $(SO_PROTECTED)_arm32.so $(SO_ARM32)
	@echo "[+] Protected: $(SO_PROTECTED)_arm32.so"

so32-run: so32 so32-protect
	@echo ""
	@echo "============================================"
	@echo " Docker ARM32 .so test"
	@echo "============================================"
	@echo "[*] 1/2 Running with UNPROTECTED .so ..."
	docker run --rm --platform linux/arm/v7 \
	  -v $(CURDIR)/$(BUILD_DIR):/test -w /test \
	  arm32v7/ubuntu:20.04 \
	  /bin/sh -c 'LD_LIBRARY_PATH=/test /test/simple_app_so_arm32'
	@echo ""
	@echo "[*] 2/2 Running with PROTECTED .so ..."
	docker run --rm --platform linux/arm/v7 \
	  -v $(CURDIR)/$(BUILD_DIR):/test -w /test \
	  arm32v7/ubuntu:20.04 \
	  /bin/sh -c 'mkdir -p /tmp/vmp && cp /test/libvmptest_protected_arm32.so /tmp/vmp/libvmptest_arm32.so && cp /test/simple_app_so_arm32 /tmp/vmp/ && LD_LIBRARY_PATH=/tmp/vmp /tmp/vmp/simple_app_so_arm32'

# --- ARM64 ---
so64: $(SO_ARM64) $(APP_SO_ARM64)
$(SO_ARM64): $(SO_SRC) libvmptest.h | $(BUILD_DIR)
	$(CC_ARM64) -shared -fPIC $(SO_CFLAGS_ARM64) -o $@ $<
	@echo "[+] Built shared lib: $@"

$(APP_SO_ARM64): $(SRC) $(SO_ARM64) libvmptest.h | $(BUILD_DIR)
	$(CC_ARM64) $(CFLAGS) -DUSE_SHARED_LIB -I. -o $@ $< -L$(BUILD_DIR) -lvmptest_arm64 -Wl,-rpath,'$$ORIGIN'
	@echo "[+] Built app (dynamic): $@"

so64-protect: $(SO_ARM64)
	$(VMPACKER) -func "$(SO_FUNCS)" -v -debug -o $(SO_PROTECTED)_arm64.so $(SO_ARM64)
	@echo "[+] Protected: $(SO_PROTECTED)_arm64.so"

so64-run: so64 so64-protect
	@echo ""
	@echo "============================================"
	@echo " Docker ARM64 .so test"
	@echo "============================================"
	@echo "[*] 1/2 Running with UNPROTECTED .so ..."
	docker run --rm --platform linux/arm64 \
	  -v $(CURDIR)/$(BUILD_DIR):/test -w /test \
	  ubuntu:22.04 \
	  /bin/sh -c 'LD_LIBRARY_PATH=/test /test/simple_app_so_arm64'
	@echo ""
	@echo "[*] 2/2 Running with PROTECTED .so ..."
	docker run --rm --platform linux/arm64 \
	  -v $(CURDIR)/$(BUILD_DIR):/test -w /test \
	  ubuntu:22.04 \
	  /bin/sh -c 'mkdir -p /tmp/vmp && cp /test/libvmptest_protected_arm64.so /tmp/vmp/libvmptest_arm64.so && cp /test/simple_app_so_arm64 /tmp/vmp/ && LD_LIBRARY_PATH=/tmp/vmp /tmp/vmp/simple_app_so_arm64'

# ============================================================
clean:
	rm -rf $(BUILD_DIR)
