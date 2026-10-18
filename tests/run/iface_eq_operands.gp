// phase: 05

// == between an interface and a non-interface operand, nil rules and NaN (step 5.6).
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
	var a, b any
	// interface vs non-interface operands
	var i any = 3
	try("i == 3", func() bool { return i == 3 })
	try("i == 3.0", func() bool { return i == 3.0 })
	try("i == int64(3)", func() bool { return i == int64(3) })
	try("i == Named", func() bool { return i == Named(3) })
	i = Blank{1, "", 2}
	try("i == Blank", func() bool { return i == Blank{1, "z", 2} })
	var s Str = Named(3)
	try("s == Named", func() bool { return s == Named(3) })
	try("s == i", func() bool { return Str(Named(3)) == any(Named(3)) })
	var e error
	try("e == nil", func() bool { return e == nil })
	var ne *Named
	s = ne
	try("typed nil", func() bool { return s == nil })
	try("typed nil ptr", func() bool { return s == Str(ne) })
	// float / NaN inside interfaces
	nan := 0.0
	nan = nan / nan
	a, b = nan, nan
	try("nan", func() bool { return a == b })
	a, b = 0.0, -nan*0
	try("zeros", func() bool { return a == b })
	a, b = complex(1, 2), complex(1, 2)
	try("complex", func() bool { return a == b })
	a, b = [2]float64{nan, 1}, [2]float64{nan, 1}
	try("array nan", func() bool { return a == b })
}

// Output:
// i == 3 true
// i == 3.0 false
// i == int64(3) false
// i == Named false
// i == Blank true
// s == Named true
// s == i true
// e == nil true
// typed nil false
// typed nil ptr true
// nan false
// zeros false
// complex true
// array nan false
