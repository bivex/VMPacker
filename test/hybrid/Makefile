# Makefile for Hybrid Mode e2e test
# Builds a standalone x86_64 binary that embeds the VM interpreter.

CC = gcc
CFLAGS = -static -O1 -fno-stack-protector -fno-builtin -nostdlib \
         -march=x86-64 -I../../stub/linux/x86_64 \
         -DVM_INDIRECT_DISPATCH

SRC = hybrid_test.c
OBJ = $(SRC:.c=.o)
INTERP_SRC = ../../stub/linux/x86_64/vm_interp.c
INTERP_OBJ = vm_interp.o
TARGET = hybrid_test_x86_64

all: $(TARGET)

$(INTERP_OBJ): $(INTERP_SRC)
	$(CC) $(CFLAGS) -c $< -o $@

$(TARGET): hybrid_test.o $(INTERP_OBJ)
	$(CC) $(CFLAGS) -o $@ $^
	@echo "[+] Built $@"

hybrid_test.o: hybrid_test.c
	$(CC) $(CFLAGS) -c $< -o $@

clean:
	rm -f $(TARGET) *.o

.PHONY: all clean
