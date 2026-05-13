#include <stdio.h>
#include <string.h>

__attribute__((noinline))
int verify_license_key(unsigned int user_id, unsigned int key, int shift_amount) {
    unsigned int hash = user_id ^ 0x55AA55AA;
    hash += 0x1337;
    
    // Инструкции сдвига на переменную (SHL/SHR с регистром CL) 
    // заставят транслятор использовать Hybrid Mode.
    hash = (hash << (shift_amount & 31)) | (hash >> (32 - (shift_amount & 31)));
    
    hash ^= 0xDEADBEEF;
    
    if (hash == key) return 1;
    return 0;
}

int main(int argc, char** argv) {
    unsigned int id = 12345;
    int shift = 7;
    
    // Правильный ключ вычисленный через Python: 0xB91DBC5
    unsigned int valid_key = 0xB91DBC5; 

    printf("[*] Testing Hybrid Mode license check...\n");
    printf("[*] ID: %u, Shift: %d, Key: 0x%08X\n", id, shift, valid_key);

    if (verify_license_key(id, valid_key, shift)) {
        printf("[+] ACCESS GRANTED: License is valid!\n");
    } else {
        printf("[-] ACCESS DENIED: Invalid key!\n");
    }
    
    return 0;
}
