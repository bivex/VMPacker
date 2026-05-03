# VMPacker Android JNI Test

验证 VMPacker 对 Android `.so` 文件（ARM32 + ARM64）的保护效果。

## 项目结构

```
test/android/
├── Makefile                          # 独立 NDK 编译 + VMPacker 保护
├── README.md
├── jni/
│   ├── CMakeLists.txt                # CMake 构建（供 Gradle 和独立编译共用）
│   ├── native_bridge.c               # JNI 桥接：Java ↔ libvmptest
│   └── test_runner.c                 # adb shell 独立测试程序（dlopen 方式）
├── app/
│   ├── build.gradle                  # Android 模块构建
│   └── src/main/
│       ├── AndroidManifest.xml
│       ├── java/com/vmpacker/test/
│       │   ├── MainActivity.java     # 测试界面
│       │   └── NativeTest.java       # JNI 声明
│       └── res/                      # 布局和资源
├── build.gradle                      # 项目级 Gradle
├── settings.gradle
├── gradle.properties
└── scripts/
    └── protect_and_repack.sh         # protect .so in APK + re-sign
```

## 前置要求

- **Android NDK** (r21+)，设置环境变量：
  ```bash
  export NDK_HOME=/path/to/android-ndk-rXX
  ```
- **Go 1.21+**（用于运行 VMPacker）
- **adb**（用于设备测试）
- **Android SDK + Gradle**（仅 APK 构建需要）

## 方式一：独立 NDK 编译 + adb 测试（推荐）

无需 Gradle/Android SDK，直接用 NDK 编译 `.so` 并在设备上测试。

```bash
cd test/android

# 编译 ARM64 .so + 测试程序
make so64 runner64

# VMPacker 保护
make protect64

# 推送到设备并运行
make push64
make run-on-device

# 一键完成所有步骤
make test64

# 同时测试 ARM32 和 ARM64
make test-all
```

### 测试流程

1. `make so32/so64` — 使用 NDK clang 编译 `libnative_test.so`
2. `make protect32/protect64` — VMPacker protects target functions in `.so`
3. `make push32/push64` — 通过 adb 推送到设备 `/data/local/tmp/vmptest/`
4. `make run-on-device` — 分别用未保护和保护后的 `.so` 运行测试
5. 对比输出：保护前后结果应完全一致

### 被保护的函数

| 函数 | 类型 | 说明 |
|------|------|------|
| `vmp_compute` | 纯计算 | 哈希 + 算术 + 位运算 |
| `vmp_verify_key` | 纯计算 | 密钥校验逻辑 |
| `vmp_md5_hex` | PLT 调用 | MD5 计算，调用 `strlen`/`memcpy`/`memset`/`snprintf` |
| `vmp_get_process_name` | PLT 调用 | 读取 `/proc/self/comm`，调用 `open`/`read`/`close` |

## 方式二：Gradle APK 构建

构建完整的 Android 应用，在设备上运行 UI 测试。

```bash
cd test/android

# 构建 debug APK
make apk

# Protect .so in APK and re-sign
make apk-protect

# 安装到设备
make apk-install
```

APK 构建需要额外配置 `local.properties`：
```properties
sdk.dir=/path/to/Android/sdk
ndk.dir=/path/to/android-ndk-rXX
```

## 预期输出

```
=== VMPacker Android SO Test Runner ===
[*] Loading: ./libnative_test.so

--- vmp_compute ---
  compute("hello",0,42) = XXXXX
  [PASS] compute returns non-negative for valid input
  [PASS] compute returns -1 for NULL
  [PASS] different mode gives different result
  [PASS] different input gives different result

--- vmp_md5_hex ---
  md5("hello")     = 5d41402abc4b2a76b9719d911017c592  (rc=0)
  [PASS] md5("hello") matches
  ...

========================================
  PASS: XX    FAIL: 0    TOTAL: XX
========================================
```

保护后的 `.so` 应产生完全相同的输出。

## 注意事项

- ARM64 编译使用 `-mgeneral-regs-only` 避免 NEON/SIMD 指令（VMPacker 暂不支持）
- 测试程序使用 `dlopen` 加载 `.so`，与 Android 运行时行为一致
- `/proc/self/comm` 在 Android 上返回进程名（即包名或可执行文件名）
