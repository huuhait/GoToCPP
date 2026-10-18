// phase: 05

package main

import "fmt"

// A deferred closure changes a named result after a panic: the function returns it.
func parse(s string) (n int, msg string) {
	defer func() {
		if r := recover(); r != nil {
			n = -1
			msg = fmt.Sprint("recovered: ", r)
		}
	}()
	if s == "" {
		panic("empty")
	}
	n = len(s)
	msg = "ok"
	return
}

// The deferred call runs after "return 4" set the result: it can change it.
func double() (n int) {
	defer func() { n *= 2 }()
	return 4
}

// Without a recover the result is as the return set it; with one it is what the
// function had when the panic started.
func partial() (a int, b string) {
	defer func() { recover() }()
	a = 7
	panic("p")
}

// An unnamed result is its zero value after a recovered panic.
func unnamed() int {
	defer func() { recover() }()
	panic("p")
}

func unnamedTwo() (int, string) {
	defer func() { recover() }()
	panic("p")
}

func main() {
	fmt.Println(parse("hello"))
	fmt.Println(parse(""))
	fmt.Println(double())
	fmt.Println(partial())
	fmt.Println(unnamed())
	fmt.Println(unnamedTwo())
}
