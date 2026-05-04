#include <stdio.h>
#include <string.h>

/* A simple function with strings to test encryption */
int check_security(const char *input) {
    const char *secret = "VMP_SECRET_12345";
    const char *msg_success = "Access Granted!";
    const char *msg_fail = "Access Denied!";

    if (strcmp(input, secret) == 0) {
        printf("%s\n", msg_success);
        return 1;
    } else {
        printf("%s\n", msg_fail);
        return 0;
    }
}

int main(int argc, char **argv) {
    if (argc < 2) {
        printf("Usage: %s <password>\n", argv[0]);
        return 1;
    }
    return check_security(argv[1]);
}
