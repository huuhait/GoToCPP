// phase: 08

// Receives in return statements, deferred calls and method calls.
package main

import "fmt"

type Stack struct{ items chan int }

func (s Stack) Pop() int { return <-s.items }

func pair(c chan int) (int, int) { return <-c, <-c }

func pop2(c chan int) (a, b int) {
	defer func() { a, b = b, a }()
	return <-c, <-c
}

func main() {
	c := make(chan int, 10)
	for i := 1; i <= 8; i++ {
		c <- i
	}
	fmt.Println(pair(c))
	fmt.Println(pop2(c))
	s := Stack{c}
	fmt.Println(s.Pop(), s.Pop())
	fmt.Println(len(c))
	defer fmt.Println("deferred argument", <-c)
	fmt.Println(len(c))
}

// Output:
// 1 2
// 4 3
// 5 6
// 2
// 1
// deferred argument 7
