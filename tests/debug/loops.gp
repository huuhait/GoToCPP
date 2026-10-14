// phase: 02

// Loops, switches, labels and goto, and a shadowed name: the inner v keeps
// its Go name in a C++ block of its own (int64_t gp_t0 = v + i + 1; {
// int64_t v = gp_t0; ...}), so "print v" shows the outer v up to and on the
// declaration line, then the inner v (README.md, "Shadowed names").
package main

import "fmt"

func main() {
	v := 10
	total := 0
	for i := range 3 {
		v := v + i + 1
		total += v // gdb: print i = 0; print v = 11; next 17 14; print v = 10; next 15; print v = 10; next 16; print i = 1; print v = 12
		fmt.Println(i, v, classify(v-12))
	}
	k := 0 // gdb: print total = 36; print v = 10
outer:
	for k < 10 {
		switch k {
		case 3:
			k += 2 // gdb: print k = 3; next 25 21 22 29 21 22
			continue outer
		case 7:
			break outer // gdb: print k = 7; next 32 33 37
		default:
			k++
		}
	}
	if k > 5 {
		goto end
	}
	fmt.Println("not reached")
end:
	report(k)
}

// report is a func without results: next at its last line goes to its
// closing brace, then back to the caller, after the call.
func report(k int) {
	fmt.Println("k =", k) // gdb: print k = 7; bt gpkg_main::report gpkg_main::main; next 44 38
}

func classify(n int) string {
	switch {
	case n < 0:
		return "negative"
	case n == 0:
		return "zero"
	}
	return "positive"
}
