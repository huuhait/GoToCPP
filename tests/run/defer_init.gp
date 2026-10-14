// phase: 03

// Defers in init functions and in main: each init's deferred calls run when that init returns,
// before the next init and before main; main's run when main returns, before the program
// exits.
package main

import "fmt"

var order = 0

func next() int {
	order++
	return order
}

func init() {
	defer fmt.Println("init 1 deferred", next())
	fmt.Println("init 1", order)
}

func init() {
	defer fmt.Println("init 2 deferred")
	fmt.Println("init 2", next())
}

func main() {
	defer fmt.Println("main deferred")
	fmt.Println("main", next())
}

// Output:
// init 1 1
// init 1 deferred 1
// init 2 2
// init 2 deferred
// main 3
// main deferred
