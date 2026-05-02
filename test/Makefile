# test/Makefile - 编译 VMPacker 测试程序
#
# 用法:
#   make arm64        - 交叉编译 ARM64 可执行文件 (需 aarch64-linux-gnu-gcc)
#   make arm32        - 交叉编译 ARM32 可执行文件 (需 arm-linux-gnueabihf-gcc)
#   make protect      - 对 ARM64 版本执行 VMP 保护 (需先 make arm64)
#   make run          - 使用 qemu 运行保护后的程序 (需 qemu-user)
#
#   make so32         - 编译 ARM32 共享库 + 动态链接测试程序
#   make so64         - 编译 ARM64 共享库 + 动态链接测试程序
#   make so32-protect - 保护 ARM32 .so (vmp_compute + vmp_verify_key)
#   make so64-protect - 保护 ARM64 .so
#   make so32-run     - Docker ARM32 环境中运行 (保护前 vs 保护后)
#
# macOS 交叉编译 ARM32 可选:
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

# 使用 -O1 避免过度优化导致函数被内联（保护时需要能定位到 log2Console）
# -fno-lto 避免 macOS 上 LTO 插件代码签名导致链接失败
CFLAGS = -O1 -fno-inline-functions -fno-lto

# ARM64 SO 额外加 -mgeneral-regs-only 避免编译器对 memset/memcpy 生成
# NEON/SIMD 指令 (VMPacker 尚不支持 Advanced SIMD)
SO_CFLAGS_ARM64 = $(CFLAGS) -mgeneral-regs-only

# ---------- 共享库相关 ----------
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
# 静态可执行文件 (已有)
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
# 共享库测试 (.so)
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
