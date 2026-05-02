#ifndef LIBVMPTEST_H
#define LIBVMPTEST_H

#ifdef __cplusplus
extern "C" {
#endif

/**
 * vmp_compute - pure computation function (no external calls).
 *
 * Hashes the input string with arithmetic, bitwise, loop, and branch
 * instructions — the same mix that log2Console exercises.
 *
 * @param input  NUL-terminated string (NULL → returns -1)
 * @param mode   selects the post-hash transform (0/1/2)
 * @param seed   initial hash seed
 * @return       16-bit result (0x0000–0xFFFF), or -1 on NULL input
 */
int vmp_compute(const char *input, int mode, int seed);

/**
 * vmp_verify_key - license-key style verifier (no external calls).
 *
 * Walks the key string, accumulates a checksum, and returns 1 if it
 * matches the expected value for the given product_id, 0 otherwise.
 */
int vmp_verify_key(const char *key, int product_id);

/**
 * vmp_md5_hex - compute MD5 digest and write hex string.
 *
 * Calls libc functions (strlen, memcpy, memset, snprintf) via PLT,
 * exercising the VM's ability to handle external function calls.
 *
 * @param input    NUL-terminated string to hash (NULL → returns -1)
 * @param out_hex  output buffer for 32-char hex digest + NUL
 * @param out_len  size of out_hex (must be >= 33)
 * @return         0 on success, -1 on error
 */
int vmp_md5_hex(const char *input, char *out_hex, int out_len);

/**
 * vmp_get_process_name - read current process name from /proc/self/comm.
 *
 * Calls libc I/O functions (open, read, close, memset) via PLT.
 *
 * @param out      output buffer for process name
 * @param out_len  size of out buffer
 * @return         length of name on success, -1 on error
 */
int vmp_get_process_name(char *out, int out_len);

#ifdef __cplusplus
}
#endif

#endif /* LIBVMPTEST_H */
