// phase: 09

package main

import (
	"fmt"
	"testing"
)

func main() {
	sum := 0
	r := testing.Benchmark(func(b *testing.B) {
		b.SetBytes(8)
		for i := 0; i < b.N; i++ {
			sum += i
		}
	})
	fmt.Println(r.N > 0, r.Bytes, r.NsPerOp() >= 0, r.T >= 0)

	loops := 0
	r = testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			loops++
		}
	})
	fmt.Println(r.N > 0, loops >= r.N)

	r = testing.Benchmark(func(b *testing.B) {
		b.Run("sub", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
			}
		})
	})
	fmt.Println(r.N)
}

// Output:
// true 8 true true
// true true
// 1
