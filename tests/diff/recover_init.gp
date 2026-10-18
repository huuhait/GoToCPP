// phase: 05

package main

import "fmt"

var state = "initial"

func init() {
	defer func() {
		state = fmt.Sprint("init recovered ", recover())
	}()
	panic("in init")
}

func init() {
	fmt.Println("second init sees:", state)
}

func main() {
	defer func() { fmt.Println("main recovered:", recover()) }()
	fmt.Println("main sees:", state)
	var a []int
	_ = a[0]
}
