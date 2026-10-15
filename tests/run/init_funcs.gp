// phase: 03

// Several init functions in one file run after all package variables are set, in source
// order, each one to its end (its deferred calls too) before the next. They can read and
// change the package variables, make closures (which may outlive them) and call other
// functions; main sees what they left.
package main

import "fmt"

var log []string

var total = sum(values)

var values = []int{1, 2, 3}

var hook func() int

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func record(s string) {
	log = append(log, s)
}

func init() {
	record("init 1")
	fmt.Println("init 1: total", total, "values", len(values))
	values = append(values, 4)
}

func init() {
	defer record("init 2 deferred")
	record("init 2")
	n := 0
	hook = func() int {
		n++
		return n
	}
	fmt.Println("init 2:", hook())
}

func init() {
	record("init 3")
	total = sum(values)
	fmt.Println("init 3: total", total)
}

func main() {
	fmt.Println(hook(), total)
	for i, s := range log {
		fmt.Println(i, s)
	}
	fmt.Println(log, values)
}

// Output:
// init 1: total 6 values 3
// init 2: 1
// init 3: total 10
// 2 10
// 0 init 1
// 1 init 2
// 2 init 2 deferred
// 3 init 3
// [init 1 init 2 init 2 deferred init 3] [1 2 3 4]
