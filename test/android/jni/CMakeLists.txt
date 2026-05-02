cmake_minimum_required(VERSION 3.18)
project(vmpacker_test C)

set(TEST_ROOT ${CMAKE_CURRENT_SOURCE_DIR}/../..)

# --- Shared library: libnative_test.so ---
add_library(native_test SHARED
    native_bridge.c
    ${TEST_ROOT}/libvmptest.c
)

target_include_directories(native_test PRIVATE
    ${TEST_ROOT}
)

target_compile_options(native_test PRIVATE
    -O1
    -fno-inline-functions
    -fno-lto
    -fno-stack-protector
    -Wall
)

# Disable NEON/SIMD that VMPacker can't translate yet
if(CMAKE_ANDROID_ARCH_ABI STREQUAL "arm64-v8a")
    target_compile_options(native_test PRIVATE -mgeneral-regs-only)
elseif(CMAKE_ANDROID_ARCH_ABI STREQUAL "armeabi-v7a")
    # -marm: generate ARM mode code (VMPacker Thumb-2 decoder has gaps, e.g. UMULL)
    # -mfpu=vfpv3-d16: VFP only, no NEON
    target_compile_options(native_test PRIVATE -marm -mfpu=vfpv3-d16)
endif()

target_link_libraries(native_test
    android
    log
)
