// phase: 05

// Go names that are C++ keywords or std names (class, new, delete, typename, register, auto, this, template, ...) in types, fields, methods and interfaces.
package main

import "fmt"

type class struct{ this, template, operator int }
type virtual interface {
	new() int
	delete(int) string
	typename() class
	register() bool
	auto()
}
type namespace struct {
	class
	friend  string
	private int
}

func (c class) new() int            { return c.this + 1 }
func (c class) delete(n int) string { return fmt.Sprint("deleted ", n+c.template) }
func (c class) typename() class     { return c }
func (c class) register() bool      { return c.operator > 0 }
func (c *class) auto()              { c.this++ }
func (n namespace) static() string  { return n.friend }

type union struct{ int }
type enum struct{ x, y int }
type signed int
type unsigned uint
type long float64
type NULL struct{}
type string_ string
type size_t int
type std struct{ vector []int }

func (e enum) volatile() int { return e.x * e.y }
func (NULL) nullptr() string { return "nullptr" }

func main() {
	c := &class{1, 2, 3}
	var v virtual = c
	v.auto()
	fmt.Println(v.new(), v.delete(5), v.typename(), v.register())
	ns := namespace{*c, "f", 7}
	fmt.Println(ns.static(), ns.new(), ns.this, ns.private)
	var u union = union{4}
	fmt.Println(u.int, enum{3, 4}.volatile(), NULL{}.nullptr())
	var s signed = -1
	var us unsigned = 2
	var l long = 1.5
	fmt.Println(s, us, l, string_("x"), size_t(3), std{[]int{1}})
	var int_ int = 3
	new := func(x int) int { return x * 2 }
	delete := map[int]int{1: 2}
	printf := func(f string, a ...any) string { return fmt.Sprintf(f, a...) }
	fmt.Println(new(int_), delete, printf("%d", int_))
	var x any = ns
	switch t := x.(type) {
	case virtual:
		fmt.Println("virtual", t)
	case interface{ static() string }:
		fmt.Println("static", t.static())
	}
	fmt.Printf("%T %T %T %T\n", c, ns, u, l)
}
