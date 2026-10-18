// phase: 05

package main

import "fmt"

func f() {
	defer func() {
		r := recover()
		fmt.Println("r == nil:", r == nil)
		fmt.Println("r:", r)
	}()
	panic(nil)
}

func g() {
	var e any
	defer func() {
		r := recover()
		fmt.Println("g: r == nil:", r == nil, "r:", r)
	}()
	panic(e)
}

func main() {
	f()
	g()
}
