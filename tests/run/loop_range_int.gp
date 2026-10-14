// phase: 02

// range over an integer with Go 1.22 semantics: each iteration has its own
// variable, so changing it in the body doesn't change the iteration, and
// the count is evaluated once.
package main

import "fmt"

func limit(n int) int {
	fmt.Println("limit called")
	return n
}

func main() {
	for i := range 5 {
		fmt.Print(i, " ")
		i += 2
		fmt.Print(i, " ")
	}
	fmt.Println("|")

	n := 3
	for i := range n {
		n = 10
		fmt.Print(i, " ")
	}
	fmt.Println("n is", n)

	for i := range limit(3) {
		fmt.Print(i, " ")
	}
	fmt.Println("|")

	var i8 int8 = 4
	for v := range i8 {
		fmt.Print(v*32, " ")
	}
	fmt.Println("|")

	for i := range 0 {
		fmt.Println("never", i)
	}
	for i := range -3 {
		fmt.Println("never", i)
	}

	total := 0
	for i := range 100 {
		if i == 50 {
			break
		}
		if i%10 != 0 {
			continue
		}
		total += i
	}
	fmt.Println("total", total)

	for i := 0; i < 3; i++ {
		fmt.Print(i, " ")
		i++
	}
	fmt.Println("|")
}

// Output:
// 0 2 1 3 2 4 3 5 4 6 |
// 0 1 2 n is 10
// limit called
// 0 1 2 |
// 0 32 64 96 |
// total 100
// 0 2 |
