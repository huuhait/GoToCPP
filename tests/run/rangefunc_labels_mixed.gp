// phase: 07

package main

import "fmt"

func Cnt(name string, n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println(name, "done")
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func main() {
	// labels on plain loops and switches, left from range-over-func bodies
outer:
	for i := 0; i < 3; i++ {
		fmt.Println("i", i)
	sw:
		switch i {
		case 0:
			for x := range Cnt("X", 2) {
				for y := range Cnt("Y", 2) {
					fmt.Println("x y", x, y)
					if y == 1 {
						break sw
					}
				}
			}
			fmt.Println("not reached")
		case 1:
			for x := range Cnt("P", 3) {
				for y := range Cnt("Q", 3) {
					if y == 1 {
						continue outer
					}
					fmt.Println("p q", x, y)
				}
			}
		default:
			for x := range Cnt("R", 3) {
				if x == 1 {
					break outer
				}
				fmt.Println("r", x)
			}
		}
		fmt.Println("end of iteration", i)
	}
	fmt.Println("after outer")

	// continue with a post statement that is not simple
	for i, j := 0, 5; i < j; i, j = i+1, j-1 {
	loop:
		for x := range Cnt("S", 3) {
			for y := range Cnt("T", 3) {
				if y == 1 {
					continue loop
				}
				if x == 2 {
					break loop
				}
				fmt.Println("s t", i, j, x, y)
			}
		}
		for range Cnt("U", 1) {
			if i == 1 {
				goto next
			}
		}
		fmt.Println("tail", i)
	next:
	}
}

// Output:
// i 0
// x y 0 0
// x y 0 1
// Y done
// X done
// end of iteration 0
// i 1
// p q 0 0
// Q done
// P done
// i 2
// r 0
// R done
// after outer
// s t 0 5 0 0
// T done
// s t 0 5 1 0
// T done
// T done
// S done
// U done
// tail 0
// s t 1 4 0 0
// T done
// s t 1 4 1 0
// T done
// T done
// S done
// U done
// s t 2 3 0 0
// T done
// s t 2 3 1 0
// T done
// T done
// S done
// U done
// tail 2
