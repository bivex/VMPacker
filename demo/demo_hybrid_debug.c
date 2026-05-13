#include <stdio.h>
#include <stdlib.h>

// Test 1: Just compute hash, no strtoul, no comparison
__attribute__((noinline))
unsigned int test_hash(unsigned int uid) {
    unsigned int hash = uid ^ 0xA5A5A5A5;
    hash += 0xDEAD;
    hash ^= (hash >> 16);
    int rot = (uid & 7) + 3;
    hash = (hash << (rot & 31)) | (hash >> (32 - (rot & 31)));
    return hash;
}

// Test 2: Full hash with two ROLs
__attribute__((noinline))
unsigned int test_hash_full(unsigned int uid) {
    unsigned int hash = uid ^ 0xA5A5A5A5;
    hash += 0xDEAD;
    hash ^= (hash >> 16);
    int rot = (uid & 7) + 3;
    hash = (hash << (rot & 31)) | (hash >> (32 - (rot & 31)));
    hash *= 0x12345678;
    hash ^= 0xCAFEBABE;
    int rot2 = (hash & 0xF) + 1;
    hash = (hash << (rot2 & 31)) | (hash >> (32 - (rot2 & 31)));
    return hash;
}

// Test 3: Full check with strtoul
__attribute__((noinline))
int test_check(unsigned int uid, const char *key_str) {
    unsigned int hash = uid ^ 0xA5A5A5A5;
    hash += 0xDEAD;
    hash ^= (hash >> 16);
    int rot = (uid & 7) + 3;
    hash = (hash << (rot & 31)) | (hash >> (32 - (rot & 31)));
    hash *= 0x12345678;
    hash ^= 0xCAFEBABE;
    int rot2 = (hash & 0xF) + 1;
    hash = (hash << (rot2 & 31)) | (hash >> (32 - (rot2 & 31)));
    unsigned int provided = (unsigned int)strtoul(key_str, NULL, 16);
    return (hash == provided) ? 1 : 0;
}

int main() {
    unsigned int uid = 42;
    const char *correct = "7EEF185C";
    const char *wrong   = "DEADBEEF";

    printf("=== Hash Debug ===\n");
    printf("test_hash(42)    = 0x%08X (expect 0x8E8F27C1)\n", test_hash(uid));
    printf("test_hash_full(42) = 0x%08X (expect 0x7EEF185C)\n\n", test_hash_full(uid));

    printf("test_check(42, %s) = %d (expect 1)\n", correct, test_check(uid, correct));
    printf("test_check(42, %s) = %d (expect 0)\n", wrong,   test_check(uid, wrong));

    return 0;
}
