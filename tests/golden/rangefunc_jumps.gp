// phase: 07

package main

import "fmt"

func Count(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

// find returns from the middle of two loops, with defers and a labeled continue of the outer one.
func find(n int) (res int, ok bool) {
	defer fmt.Println("find done")
	total := 0
outer:
	for x := range Count(n) {
		defer fmt.Println("defer in body", x)
		for y := range Count(n) {
			total += y
			switch {
			case y == 1:
				continue outer
			case x == 2:
				return x*10 + y, true
			case total > 100:
				goto out
			}
		}
	}
out:
	return total, false
}

func main() {
	for x, ok := range func(yield func(int, bool) bool) { yield(1, true) } {
		fmt.Println(x, ok)
		if x == 1 {
			break
		}
	}
	fmt.Println(find(4))
}
