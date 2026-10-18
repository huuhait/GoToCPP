// phase: 05

// == of interface values that hold structs with interface fields, arrays of interfaces,
// uncomparable values and blank fields (step 5.6): equal when the dynamic types are identical and
// the values equal; a panic only when two values of an uncomparable dynamic type are compared, or
// when the comparison reaches an uncomparable part.
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
	a, b = Outer{Inner{1, "x"}, nil, 0, "s"}, Outer{Inner{1, "x"}, nil, 0, "s"}
	try("outer equal", func() bool { return a == b })
	b = Outer{Inner{1, "y"}, nil, 0, "s"}
	try("outer differ", func() bool { return a == b })
	e1 := newErr("e")
	a, b = Outer{e: e1}, Outer{e: e1}
	try("outer err same", func() bool { return a == b })
	b = Outer{e: newErr("e")}
	try("outer err other", func() bool { return a == b })
	a, b = Outer{i: Inner{b: []int{1}}}, Outer{i: Inner{b: []int{1}}}
	try("outer slice inside", func() bool { return a == b })
	b = Outer{i: Inner{a: 2, b: []int{1}}}
	try("outer slice inside differ first", func() bool { return a == b })
	a, b = Blank{1, "p", 2}, Blank{1, "q", 2}
	try("blank", func() bool { return a == b })
	a, b = Arr{1, "a", nil}, Arr{1, "a", nil}
	try("arr", func() bool { return a == b })
	b = Arr{1, "a", 2}
	try("arr differ", func() bool { return a == b })
	a, b = Arr{1, []int{}, nil}, Arr{1, []int{}, nil}
	try("arr slice", func() bool { return a == b })
	b = Arr{2, []int{}, nil}
	try("arr slice differ first", func() bool { return a == b })
	a, b = F{}, F{}
	try("struct with func", func() bool { return a == b })
	a, b = func() {}, func() {}
	try("func", func() bool { return a == b })
	a, b = map[int]int{}, 1
	try("map vs int", func() bool { return a == b })
	a, b = map[int]int{}, map[int]int{}
	try("map", func() bool { return a == b })
	a, b = []int{1}, []int{1}
	try("slice", func() bool { return a == b })
	a, b = [1][]int{}, [1][]int{}
	try("array of slice", func() bool { return a == b })
}

// Output:
// outer equal true
// outer differ false
// outer err same true
// outer err other false
// outer slice inside panic: runtime error: comparing uncomparable type []int
// outer slice inside differ first false
// blank true
// arr true
// arr differ false
// arr slice panic: runtime error: comparing uncomparable type []int
// arr slice differ first false
// struct with func panic: runtime error: comparing uncomparable type main.F
// func panic: runtime error: comparing uncomparable type func()
// map vs int false
// map panic: runtime error: comparing uncomparable type map[int]int
// slice panic: runtime error: comparing uncomparable type []int
// array of slice panic: runtime error: comparing uncomparable type [1][]int
