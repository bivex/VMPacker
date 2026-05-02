/*
 * demo_simple.c — Minimal VMP Test: Pure calculation, no external calls.
 *
 * check_simple(a) = a * 3 + 7
 */

int check_simple(int a) {
  return a * 3 + 7;
}

/* Static buffer for output to avoid stack issues in _start */
char out_buf[2] = {0, '\n'};

void _start(void) {
  int result = check_simple(10); /* Expected: 37 */

  /* Prepare output: '0' + 7 = '7' */
  out_buf[0] = '0' + (result % 10);

  /* syscall: write(fd=1, buf=&out_buf, len=2) */
  register long x0 asm("x0") = 1;
  register long x1 asm("x1") = (long)out_buf;
  register long x2 asm("x2") = 2;
  register long x8 asm("x8") = 64; /* __NR_write */
  asm volatile("svc #0" : : "r"(x0), "r"(x1), "r"(x2), "r"(x8) : "memory");

  /* syscall: exit(result) */
  x0 = result;
  x8 = 93; /* __NR_exit */
  asm volatile("svc #0" : : "r"(x0), "r"(x8));
}
