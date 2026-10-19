// phase: 07

package main

import "fmt"

func Count(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println("count cleanup", n)
		for i := 0; i < n; i++ {
			if !yield(i) {
				fmt.Println("count stopped at", i)
				return
			}
		}
		fmt.Println("count exhausted")
	}
}

func main() {
	for x := range Count(10) {
		if x%2 == 0 {
			continue
		}
		if x > 6 {
			break
		}
		fmt.Println("odd", x)
	}
	for x := range Count(3) {
		switch x {
		case 1:
			continue
		case 2:
			break
		}
		fmt.Println("after switch", x)
	}
	for x := range Count(5) {
		for y := range Count(3) {
			if y == 1 {
				continue
			}
			if x == 1 {
				break
			}
			fmt.Println("pair", x, y)
		}
		if x == 2 {
			break
		}
	}
	n := 0
	for range Count(3) {
		n++
		if n == 2 {
			break
		}
	}
	fmt.Println("n", n)
}

// Output:
// odd 1
// odd 3
// odd 5
// count stopped at 7
// count cleanup 10
// after switch 0
// after switch 2
// count exhausted
// count cleanup 3
// pair 0 0
// pair 0 2
// count exhausted
// count cleanup 3
// count stopped at 0
// count cleanup 3
// pair 2 0
// pair 2 2
// count exhausted
// count cleanup 3
// count stopped at 2
// count cleanup 5
// count stopped at 1
// count cleanup 3
// n 2
