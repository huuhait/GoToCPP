// phase: 05

package main

import "fmt"

var atInit = recover()

var viaFunc = func() any { return recover() }()

func main() {
	fmt.Println(atInit == nil, viaFunc == nil)
	defer func() {
		fmt.Println("deferred:", recover())
	}()
	panic("p")
}
