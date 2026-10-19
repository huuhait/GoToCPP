// phase: 07

// comparable, any and interface-typed arguments: instances over interfaces compare by dynamic value.
package main

import "fmt"

type Eq[T comparable] struct{ a, b T }

func (e Eq[T]) Same() bool { return e.a == e.b }

type Holder[T any] struct{ v T }

type Shape interface{ Name() string }
type Sq struct{ n int }

func (s Sq) Name() string { return fmt.Sprint("sq", s.n) }

func main() {
	fmt.Println(Eq[int]{1, 1}.Same(), Eq[string]{"a", "b"}.Same(), Eq[[2]int]{[2]int{1, 2}, [2]int{1, 2}}.Same())
	fmt.Println(Eq[any]{1, 1}.Same(), Eq[any]{1, "1"}.Same(), Eq[Shape]{Sq{1}, Sq{1}}.Same(), Eq[Shape]{Sq{1}, Sq{2}}.Same())
	fmt.Println(Eq[*int]{nil, nil}.Same(), Eq[error]{}.Same())
	h := Holder[Shape]{Sq{3}}
	fmt.Println(h.v.Name(), h)
	var hs []Holder[any]
	hs = append(hs, Holder[any]{1}, Holder[any]{"x"}, Holder[any]{})
	fmt.Println(hs, len(hs))
	fmt.Printf("%T %T\n", h, hs)
	defer func() { fmt.Println("recovered:", recover()) }()
	fmt.Println(Eq[any]{[]int{1}, []int{1}}.Same())
}

// Output:
// true false true
// true false true false
// true true
// sq3 {{3}}
// [{1} {x} {<nil>}] 3
// main.Holder[main.Shape] []main.Holder[interface {}]
// recovered: runtime error: comparing uncomparable type []int
