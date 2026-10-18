// phase: 05

package main

import "fmt"

func f() {
	defer func() { fmt.Println("outer recovered:", recover()) }()
	defer recover() // the deferred call is recover itself: it does not stop the panic
	panic("p")
}

func g() {
	defer func() { fmt.Println("g recovered:", recover()) }()
	defer fmt.Println("deferred print")
	defer recover()
	panic("q")
}

func main() {
	f()
	g()
}
