// phase: 05

// switch on an interface value with constants of different types in the cases (step 5.6): a
// case matches only a value of the same type and value; nil matches the nil interface; an
// uncomparable value panics when it reaches a case of its own type.
package main

import "fmt"

type myErr struct{ s string }

func (e *myErr) Error() string { return e.s }

func newErr(s string) error { return &myErr{s} }

type Inner struct {
	a int
	b any
}
type Outer struct {
	i Inner
	e error
	_ int
	s string
}
type Blank struct {
	x int
	_ string
	y float64
}
type Arr [3]any
type F struct{ f func() }
type Named int
type Str interface{ String() string }

func (n Named) String() string { return fmt.Sprint("N", int(n)) }

func try(name string, f func() bool) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(name, "panic:", r)
		}
	}()
	fmt.Println(name, f())
}

func main() {
	// switch with mixed constants
	for _, x := range []any{1, "a", nil, 2.5, int8(1), 'x', true, Named(1)} {
		switch x {
		case 1, "a", nil:
			fmt.Println("c1", x)
		case 2.5:
			fmt.Println("c2", x)
		case int8(1):
			fmt.Println("int8")
		case 'x':
			fmt.Println("rune")
		case true:
			fmt.Println("true")
		default:
			fmt.Println("default", x)
		}
	}
	var u any = []int{1}
	switch u {
	case 1:
		fmt.Println("one")
	default:
		fmt.Println("def")
	}
	try("switch uncomparable", func() bool {
		switch u {
		case u:
			return true
		}
		return false
	})
}

// Output:
// c1 1
// c1 a
// c1 <nil>
// c2 2.5
// int8
// rune
// true
// default N1
// def
// switch uncomparable panic: runtime error: comparing uncomparable type []int
