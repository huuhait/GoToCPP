// phase: 02

// fallthrough: chains, into default, from default in the middle, and a
// break inside a case that falls through.
package main

import "fmt"

func describe(n int) {
	fmt.Print(n, ":")
	switch n {
	case 0:
		fmt.Print(" zero")
		fallthrough
	case 1:
		fmt.Print(" small")
		fallthrough
	case 2:
		fmt.Print(" tiny")
	case 3:
		fmt.Print(" three")
		fallthrough
	default:
		fmt.Print(" default")
		fallthrough
	case 10:
		fmt.Print(" ten-ish")
	case 11:
		fmt.Print(" eleven")
	}
	fmt.Println()
}

func tagless(n int) {
	switch {
	case n < 0:
		fmt.Println("negative")
		fallthrough
	case n == 0:
		fmt.Println("not positive")
	case n > 100:
		if n > 1000 {
			fmt.Println("huge")
			break
		}
		fmt.Println("big")
		fallthrough
	default:
		fmt.Println("positive")
	}
}

func main() {
	for i := range 5 {
		describe(i)
	}
	describe(10)
	describe(11)
	describe(42)
	tagless(-1)
	tagless(0)
	tagless(500)
	tagless(5000)
	tagless(7)
}

// Output:
// 0: zero small tiny
// 1: small tiny
// 2: tiny
// 3: three default ten-ish
// 4: default ten-ish
// 10: ten-ish
// 11: eleven
// 42: default ten-ish
// negative
// not positive
// not positive
// big
// positive
// huge
// positive
