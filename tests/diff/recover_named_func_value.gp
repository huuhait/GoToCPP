// phase: 05

package main

import "fmt"

func handler() {
	fmt.Println("handler recovered:", recover())
}

func notHandler() {
	handler() // calls the aware function normally: nil
}

var table = map[string]func(){"h": handler, "n": notHandler}

func viaVariable() {
	h := handler
	defer h()
	panic("variable")
}

func viaTable(key string) {
	defer table[key]()
	panic("table " + key)
}

func viaField() {
	s := struct{ f func() }{handler}
	defer s.f()
	panic("field")
}

func main() {
	viaVariable()
	viaTable("h")
	viaField()
	func() {
		defer func() { fmt.Println("outer:", recover()) }()
		viaTable("n")
	}()
}
