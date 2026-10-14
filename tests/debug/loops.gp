// phase: 02

// Loops, switches, labels and goto, and a shadowed name. gocpp renames a
// variable that shadows one of the same name (v := v + i + 1 becomes
// int64_t v_1 = v + i + 1), so in gdb the inner v is v_1, and "print v" shows
// the outer v.
package main

import "fmt"

func main() {
	v := 10
	total := 0
	for i := range 3 {
		v := v + i + 1
		total += v // gdb: print i = 0; print v_1 = 11; print v = 10; next 17 14 15 16
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
