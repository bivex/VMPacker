package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"github.com/vmpacker/pkg/vm"
)

func main() {
	vm.GenerateDynamicISA()
	goSizes := make(map[string]int)
	for op, info := range vm.OpTable() {
		// we need to find its logical name
		// actually OpTable is keyed by physical byte.
		// we want logical name -> size.
		// let's use the reflection or just regex on opcodes.go!
	}
}
