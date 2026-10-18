// phase: 05

package main

import "fmt"

// A named func type: a deferred value of it is a func value too.
type H func()

func direct() {
	fmt.Println("direct recovered:", recover())
}

func mkH(tag string) H {
	return func() { fmt.Println(tag, "recovered:", recover()) }
}

type holder struct{ h H }

func (h H) Run(msg string) {
	defer h()
	panic(msg)
}

func (hd holder) Go(msg string) {
	defer hd.h()
	panic(msg)
}

func conv() {
	defer H(direct)()
	defer func() {}() // not the one that recovers
	panic("conversion")
}

func variable() {
	var h H = direct
	defer h()
	panic("variable")
}

func returned() {
	defer mkH("returned")()
	panic("returned")
}

func notRecovering() {
	defer H(func() { fmt.Println("literal sees:", func() any { return recover() }()) })()
	defer func() { fmt.Println("outer:", recover()) }()
	panic("indirect")
}

func main() {
	conv()
	variable()
	returned()
	H(direct).Run("method")
	holder{mkH("field")}.Go("field")
	notRecovering()
	var nilH H
	func() {
		defer func() { fmt.Println("nil H:", recover()) }()
		defer nilH()
		fmt.Println("body")
	}()
}
