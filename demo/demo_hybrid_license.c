#include <stdio.h>
#include <string.h>
#include <stdlib.h>

// Complex license check that forces hybrid mode via variable shifts
__attribute__((noinline))
int check_license(unsigned int user_id, const char *key_str) {
    unsigned int hash = user_id ^ 0xA5A5A5A5;
    hash += 0xDEAD;
    hash ^= (hash >> 16);

    // Variable rotation — triggers ROL/ROR via CL, hybrid fallback
    int rot = (user_id & 7) + 3;
    hash = (hash << (rot & 31)) | (hash >> (32 - (rot & 31)));

    hash *= 0x12345678;
    hash ^= 0xCAFEBABE;

    // Second variable shift — another hybrid island
    int rot2 = (hash & 0xF) + 1;
    hash = (hash << (rot2 & 31)) | (hash >> (32 - (rot2 & 31)));

    // Parse hex key from string
    unsigned int provided = (unsigned int)strtoul(key_str, NULL, 16);

    return (hash == provided) ? 1 : 0;
}

__attribute__((noinline))
int validate_tier(unsigned int user_id, unsigned int feature_bits) {
    unsigned int mask = user_id ^ 0x5A5A5A5A;
    mask += 0xBEEF;

    // Variable shift for tier calculation — hybrid
    int shift = (user_id % 13) + 1;
    mask = (mask << (shift & 31)) | (mask >> (32 - (shift & 31)));

    mask &= 0xFFFF;

    return (feature_bits & mask) == mask ? 1 : 0;
}

int main(int argc, char **argv) {
    unsigned int uid = 42;

    // Pre-computed: hash for uid=42, rot=5, valid license
    const char *valid_key = "7EEF185C";

    printf("=== Hybrid Mode License Demo ===\n\n");

    printf("[1] License check (uid=%u, key=%s)\n", uid, valid_key);
    if (check_license(uid, valid_key)) {
        printf("    [+] LICENSE VALID\n\n");
    } else {
        printf("    [-] LICENSE INVALID\n\n");
    }

    printf("[2] License check with wrong key\n");
    if (check_license(uid, "DEADBEEF")) {
        printf("    [+] LICENSE VALID (unexpected!)\n\n");
    } else {
        printf("    [-] LICENSE INVALID (expected)\n\n");
    }

    printf("[3] Feature tier check (uid=%u)\n", uid);
    unsigned int features = 0x3FFF;
    if (validate_tier(uid, features)) {
        printf("    [+] PREMIUM TIER GRANTED\n\n");
    } else {
        printf("    [-] TIER DENIED\n\n");
    }

    printf("[4] Feature tier check with different uid\n");
    if (validate_tier(9999, features)) {
        printf("    [+] PREMIUM TIER GRANTED\n\n");
    } else {
        printf("    [-] TIER DENIED\n\n");
    }

    printf("=== All checks completed ===\n");
    return 0;
}
