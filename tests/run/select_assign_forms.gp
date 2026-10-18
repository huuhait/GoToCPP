// phase: 08

// Receive cases with assignment: the left-hand sides are evaluated only for the case that is
// chosen, after the choice.
package main

import "fmt"

type box struct {
	v  int
	ok bool
}

var evals []string

func idx(name string, i int) int {
	evals = append(evals, name)
	return i
}

func main() {
	a := make(chan int, 1)
	b := make(chan int, 1)
	arr := []int{0, 0, 0}
	m := map[string]int{}
	var x int
	var ok bool
	var bx box

	a <- 10
	select {
	case x = <-a:
	case arr[idx("arr", 1)] = <-b:
	}
	fmt.Println(x, arr, evals)

	b <- 20
	select {
	case x = <-a:
	case arr[idx("arr", 2)] = <-b:
	}
	fmt.Println(x, arr, evals)

	a <- 30
	select {
	case bx.v, bx.ok = <-a:
	}
	fmt.Println(bx)

	close(b)
	select {
	case m[fmt.Sprint("k", idx("m", 0))], ok = <-b:
	}
	fmt.Println(m, ok, evals)

	a <- 7
	select {
	case _, ok = <-a:
		fmt.Println("blank", ok)
	}
	a <- 8
	select {
	case _ = <-a:
		fmt.Println("blank2")
	}
	p := &x
	a <- 9
	select {
	case *p = <-a:
	}
	fmt.Println(x)
}

// Output:
// 10 [0 0 0] []
// 10 [0 0 20] [arr]
// {30 true}
// map[k0:0] false [arr m]
// blank true
// blank2
// 9
