// phase: 05

// Where a nil interface's method value panics (step 5.9): at the expression, in every place a
// method value can be: initializer, argument, field, map element, return value, a deferred
// call's callee, and through a nil embedded interface field.
package main

import "fmt"

type Greeter interface{ Greet() string }

type Holder struct {
	Greeter
	name string
}

type Inner struct{ Greeter }
type Outer struct{ *Inner }

type Real struct{}

func (Real) Greet() string { return "hello" }

func try(name string, f func()) {
	defer func() {
		r := recover()
		fmt.Printf("%s: %v\n", name, r)
	}()
	f()
}

func use(f func() string) string { return "used" }

func get(g Greeter) func() string { return g.Greet }

func main() {
	var g Greeter
	try("var", func() {
		f := g.Greet
		fmt.Println("not reached", f != nil)
	})
	try("arg", func() { fmt.Println(use(g.Greet)) })
	try("return", func() { fmt.Println(get(nil) != nil) })
	try("field", func() { _ = Holder{Greeter: g, name: "x"}.Greeter.Greet })
	try("map", func() { _ = map[string]func() string{"a": g.Greet} })
	try("slice", func() { _ = []func() string{Real{}.Greet, g.Greet} })
	try("embedded nil", func() {
		var h Holder
		f := h.Greet
		fmt.Println("not reached", f != nil)
	})
	try("embedded through nil pointer", func() {
		var o Outer
		f := o.Greet
		fmt.Println("not reached", f != nil)
	})
	try("defer value", func() {
		defer g.Greet()
		fmt.Println("not reached")
	})
	try("assigned later", func() {
		var x Greeter
		var f func() string
		x = Real{}
		f = x.Greet
		x = nil
		fmt.Println(f(), x == nil)
	})
	try("typed nil pointer is not nil", func() {
		var p *Real
		var y Greeter = p
		f := y.Greet
		fmt.Println("bound", f != nil)
		fmt.Println(f())
	})
}

// Output:
// var: runtime error: invalid memory address or nil pointer dereference
// arg: runtime error: invalid memory address or nil pointer dereference
// return: runtime error: invalid memory address or nil pointer dereference
// field: runtime error: invalid memory address or nil pointer dereference
// map: runtime error: invalid memory address or nil pointer dereference
// slice: runtime error: invalid memory address or nil pointer dereference
// embedded nil: runtime error: invalid memory address or nil pointer dereference
// embedded through nil pointer: runtime error: invalid memory address or nil pointer dereference
// defer value: runtime error: invalid memory address or nil pointer dereference
// hello true
// assigned later: <nil>
// bound true
// typed nil pointer is not nil: value method main.Real.Greet called using nil *Real pointer
