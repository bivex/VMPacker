package main

import "fmt"

// checkKey — pure computation function for testing VMP-protected Go binaries
// algorithm: ((input * 7) + 42) ^ 0xFF
//
//go:noinline
func checkKey(input int) int {
	a := input * 7
	b := a + 42
	c := b ^ 0xFF
	return c
}

func main() {
	result := checkKey(10)
	fmt.Printf("checkKey(10) = %d\n", result)
	// expected: ((10 * 7) + 42) ^ 0xFF = (70 + 42) ^ 255 = 112 ^ 255 = 143
	if result == 143 {
		fmt.Println("[+] OK")
	} else {
		fmt.Println("[-] FAIL")
	}
}
