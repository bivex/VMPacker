/* demo_mrs_udiv.c -- Test VMP translation of MRS and UDIV instructions
 *
 * Features:
 *   1. timing_checkpoint: Read timer using MRS cntvct_el0
 *   2. timing_check:     Calculate time difference using MRS cntfrq_el0 + UDIV
 *
 * Compile:
 *   aarch64-linux-gnu-gcc -O2 -static -o demo_mrs_udiv demo_mrs_udiv.c
 */
#include <stdint.h>
#include <stdio.h>


/* Read ARM64 timer value */
uint64_t my_timing_checkpoint(void) {
  uint64_t val;
  __asm__ volatile("mrs %0, cntvct_el0" : "=r"(val));
  return val;
}

/* Calculate elapsed time (microseconds) using MRS + UDIV */
int my_timing_check(uint64_t t_start, uint64_t t_end, uint32_t threshold_us) {
  uint64_t freq;
  __asm__ volatile("mrs %0, cntfrq_el0" : "=r"(freq));

  if (freq == 0 || t_end <= t_start)
    return 0;

  uint64_t elapsed_us = (t_end - t_start) * 1000000 / freq;
  return elapsed_us > (uint64_t)threshold_us ? 1 : 0;
}

int main(void) {
  uint64_t t0 = my_timing_checkpoint();

  /* Perform some calculations to consume time */
  volatile uint64_t sum = 0;
  for (int i = 0; i < 1000; i++) {
    sum += i * i;
  }

  uint64_t t1 = my_timing_checkpoint();

  /* Threshold 1 second - 1000 iterations will not exceed it */
  int slow = my_timing_check(t0, t1, 1000000);
  /* Threshold 0 microseconds - any operation will exceed it */
  int fast = my_timing_check(t0, t1, 0);

  if (slow == 0 && fast == 1) {
    printf("MRS_UDIV PASS\n");
    return 0;
  } else {
    printf("MRS_UDIV FAIL slow=%d fast=%d\n", slow, fast);
    return 1;
  }
}
