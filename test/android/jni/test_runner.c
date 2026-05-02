/**
 * test_runner.c — Standalone test for Android adb shell
 *
 * Loads the .so via dlopen and exercises all exported functions.
 * Usage:
 *   adb push test_runner /data/local/tmp/
 *   adb push libnative_test.so /data/local/tmp/
 *   adb shell "cd /data/local/tmp && LD_LIBRARY_PATH=. ./test_runner"
 *
 * To test protected version:
 *   adb push libnative_test_protected.so /data/local/tmp/libnative_test.so
 *   adb shell "cd /data/local/tmp && LD_LIBRARY_PATH=. ./test_runner"
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <dlfcn.h>

typedef int  (*fn_compute_t)(const char *, int, int);
typedef int  (*fn_verify_key_t)(const char *, int);
typedef int  (*fn_md5_hex_t)(const char *, char *, int);
typedef int  (*fn_get_process_name_t)(char *, int);

static int g_pass = 0;
static int g_fail = 0;

#define CHECK(desc, cond)                                                      \
    do {                                                                        \
        if (cond) {                                                             \
            printf("  [PASS] %s\n", desc);                                     \
            g_pass++;                                                           \
        } else {                                                                \
            printf("  [FAIL] %s\n", desc);                                     \
            g_fail++;                                                           \
        }                                                                       \
    } while (0)

int main(int argc, char *argv[])
{
    const char *lib = argc > 1 ? argv[1] : "./libnative_test.so";

    printf("=== VMPacker Android SO Test Runner ===\n");
    printf("[*] Loading: %s\n\n", lib);

    void *h = dlopen(lib, RTLD_NOW);
    if (!h) {
        fprintf(stderr, "[ERROR] dlopen: %s\n", dlerror());
        return 1;
    }

    fn_compute_t          compute  = (fn_compute_t)dlsym(h, "vmp_compute");
    fn_verify_key_t       verify   = (fn_verify_key_t)dlsym(h, "vmp_verify_key");
    fn_md5_hex_t          md5hex   = (fn_md5_hex_t)dlsym(h, "vmp_md5_hex");
    fn_get_process_name_t getname  = (fn_get_process_name_t)dlsym(h, "vmp_get_process_name");

    if (!compute || !verify || !md5hex || !getname) {
        fprintf(stderr, "[ERROR] Missing symbols: compute=%p verify=%p md5hex=%p getname=%p\n",
                compute, verify, md5hex, getname);
        dlclose(h);
        return 1;
    }

    /* ---- vmp_compute ---- */
    printf("--- vmp_compute ---\n");
    int r1 = compute("hello", 0, 42);
    int r2 = compute("hello", 1, 42);
    int r3 = compute("hello", 2, 42);
    int r4 = compute("world", 0, 42);
    int r5 = compute(NULL, 0, 0);

    printf("  compute(\"hello\",0,42) = %d\n", r1);
    printf("  compute(\"hello\",1,42) = %d\n", r2);
    printf("  compute(\"hello\",2,42) = %d\n", r3);
    printf("  compute(\"world\",0,42) = %d\n", r4);
    printf("  compute(NULL,0,0)      = %d\n", r5);

    CHECK("compute returns non-negative for valid input",  r1 >= 0);
    CHECK("compute returns -1 for NULL",                   r5 == -1);
    CHECK("different mode gives different result",         r1 != r2);
    CHECK("different input gives different result",        r1 != r4);

    /* ---- vmp_verify_key ---- */
    printf("\n--- vmp_verify_key ---\n");
    int v1 = verify("ABCD-1234-EFGH", 100);
    int v2 = verify("short", 100);
    int v3 = verify(NULL, 100);

    printf("  verify_key(\"ABCD-1234-EFGH\",100) = %d\n", v1);
    printf("  verify_key(\"short\",100)           = %d\n", v2);
    printf("  verify_key(NULL,100)               = %d\n", v3);

    CHECK("verify_key returns 0 or 1",  v1 == 0 || v1 == 1);
    CHECK("verify_key(NULL) returns 0", v3 == 0);

    /* ---- vmp_md5_hex ---- */
    printf("\n--- vmp_md5_hex ---\n");
    char hex[64];
    int rc;

    rc = md5hex("hello", hex, sizeof(hex));
    printf("  md5(\"hello\")     = %s  (rc=%d)\n", hex, rc);
    CHECK("md5(\"hello\") matches", rc == 0 && strcmp(hex, "5d41402abc4b2a76b9719d911017c592") == 0);

    rc = md5hex("", hex, sizeof(hex));
    printf("  md5(\"\")          = %s  (rc=%d)\n", hex, rc);
    CHECK("md5(\"\") matches", rc == 0 && strcmp(hex, "d41d8cd98f00b204e9800998ecf8427e") == 0);

    rc = md5hex("The quick brown fox jumps over the lazy dog", hex, sizeof(hex));
    printf("  md5(\"The q...\")  = %s  (rc=%d)\n", hex, rc);
    CHECK("md5(long string) matches", rc == 0 && strcmp(hex, "9e107d9d372bb6826bd81d3542a419d6") == 0);

    rc = md5hex(NULL, hex, sizeof(hex));
    CHECK("md5(NULL) returns -1", rc == -1);

    /* ---- vmp_get_process_name ---- */
    printf("\n--- vmp_get_process_name ---\n");
    char procname[128];
    int pnlen = getname(procname, sizeof(procname));
    printf("  process_name = \"%s\"  (len=%d)\n", procname, pnlen);
    CHECK("get_process_name returns positive length", pnlen > 0);

    dlclose(h);

    /* ---- Summary ---- */
    printf("\n========================================\n");
    printf("  PASS: %d    FAIL: %d    TOTAL: %d\n", g_pass, g_fail, g_pass + g_fail);
    printf("========================================\n");

    return g_fail > 0 ? 1 : 0;
}
