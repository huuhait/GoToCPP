// phase: 08

// The function value and the arguments of a go statement are evaluated by the calling goroutine,
// when the statement runs; only the call happens in the new goroutine.
package main

import "fmt"

var trace []string

func note(s string) string {
	trace = append(trace, s)
	return s
}

func show(out chan<- string, a, b string) {
	out <- a + "+" + b
}

func pick() func(chan<- string, string, string) {
	note("pick")
	return show
}

func main() {
	out := make(chan string)
	x := "x1"
	go pick()(out, note("arg1"), x)
	x = "x2" // too late: the argument was copied already
	fmt.Println(trace) // pick and the arguments were evaluated in order, before the goroutine ran
	fmt.Println(<-out)

	// a method value's receiver is evaluated by the go statement too
	type T struct{ n int }
	t := T{1}
	f := func(c chan<- int, v T) { c <- v.n }
	res := make(chan int)
	go f(res, t)
	t.n = 99
	fmt.Println(<-res)

	// a pointer argument is shared: the goroutine sees the change made before it sends
	p := &T{5}
	ready := make(chan bool)
	g := func(c chan<- int, q *T) { <-ready; c <- q.n }
	go g(res, p)
	p.n = 6
	ready <- true
	fmt.Println(<-res)
}

// Output:
// [pick arg1]
// arg1+x1
// 1
// 6
