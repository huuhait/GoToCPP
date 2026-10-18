// phase: 05

package main

import "fmt"

type MyErr struct{ Code int }

func (e MyErr) Error() string { return fmt.Sprint("myerr ", e.Code) }

type Stringy struct{ N int }

func (s Stringy) String() string { return fmt.Sprint("stringy ", s.N) }

type Point struct{ X, Y int }

type Stringer interface{ String() string }

// classify runs f and says what kind of value its panic had.
func classify(name string, f func()) {
	defer func() {
		r := recover()
		switch v := r.(type) {
		case nil:
			fmt.Println(name, "-> nil")
		case MyErr:
			fmt.Println(name, "-> MyErr", v.Code)
		case error:
			fmt.Println(name, "-> error:", v.Error())
		case Stringer:
			fmt.Println(name, "-> Stringer:", v.String())
		case string:
			fmt.Println(name, "-> string:", v)
		case int:
			fmt.Println(name, "-> int:", v)
		case Point:
			fmt.Println(name, "-> Point:", v.X, v.Y)
		case *Point:
			fmt.Println(name, "-> *Point:", v.X, v.Y)
		default:
			fmt.Println(name, "-> other")
		}
	}()
	f()
}

func main() {
	var a []int
	var m map[string]int
	z := 0
	classify("string", func() { panic("s") })
	classify("int", func() { panic(42) })
	classify("custom error", func() { panic(MyErr{3}) })
	classify("stringer", func() { panic(Stringy{2}) })
	classify("struct", func() { panic(Point{1, 2}) })
	classify("pointer", func() { panic(&Point{3, 4}) })
	classify("float", func() { panic(2.5) })
	classify("index", func() { _ = a[1] })
	classify("nil map", func() { m["a"] = 1 })
	classify("divide", func() { _ = 1 / z })
	classify("nil deref", func() { var p *Point; _ = p.X })
	classify("assertion", func() { var x any = "str"; _ = x.(int) })
	classify("panic nil", func() { panic(nil) })
	classify("no panic", func() {})
}

// Output:
// string -> string: s
// int -> int: 42
// custom error -> MyErr 3
// stringer -> Stringer: stringy 2
// struct -> Point: 1 2
// pointer -> *Point: 3 4
// float -> other
// index -> error: runtime error: index out of range [1] with length 0
// nil map -> error: assignment to entry in nil map
// divide -> error: runtime error: integer divide by zero
// nil deref -> error: runtime error: invalid memory address or nil pointer dereference
// assertion -> error: interface conversion: interface {} is string, not int
// panic nil -> error: panic called with nil argument
// no panic -> nil
