// All the for forms: for {}, for cond, 3-clause loops (with a post that is one
// C++ expression or not), range over an integer (Go 1.22: the range variable is a
// copy of a hidden counter), and continue and break in each.
package main

import "fmt"

var evals = 0

func limit(n int) int {
	evals++
	return n
}

func main() {
	// for i := range n: assigning to i doesn't change the iteration.
	for i := range 5 {
		print(i)
		i += 2
	}
	println()
	n := 3
	for i := range n {
		n = 10 // n is evaluated once
		fmt.Print(i, " ")
	}
	fmt.Println(n)
	for i := range limit(4) {
		fmt.Print(i, " ")
	}
	fmt.Println("evals", evals)
	for range 3 {
		fmt.Print("x")
	}
	fmt.Println()
	var u uint8 = 255
	for i := range u {
		if i < 253 {
			continue
		}
		fmt.Print(i, " ")
	}
	fmt.Println()
	var k int8 = 100
	for k = range 3 { // a variable that exists: it keeps the last value
		k *= 10
	}
	fmt.Println(k)
	for k = range 0 { // no iterations: k doesn't change
	}
	fmt.Println(k)
	for i := range -5 {
		fmt.Println("never", i)
	}
	for i := range 3 {
		i := i * 100 // a new i in the body
		fmt.Print(i, " ")
	}
	fmt.Println()

	// 3-clause loops.
	for i := 0; i < 3; i++ {
		fmt.Print(i, " ")
	}
	fmt.Println()
	for i, j := 0, 10; i < j; i, j = i+1, j-2 {
		if i == 1 {
			continue // the post still runs
		}
		fmt.Print(i, ":", j, " ")
	}
	fmt.Println()
	for i, s := 0, "a"; i < 3; i++ {
		s += "b"
		fmt.Print(s, " ")
	}
	fmt.Println()
	sum := 0
	for i := 10; i > 0; i -= 3 {
		sum += i
	}
	fmt.Println(sum)
	i := 0
	for i = 5; i < 8; i++ {
	}
	fmt.Println(i)
	for i := 0; i < 3; {
		i++
		fmt.Print(i, " ")
	}
	fmt.Println()
	for j := 0; ; j++ {
		if j == 3 {
			fmt.Println("j", j)
			break
		}
	}
	for i := 0; i < 4; i++ {
		i := i + 10 // a new i in the body
		fmt.Print(i, " ")
	}
	fmt.Println()
	a, b := 0, 0
	for ; a < 3; a, b = a+1, b+a {
		if a == 2 {
			continue
		}
		x := a * 2
		fmt.Print(x, " ")
	}
	fmt.Println(a, b)

	// for cond and for {}.
	m := 1
	for m < 100 {
		m *= 3
	}
	fmt.Println(m)
	c := 0
	for {
		c++
		if c%2 == 1 {
			continue
		}
		if c > 6 {
			break
		}
		fmt.Print(c, " ")
	}
	fmt.Println()
	for false {
		fmt.Println("never")
	}
}
