// bench: fib
// category: cpu
// size: 38
// smoke: 20
// twin: yes
// about: naive recursive Fibonacci (call overhead)
package main

import "fmt"

func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func main() {
	fmt.Println(fib(benchN))
}
