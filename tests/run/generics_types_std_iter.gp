// phase: 07

// A generic type of a standard package (iter.Seq, iter.Seq2) instantiated by a user package: %T names and calls.
package main

import (
	"fmt"
	"iter"
)

func count(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func main() {
	var s iter.Seq[int] = count(3)
	fmt.Printf("%T\n", s)
	sum := 0
	s(func(v int) bool { sum += v; return true })
	fmt.Println(sum)
	var q iter.Seq2[string, int]
	fmt.Printf("%T %v\n", q, q == nil)
}

// Output:
// iter.Seq[int]
// 3
// iter.Seq2[string,int] true
