// phase: 05

// Types declared in functions: two types with one name in two functions are two types; local
// interfaces and local types that implement package interfaces (step 5.4).
package main

import "fmt"

type Maker interface{ Make() any }

func first() any {
	type T struct{ n int }
	return T{1}
}

func second() any {
	type T struct{ n int }
	return T{1}
}

func third() Maker {
	type local struct{ v string }
	return maker{local{"x"}}
}

type maker struct{ l any }

func (m maker) Make() any { return m.l }

func main() {
	a, b := first(), second()
	fmt.Println(a == b)
	_, ok := a.(struct{ n int })
	fmt.Println(ok)
	a2 := first()
	fmt.Println(a == a2)
	type Local interface{ Hello() string }
	var l Local
	fmt.Println(l == nil)
	m := third()
	fmt.Println(m.Make() != nil)
	type pt struct{ x, y int }
	var p any = pt{1, 2}
	if q, ok := p.(pt); ok {
		fmt.Println(q.x + q.y)
	}
	switch p.(type) {
	case pt:
		fmt.Println("pt")
	}
}

// Output:
// false
// false
// true
// true
// true
// 3
// pt
