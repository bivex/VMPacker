package x86_64

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/vmpacker/pkg/vm"
)

func TestMBARecursionDepth(t *testing.T) {
	vm.GenerateDynamicISA()
	vm.RebuildOpTable()

	// A sample of 100 consecutive ADD instructions (eax, ebx) to measure expansion.
	// ADD EAX, EBX is 01 D8
	code := make([]byte, 0, 200)
	for i := 0; i < 100; i++ {
		code = append(code, 0x01, 0xD8)
	}

	dec := NewDecoder()
	insts, err := dec.Decode(code, 0x1000)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	fmt.Println("=== MBA Recursion Depth Benchmark ===")
	fmt.Printf("%-10s | %-15s | %-10s | %-15s\n", "MaxDepth", "Bytecode Size", "Ratio", "Time")
	fmt.Println("---------------------------------------------------------------")

	for depth := 0; depth <= 10; depth++ {
		// Run multiple times for more stable timing
		var totalTime time.Duration
		var avgSize int
		runs := 5

		for i := 0; i < runs; i++ {
			rand.Seed(time.Now().UnixNano()) // Seed to ensure random MBA paths

			tr := NewTranslator(0x1000, len(code), code)
			tr.SetMBA(true)
			tr.SetMaxMBADepth(depth)

			start := time.Now()
			res, err := tr.Translate(insts)
			if err != nil {
				t.Fatalf("Translate failed at depth %d: %v", depth, err)
			}
			elapsed := time.Since(start)

			totalTime += elapsed
			avgSize += len(res.Bytecode)
		}

		avgTime := totalTime / time.Duration(runs)
		avgSize /= runs
		
		// Base size is when depth is 0 (no MBA applied via recursion limit)
		// but wait, if depth=0 it might still apply a little? 
		// Actually if maxDepth=0, depth(0) >= maxDepth(0) so MBA returns false immediately.
		
		ratio := float64(avgSize) / float64(len(code))

		fmt.Printf("%-10d | %-15d | %-10.2fx | %-15v\n", depth, avgSize, ratio, avgTime)
	}
	fmt.Println("=====================================")
}
