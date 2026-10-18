// phase: 05

// A String method that fails with a runtime error: an index out of range, a nil dereference, a
// plain panic and a nil pointer receiver print as %!v(PANIC=String method: runtime error: ...),
// in every position and verb.
package main

import "fmt"

type Point struct{ X, Y int }

type Boom struct{ N int }

func (b Boom) String() string {
	if b.N == 1 {
		panic("boom")
	}
	if b.N == 2 {
		s := []int{1, 2, 3}
		i := 5
		return fmt.Sprint(s[i])
	}
	if b.N == 3 {
		var p *Point
		return fmt.Sprint(p.X)
	}
	if b.N == 4 {
		var m map[string]int
		m["a"] = 1
		return ""
	}
	if b.N == 5 {
		z := 0
		return fmt.Sprint(1 / z)
	}
	if b.N == 6 {
		var a any = "s"
		return fmt.Sprint(a.(int))
	}
	return "ok"
}

type Deref struct{ V int }

func (p *Deref) String() string { return "Deref(" + fmt.Sprint(p.V) + ")" }

func main() {
	fmt.Println(Boom{0}, Boom{1})
	fmt.Println([]Boom{{0}, {1}, {2}})
	fmt.Printf("%s|%v|%d|%q|%5v\n", Boom{1}, Boom{2}, Boom{1}, Boom{1}, Boom{1})
	for i := 3; i <= 6; i++ {
		fmt.Println(Boom{i})
	}
	fmt.Println(map[string]Boom{"a": {1}, "b": {0}})
	fmt.Println(&Deref{3}, struct{ D *Deref }{nil})
	var nd *Deref
	fmt.Printf("%v|%s|%8v|\n", nd, nd, nd)
	fmt.Println(struct{ B Boom }{Boom{3}}, struct{ b Boom }{Boom{3}})
}

// Output:
// ok %!v(PANIC=String method: boom)
// [ok %!v(PANIC=String method: boom) %!v(PANIC=String method: runtime error: index out of range [5] with length 3)]
// %!s(PANIC=String method: boom)|%!v(PANIC=String method: runtime error: index out of range [5] with length 3)|{1}|%!q(PANIC=String method: boom)|%!v(PANIC=String method: boom)
// %!v(PANIC=String method: runtime error: invalid memory address or nil pointer dereference)
// %!v(PANIC=String method: assignment to entry in nil map)
// %!v(PANIC=String method: runtime error: integer divide by zero)
// %!v(PANIC=String method: interface conversion: interface {} is string, not int)
// map[a:%!v(PANIC=String method: boom) b:ok]
// Deref(3) {<nil>}
// <nil>|<nil>|<nil>|
// {%!v(PANIC=String method: runtime error: invalid memory address or nil pointer dereference)} {{3}}
