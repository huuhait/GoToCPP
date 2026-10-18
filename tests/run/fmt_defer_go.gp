// phase: 05

// fmt calls in defer statements and closures evaluate their operands when the defer runs (Go:
// when the defer statement runs); with values that change afterwards.
package main

import "fmt"

type P struct{ X int }

func main() {
	x := 1
	p := P{1}
	xs := []int{1}
	defer fmt.Println("deferred", x, p, xs)
	defer fmt.Printf("%d %v %v\n", x, p, xs)
	defer func() { fmt.Println("closure", x, p, xs) }()
	x = 2
	p.X = 2
	xs[0] = 2
	xs = append(xs, 3)
	defer fmt.Print(x, " ", p, " ", xs, "\n")
	fmt.Println("main", x, p, xs)
}

// Output:
// main 2 {2} [2 3]
// 2 {2} [2 3]
// closure 2 {2} [2 3]
// 1 {1} [2]
// deferred 1 {1} [2]
