/**
 * libvmptest.c — VMPacker shared-library test target
 *
 * Two categories of exported functions:
 *
 *   1) Pure computation (no external calls):
 *      vmp_compute, vmp_verify_key
 *
 *   2) With libc / PLT calls:
 *      vmp_md5_hex        — strlen, memcpy, memset, snprintf
 *      vmp_get_process_name — open, read, close, memset
 *
 * Build:
 *   arm-linux-gnueabihf-gcc -shared -fPIC -O1 -o libvmptest.so libvmptest.c
 * Protect:
 *   vmpacker -func "vmp_compute,vmp_verify_key,vmp_md5_hex,vmp_get_process_name" \
 *            -v -o libvmptest_protected.so libvmptest.so
 */

#include "libvmptest.h"

#include <string.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>

/* ================================================================== */
/*  vmp_compute  (pure computation)                                    */
/* ================================================================== */
int vmp_compute(const char *input, int mode, int seed) {
    if (!input)
        return -1;

    unsigned int h = (unsigned int)seed ^ 0xA5A5A5A5u;

    int i = 0;
    while (input[i] && i < 128) {
        h ^= (unsigned char)input[i];
        h *= 0x01000193u;
        i++;
    }

    int result = (int)(h & 0x7FFFFFFFu);

    if (mode == 1) {
        result = (result >> 4) ^ (result << 3);
        result &= 0x7FFFFFFF;
    } else if (mode == 2) {
        result = result * 7 + 13;
    } else {
        result ^= 0xDEADBEEF;
        result &= 0x7FFFFFFF;
    }

    for (int j = 0; j < (mode + 1) * 3; j++) {
        result += j * 5 - 2;
    }

    return result & 0xFFFF;
}

/* ================================================================== */
/*  vmp_verify_key  (pure computation)                                 */
/* ================================================================== */
int vmp_verify_key(const char *key, int product_id) {
    if (!key)
        return 0;

    unsigned int acc = (unsigned int)product_id * 2654435761u;

    int i = 0;
    while (key[i] && i < 32) {
        acc = (acc << 5) | (acc >> 27);
        acc ^= (unsigned char)key[i];
        acc += (unsigned int)i * 7;
        i++;
    }

    if (i < 4)
        return 0;

    unsigned int expected = (unsigned int)product_id ^ 0x55AA55AAu;
    expected = (expected * 31) & 0xFFFFFFFFu;

    return (acc & 0xFFFF) == (expected & 0xFFFF) ? 1 : 0;
}

/* ================================================================== */
/*  MD5 implementation (RFC 1321)                                      */
/*  Uses libc: memcpy, memset, strlen, snprintf — all go through PLT  */
/* ================================================================== */

static const unsigned int md5_t[64] = {
    0xd76aa478,0xe8c7b756,0x242070db,0xc1bdceee,0xf57c0faf,0x4787c62a,0xa8304613,0xfd469501,
    0x698098d8,0x8b44f7af,0xffff5bb1,0x895cd7be,0x6b901122,0xfd987193,0xa679438e,0x49b40821,
    0xf61e2562,0xc040b340,0x265e5a51,0xe9b6c7aa,0xd62f105d,0x02441453,0xd8a1e681,0xe7d3fbc8,
    0x21e1cde6,0xc33707d6,0xf4d50d87,0x455a14ed,0xa9e3e905,0xfcefa3f8,0x676f02d9,0x8d2a4c8a,
    0xfffa3942,0x8771f681,0x6d9d6122,0xfde5380c,0xa4beea44,0x4bdecfa9,0xf6bb4b60,0xbebfbc70,
    0x289b7ec6,0xeaa127fa,0xd4ef3085,0x04881d05,0xd9d4d039,0xe6db99e5,0x1fa27cf8,0xc4ac5665,
    0xf4292244,0x432aff97,0xab9423a7,0xfc93a039,0x655b59c3,0x8f0ccc92,0xffeff47d,0x85845dd1,
    0x6fa87e4f,0xfe2ce6e0,0xa3014314,0x4e0811a1,0xf7537e82,0xbd3af235,0x2ad7d2bb,0xeb86d391
};
static const int md5_s[64] = {
    7,12,17,22, 7,12,17,22, 7,12,17,22, 7,12,17,22,
    5, 9,14,20, 5, 9,14,20, 5, 9,14,20, 5, 9,14,20,
    4,11,16,23, 4,11,16,23, 4,11,16,23, 4,11,16,23,
    6,10,15,21, 6,10,15,21, 6,10,15,21, 6,10,15,21
};

#define ROTL32(x, n) (((x) << (n)) | ((x) >> (32 - (n))))

static void md5_transform(unsigned int state[4], const unsigned char block[64]) {
    unsigned int m[16];
    memcpy(m, block, 64);  /* PLT: memcpy */

    unsigned int a = state[0], b = state[1], c = state[2], d = state[3];

    for (int i = 0; i < 64; i++) {
        unsigned int f, g;
        if (i < 16) {
            f = (b & c) | (~b & d);
            g = (unsigned int)i;
        } else if (i < 32) {
            f = (d & b) | (~d & c);
            g = (5 * (unsigned int)i + 1) % 16;
        } else if (i < 48) {
            f = b ^ c ^ d;
            g = (3 * (unsigned int)i + 5) % 16;
        } else {
            f = c ^ (b | ~d);
            g = (7 * (unsigned int)i) % 16;
        }
        unsigned int tmp = d;
        d = c;
        c = b;
        b = b + ROTL32(a + f + md5_t[i] + m[g], md5_s[i]);
        a = tmp;
    }

    state[0] += a;
    state[1] += b;
    state[2] += c;
    state[3] += d;
}

int vmp_md5_hex(const char *input, char *out_hex, int out_len) {
    if (!input || !out_hex || out_len < 33)
        return -1;

    unsigned int state[4] = { 0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476 };

    size_t len = strlen(input);  /* PLT: strlen */
    size_t total = len;

    unsigned char buf[64];
    size_t off = 0;

    while (off + 64 <= len) {
        md5_transform(state, (const unsigned char *)input + off);
        off += 64;
    }

    size_t rem = len - off;
    memset(buf, 0, 64);           /* PLT: memset */
    memcpy(buf, input + off, rem); /* PLT: memcpy */
    buf[rem] = 0x80;

    if (rem >= 56) {
        md5_transform(state, buf);
        memset(buf, 0, 64);       /* PLT: memset */
    }

    unsigned long long bits = (unsigned long long)total * 8;
    buf[56] = (unsigned char)(bits);
    buf[57] = (unsigned char)(bits >> 8);
    buf[58] = (unsigned char)(bits >> 16);
    buf[59] = (unsigned char)(bits >> 24);
    buf[60] = (unsigned char)(bits >> 32);
    buf[61] = (unsigned char)(bits >> 40);
    buf[62] = (unsigned char)(bits >> 48);
    buf[63] = (unsigned char)(bits >> 56);
    md5_transform(state, buf);

    unsigned char digest[16];
    memcpy(digest, state, 16);    /* PLT: memcpy */

    for (int i = 0; i < 16; i++) {
        snprintf(out_hex + i * 2, 3, "%02x", digest[i]);  /* PLT: snprintf */
    }
    out_hex[32] = '\0';
    return 0;
}

/* ================================================================== */
/*  vmp_get_process_name  (reads /proc/self/comm via libc I/O)         */
/* ================================================================== */
int vmp_get_process_name(char *out, int out_len) {
    if (!out || out_len < 2)
        return -1;

    memset(out, 0, (size_t)out_len);  /* PLT: memset */

    int fd = open("/proc/self/comm", O_RDONLY);  /* PLT: open */
    if (fd < 0)
        return -1;

    int n = (int)read(fd, out, (size_t)(out_len - 1));  /* PLT: read */
    close(fd);  /* PLT: close */

    if (n <= 0)
        return -1;

    /* strip trailing newline */
    if (n > 0 && out[n - 1] == '\n') {
        out[n - 1] = '\0';
        n--;
    }
    return n;
}
