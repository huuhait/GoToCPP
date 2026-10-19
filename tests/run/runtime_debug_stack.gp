// phase: 08

// debug.Stack returns the calling goroutine's trace as bytes: it starts like runtime.Stack's.
package main

import (
	"fmt"
	"runtime/debug"
)

func has(b []byte, s string) bool {
	for i := 0; i+len(s) <= len(b); i++ {
		if string(b[i:i+len(s)]) == s {
			return true
		}
	}
	return false
}

func f() []byte { return debug.Stack() }

func main() {
	s := f()
	fmt.Println(string(s[:22]), has(s, "main.f"), has(s, "main.main"))
}

// Output:
// goroutine 1 [running]: true true
