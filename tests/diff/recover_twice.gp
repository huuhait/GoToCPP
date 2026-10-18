// phase: 05

package main

import "fmt"

func f() {
	defer func() {
		a := recover()
		b := recover()
		fmt.Println(a, b, b == nil)
	}()
	panic("once")
}

func g() {
	defer func() {
		fmt.Println("second deferred:", recover())
	}()
	defer func() {
		fmt.Println("first deferred:", recover())
	}()
	panic("only one")
}

func main() {
	f()
	g()
}
