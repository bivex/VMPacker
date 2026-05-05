#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <stdint.h>
#include <openssl/md5.h>

// Protected functions (VM entry points)
extern int vmp_compute(int a, int b, int c);
extern int vmp_verify_key(const char *key);
extern void vmp_md5_hex(const char *input, char *output);
extern const char *vmp_get_process_name(int fd);

// ------- Simplified MD5 hex (no snprintf) ------
static void md5_to_hex(const unsigned char *md, char *out) {
    const char hex[] = "0123456789abcdef";
    for (int i = 0; i < 16; i++) {
        out[i*2]   = hex[(md[i] >> 4) & 0xF];
        out[i*2+1] = hex[md[i] & 0xF];
    }
    out[32] = '\0';
}

void vmp_md5_hex(const char *input, char *output) {
    unsigned char md[16];
    MD5((unsigned char *)input, strlen(input), md);
    md5_to_hex(md, output);
}

// ------ Rest of the file unchanged ------
int vmp_compute(int a, int b, int c) {
    int result = a + b + c;
    return result;
}

int vmp_verify_key(const char *key) {
    if (key == NULL) return 0;
    if (strcmp(key, "test_key") == 0) return 1;
    if (strcmp(key, "admin") == 0) return 2;
    return 0;
}

const char *vmp_get_process_name(int fd) {
    static char name[256] = {0};
    ssize_t n = read(fd, name, sizeof(name)-1);
    if (n > 0) {
        name[n] = '\0';
        return name;
    }
    return NULL;
}
