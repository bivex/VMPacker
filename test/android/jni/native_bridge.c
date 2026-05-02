/**
 * native_bridge.c — JNI bridge for VMPacker Android .so protection test
 *
 * Wraps libvmptest functions with JNI-compatible signatures.
 * The underlying implementation functions (vmp_compute, vmp_verify_key,
 * vmp_md5_hex, vmp_get_process_name) are the VMPacker protection targets.
 *
 * Build:
 *   NDK clang: see test/android/Makefile
 *   Gradle:    see test/android/app/build.gradle + jni/CMakeLists.txt
 */

#include <jni.h>
#include <stdio.h>
#include <string.h>
#include <android/log.h>

#include "libvmptest.h"

#define TAG "VMPacker"
#define LOGI(...) __android_log_print(ANDROID_LOG_INFO, TAG, __VA_ARGS__)

JNIEXPORT jint JNICALL
Java_com_vmpacker_test_NativeTest_nativeCompute(
    JNIEnv *env, jclass clazz, jstring input, jint mode, jint seed)
{
    if (!input)
        return vmp_compute(NULL, mode, seed);

    const char *str = (*env)->GetStringUTFChars(env, input, NULL);
    jint result = vmp_compute(str, mode, seed);
    LOGI("vmp_compute(\"%s\", %d, %d) = %d", str, mode, seed, result);
    (*env)->ReleaseStringUTFChars(env, input, str);
    return result;
}

JNIEXPORT jint JNICALL
Java_com_vmpacker_test_NativeTest_nativeVerifyKey(
    JNIEnv *env, jclass clazz, jstring key, jint productId)
{
    if (!key)
        return vmp_verify_key(NULL, productId);

    const char *str = (*env)->GetStringUTFChars(env, key, NULL);
    jint result = vmp_verify_key(str, productId);
    LOGI("vmp_verify_key(\"%s\", %d) = %d", str, productId, result);
    (*env)->ReleaseStringUTFChars(env, key, str);
    return result;
}

JNIEXPORT jstring JNICALL
Java_com_vmpacker_test_NativeTest_nativeMd5Hex(
    JNIEnv *env, jclass clazz, jstring input)
{
    if (!input)
        return NULL;

    const char *str = (*env)->GetStringUTFChars(env, input, NULL);
    char hex[64];
    int rc = vmp_md5_hex(str, hex, sizeof(hex));
    LOGI("vmp_md5_hex(\"%s\") = %s (rc=%d)", str, hex, rc);
    (*env)->ReleaseStringUTFChars(env, input, str);

    if (rc != 0)
        return NULL;
    return (*env)->NewStringUTF(env, hex);
}

JNIEXPORT jstring JNICALL
Java_com_vmpacker_test_NativeTest_nativeGetProcessName(
    JNIEnv *env, jclass clazz)
{
    char name[256];
    int len = vmp_get_process_name(name, sizeof(name));
    if (len <= 0)
        return (*env)->NewStringUTF(env, "(unknown)");

    LOGI("vmp_get_process_name() = %s", name);
    return (*env)->NewStringUTF(env, name);
}

JNIEXPORT jstring JNICALL
Java_com_vmpacker_test_NativeTest_runAllTests(
    JNIEnv *env, jclass clazz)
{
    char buf[2048];
    int off = 0;

#define APPEND(...) off += snprintf(buf + off, sizeof(buf) - off, __VA_ARGS__)

    APPEND("=== VMPacker Android JNI Test ===\n\n");

    APPEND("--- Pure computation ---\n");
    APPEND("vmp_compute(\"hello\",0,42) = %d\n", vmp_compute("hello", 0, 42));
    APPEND("vmp_compute(\"hello\",1,42) = %d\n", vmp_compute("hello", 1, 42));
    APPEND("vmp_compute(\"hello\",2,42) = %d\n", vmp_compute("hello", 2, 42));
    APPEND("vmp_compute(\"world\",0,42) = %d\n", vmp_compute("world", 0, 42));
    APPEND("vmp_compute(NULL,0,0)      = %d\n", vmp_compute(NULL, 0, 0));
    APPEND("\n");

    APPEND("vmp_verify_key(\"ABCD-1234-EFGH\",100) = %d\n",
           vmp_verify_key("ABCD-1234-EFGH", 100));
    APPEND("vmp_verify_key(\"short\",100)           = %d\n",
           vmp_verify_key("short", 100));
    APPEND("vmp_verify_key(NULL,100)               = %d\n",
           vmp_verify_key(NULL, 100));
    APPEND("\n");

    APPEND("--- External calls (PLT) ---\n");
    char hex[64];
    int rc;

    rc = vmp_md5_hex("hello", hex, sizeof(hex));
    APPEND("vmp_md5_hex(\"hello\")     = %s  (rc=%d)\n", hex, rc);

    rc = vmp_md5_hex("", hex, sizeof(hex));
    APPEND("vmp_md5_hex(\"\")          = %s  (rc=%d)\n", hex, rc);

    rc = vmp_md5_hex("The quick brown fox jumps over the lazy dog", hex, sizeof(hex));
    APPEND("vmp_md5_hex(\"The q...\")  = %s  (rc=%d)\n", hex, rc);
    APPEND("\n");

    char procname[128];
    int pnlen = vmp_get_process_name(procname, sizeof(procname));
    APPEND("vmp_get_process_name() = \"%s\"  (len=%d)\n", procname, pnlen);

    APPEND("\n=== Test completed ===\n");

#undef APPEND

    LOGI("%s", buf);
    return (*env)->NewStringUTF(env, buf);
}
