package com.vmpacker.test;

/**
 * JNI bridge to libvmptest functions protected by VMPacker.
 *
 * Protected native functions:
 *   - vmp_compute        (pure computation)
 *   - vmp_verify_key     (pure computation)
 *   - vmp_md5_hex        (libc calls via PLT)
 *   - vmp_get_process_name (libc I/O via PLT)
 */
public class NativeTest {

    static {
        System.loadLibrary("native_test");
    }

    public static native int    nativeCompute(String input, int mode, int seed);
    public static native int    nativeVerifyKey(String key, int productId);
    public static native String nativeMd5Hex(String input);
    public static native String nativeGetProcessName();

    /** Run all tests and return a multiline result string. */
    public static native String runAllTests();
}
