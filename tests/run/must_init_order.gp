// phase: 10

package main

import "fmt"

func init!() {
	fmt.Println("init! 1")
}

func init() {
	fmt.Println("plain init 2")
}

func init!() {
	fmt.Println("init! 3")
}

func main() { fmt.Println("main") }

// Output:
// init! 1
// plain init 2
// init! 3
// main
