# Makefile for Hybrid Mode e2e test
# Builds a standalone x86_64 binary that embeds the VM interpreter.

CC = gcc
CFLAGS = -static -O0 -fomit-frame-pointer -fno-stack-protector -fno-builtin -nostdlib \
         -march=x86-64 -I../../stub/linux/x86_64 \
         -DVM_INDIRECT_DISPATCH

TARGET = hybrid_test_x86_64

all: $(TARGET)

$(TARGET): hybrid_test.o vm_interp.o vm_entry.o
	$(CC) $(CFLAGS) -o $@ $^

hybrid_test.o: hybrid_test.c
	$(CC) $(CFLAGS) -c $< -o $@

vm_interp.o: ../../stub/linux/x86_64/vm_interp.c
	$(CC) $(CFLAGS) -c $< -o $@

vm_entry.o: ../../stub/linux/x86_64/vm_entry.S
	$(CC) $(CFLAGS) -c $< -o $@

clean:
	rm -f $(TARGET) *.o

.PHONY: all clean
