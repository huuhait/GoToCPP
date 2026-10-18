// phase: 08

// The range variable shadows outer variables; the loop body can declare the same name again; a
// variable of the outer scope is untouched.
package main

import "fmt"

func main() {
	v := "outer"
	c := make(chan int, 3)
	c <- 1
	c <- 2
	c <- 3
	close(c)
	for v := range c {
		v := v * 10
		fmt.Println(v)
	}
	fmt.Println(v)
	var x int
	d := make(chan int, 2)
	d <- 5
	d <- 6
	close(d)
	for x = range d {
	}
	fmt.Println(x)
}

// Output:
// 10
// 20
// 30
// outer
// 6
