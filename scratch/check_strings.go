package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"unicode"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check_strings.go <file>")
		return
	}

	data, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	var current []byte
	for _, b := range data {
		if unicode.IsPrint(rune(b)) {
			current = append(current, b)
		} else {
			if len(current) >= 4 {
				fmt.Println(string(current))
			}
			current = nil
		}
	}
	if len(current) >= 4 {
		fmt.Println(string(current))
	}
}
