# test/android/Makefile — Build & test VMPacker protection on Android .so
#
# Two build modes:
#   1) Standalone NDK (no Gradle): compiles .so + test_runner, push via adb
#   2) Gradle APK: full Android app with JNI
#
# Usage (standalone):
#   make so32             - Build ARM32 .so with NDK
#   make so64             - Build ARM64 .so with NDK
#   make runner32         - Build ARM32 test runner (for adb shell)
#   make runner64         - Build ARM64 test runner (for adb shell)
#   make protect32        - Protect ARM32 .so with VMPacker
#   make protect64        - Protect ARM64 .so with VMPacker
#   make push32           - Push ARM32 files to device
#   make push64           - Push ARM64 files to device
#   make test32           - Full ARM32 pipeline: build, protect, push, run
#   make test64           - Full ARM64 pipeline: build, protect, push, run
#   make test-all         - Run both ARM32 and ARM64 tests
#
# Usage (Gradle):
#   make apk             - Build debug APK with Gradle
#   make apk-protect     - Protect .so inside APK and re-sign
#   make apk-install     - Install protected APK on device
#
# Environment:
#   NDK_HOME or ANDROID_NDK_HOME  - Path to Android NDK (required)
#   API                           - Android API level (default: 21)

# ---- NDK detection ----
NDK      ?= $(or $(NDK_HOME),$(ANDROID_NDK_HOME),$(ANDROID_NDK))
API      ?= 21

ifeq ($(NDK),)
$(warning $$NDK_HOME / $$ANDROID_NDK_HOME not set — standalone builds will fail.)
$(warning Set it: export NDK_HOME=/path/to/android-ndk-rXX)
endif

UNAME   := $(shell uname -s)
ifeq ($(UNAME),Darwin)
  HOST_TAG = darwin-x86_64
else
  HOST_TAG = linux-x86_64
endif

NDK_BIN    = $(NDK)/toolchains/llvm/prebuilt/$(HOST_TAG)/bin
CC_ARM32   = $(NDK_BIN)/armv7a-linux-androideabi$(API)-clang
CC_ARM64   = $(NDK_BIN)/aarch64-linux-android$(API)-clang
STRIP32    = $(NDK_BIN)/llvm-strip
STRIP64    = $(NDK_BIN)/llvm-strip

# ---- Project paths ----
VMPACKER       = go run ../../cmd/vmpacker/
BUILD_DIR      = build
TEST_ROOT      = ../..
LIBVMPTEST_SRC = $(TEST_ROOT)/test/libvmptest.c
LIBVMPTEST_HDR = $(TEST_ROOT)/test/libvmptest.h

# Compiler flags: -O1 avoids excessive inlining; -fPIC for shared lib
# -fno-stack-protector: avoid MRS TPIDR_EL0 canary that VM can't handle on Android
CFLAGS     = -O1 -fno-inline-functions -fno-lto -fno-stack-protector -fPIC -Wall
# ARM32: -marm forces ARM mode; -mfpu=vfpv3-d16 selects VFP without NEON
CFLAGS_32  = $(CFLAGS) -marm -mfpu=vfpv3-d16
# ARM64: -mgeneral-regs-only prevents Advanced SIMD/FP
CFLAGS_64  = $(CFLAGS) -mgeneral-regs-only

# Functions to protect
SO_FUNCS = vmp_compute,vmp_verify_key,vmp_md5_hex,vmp_get_process_name

# ---- Output files ----
SO_ARM32           = $(BUILD_DIR)/libnative_test_arm32.so
SO_ARM64           = $(BUILD_DIR)/libnative_test_arm64.so
SO_PROTECTED_ARM32 = $(BUILD_DIR)/libnative_test_protected_arm32.so
SO_PROTECTED_ARM64 = $(BUILD_DIR)/libnative_test_protected_arm64.so
RUNNER_ARM32       = $(BUILD_DIR)/test_runner_arm32
RUNNER_ARM64       = $(BUILD_DIR)/test_runner_arm64

# adb target directory
ADB_DIR = /data/local/tmp/vmptest
# Optional: target a specific device (e.g. ADB_SERIAL=33af1a2b)
ADB     = adb $(if $(ADB_SERIAL),-s $(ADB_SERIAL),)

.PHONY: all clean so32 so64 runner32 runner64 protect32 protect64 \
        push32 push64 test32 test64 test-all \
        apk apk-protect apk-install help

help:
	@echo "VMPacker Android JNI Test"
	@echo ""
	@echo "Standalone NDK targets:"
	@echo "  make so32 / so64         Build .so with NDK"
	@echo "  make runner32 / runner64 Build test runner for adb shell"
	@echo "  make protect32/protect64 Protect .so with VMPacker"
	@echo "  make test32 / test64     Full pipeline: build → protect → push → run"
	@echo "  make test-all            Run both ARM32 and ARM64"
	@echo ""
	@echo "Gradle targets:"
	@echo "  make apk                 Build debug APK"
	@echo "  make apk-protect         Protect .so in APK"
	@echo "  make apk-install         Install protected APK"
	@echo ""
	@echo "Required: NDK_HOME=/path/to/ndk  (or ANDROID_NDK_HOME)"

all: so32 so64 runner32 runner64

$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

# ==================================================================
# Shared library builds
# ==================================================================

so32: $(SO_ARM32)
$(SO_ARM32): jni/native_bridge.c $(LIBVMPTEST_SRC) $(LIBVMPTEST_HDR) | $(BUILD_DIR)
	$(CC_ARM32) -shared $(CFLAGS_32) \
		-I$(TEST_ROOT)/test \
		-o $@ jni/native_bridge.c $(LIBVMPTEST_SRC) \
		-llog -landroid
	@echo "[+] Built ARM32 .so: $@"

so64: $(SO_ARM64)
$(SO_ARM64): jni/native_bridge.c $(LIBVMPTEST_SRC) $(LIBVMPTEST_HDR) | $(BUILD_DIR)
	$(CC_ARM64) -shared $(CFLAGS_64) \
		-I$(TEST_ROOT)/test \
		-o $@ jni/native_bridge.c $(LIBVMPTEST_SRC) \
		-llog -landroid
	@echo "[+] Built ARM64 .so: $@"

# ==================================================================
# Standalone test runner (for adb shell)
# ==================================================================

runner32: $(RUNNER_ARM32)
$(RUNNER_ARM32): jni/test_runner.c | $(BUILD_DIR)
	$(CC_ARM32) -O1 -pie -o $@ $< -ldl
	@echo "[+] Built ARM32 test runner: $@"

runner64: $(RUNNER_ARM64)
$(RUNNER_ARM64): jni/test_runner.c | $(BUILD_DIR)
	$(CC_ARM64) -O1 -pie -o $@ $< -ldl
	@echo "[+] Built ARM64 test runner: $@"

# ==================================================================
# VMPacker protection
# ==================================================================

protect32: $(SO_ARM32)
	$(VMPACKER) -func "$(SO_FUNCS)" -v -debug \
		-o $(SO_PROTECTED_ARM32) $(SO_ARM32)
	@echo "[+] Protected ARM32: $(SO_PROTECTED_ARM32)"

protect64: $(SO_ARM64)
	$(VMPACKER) -func "$(SO_FUNCS)" -v -debug \
		-o $(SO_PROTECTED_ARM64) $(SO_ARM64)
	@echo "[+] Protected ARM64: $(SO_PROTECTED_ARM64)"

# ==================================================================
# ADB push & run
# ==================================================================

push32: $(RUNNER_ARM32) $(SO_ARM32) $(SO_PROTECTED_ARM32)
	$(ADB) shell "mkdir -p $(ADB_DIR)"
	$(ADB) push $(RUNNER_ARM32) $(ADB_DIR)/test_runner
	$(ADB) push $(SO_ARM32)     $(ADB_DIR)/libnative_test.so
	$(ADB) push $(SO_PROTECTED_ARM32) $(ADB_DIR)/libnative_test_protected.so
	$(ADB) shell "chmod +x $(ADB_DIR)/test_runner"
	@echo "[+] Pushed ARM32 files to $(ADB_DIR)"

push64: $(RUNNER_ARM64) $(SO_ARM64) $(SO_PROTECTED_ARM64)
	$(ADB) shell "mkdir -p $(ADB_DIR)"
	$(ADB) push $(RUNNER_ARM64) $(ADB_DIR)/test_runner
	$(ADB) push $(SO_ARM64)     $(ADB_DIR)/libnative_test.so
	$(ADB) push $(SO_PROTECTED_ARM64) $(ADB_DIR)/libnative_test_protected.so
	$(ADB) shell "chmod +x $(ADB_DIR)/test_runner"
	@echo "[+] Pushed ARM64 files to $(ADB_DIR)"

run-on-device:
	@echo ""
	@echo "============================================"
	@echo " Testing UNPROTECTED .so"
	@echo "============================================"
	$(ADB) shell "cd $(ADB_DIR) && LD_LIBRARY_PATH=. ./test_runner ./libnative_test.so"
	@echo ""
	@echo "============================================"
	@echo " Testing PROTECTED .so"
	@echo "============================================"
	$(ADB) shell "cd $(ADB_DIR) && LD_LIBRARY_PATH=. ./test_runner ./libnative_test_protected.so"

# Full pipelines
test32: so32 runner32 protect32 push32 run-on-device
	@echo ""
	@echo "[+] ARM32 Android test completed."

test64: so64 runner64 protect64 push64 run-on-device
	@echo ""
	@echo "[+] ARM64 Android test completed."

test-all: test32 test64

# ==================================================================
# Gradle APK build
# ==================================================================

apk:
	./gradlew assembleDebug
	@echo "[+] APK built: app/build/outputs/apk/debug/app-debug.apk"

apk-protect: apk
	@bash scripts/protect_and_repack.sh
	@echo "[+] Protected APK: $(BUILD_DIR)/app-protected.apk"

apk-install: apk-protect
	adb install -r $(BUILD_DIR)/app-protected.apk
	@echo "[+] Installed protected APK"
	adb shell am start -n com.vmpacker.test/.MainActivity

# ==================================================================
# Clean
# ==================================================================

clean:
	rm -rf $(BUILD_DIR)
	rm -rf app/build app/.cxx .gradle
	@echo "[+] Cleaned."
