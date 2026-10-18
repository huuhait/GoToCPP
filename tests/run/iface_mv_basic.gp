// phase: 05

// Method values of interface methods (step 5.9): a value or pointer receiver in the dynamic
// type, arguments, several results, a variadic method, and the func value's type.
package main

import "fmt"

type Shape interface {
	Area() int
	Name(prefix string) string
}

type Sq struct{ s int }

func (q Sq) Area() int            { return q.s * q.s }
func (q Sq) Name(p string) string { return p + "square" }

type Ct struct{ n int }

func (c *Ct) Area() int            { c.n++; return c.n }
func (c *Ct) Name(p string) string { return fmt.Sprint(p, "counter", c.n) }

type Summer interface {
	Sum(xs ...int) (int, string)
	Pair() (int, string, bool)
}

type Adder struct{ base int }

func (a Adder) Sum(xs ...int) (int, string) {
	t := a.base
	for _, x := range xs {
		t += x
	}
	return t, fmt.Sprint(len(xs), " items")
}

func (a Adder) Pair() (int, string, bool) { return a.base, "pair", a.base > 0 }

func main() {
	var s Shape = Sq{3}
	area := s.Area
	name := s.Name
	fmt.Println(area(), name("a "), name(">"))

	s = &Ct{}
	next := s.Area
	fmt.Println(next(), next(), next())
	fmt.Println(s.Name("c:"))

	var m Summer = Adder{10}
	sum := m.Sum
	fmt.Println(sum())
	fmt.Println(sum(1))
	fmt.Println(sum(1, 2, 3))
	fmt.Println(sum([]int{4, 5}...))
	a, b := sum(7, 7)
	fmt.Println(a, b)
	pair := m.Pair
	n, str, ok := pair()
	fmt.Println(n, str, ok)

	var f func() int = s.Area
	var g func(string) string = s.Name
	fmt.Println(f(), g("g:"))
}

// Output:
// 9 a square >square
// 1 2 3
// c:counter3
// 10 0 items
// 11 1 items
// 16 3 items
// 19 2 items
// 24 2 items
// 10 pair true
// 4 g:counter4
