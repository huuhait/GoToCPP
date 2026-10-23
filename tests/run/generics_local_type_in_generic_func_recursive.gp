// phase: 07

// A generic type declared inside a generic function can refer to itself and to the function's type parameters, be a map key,
// a slice element, and be declared in a nested literal of the generic function.
package main

import "fmt"

func Show[T any](v T) string { return fmt.Sprintf("%T", v) }

func Chain[T any](xs []T) any {
	type List[U any] struct {
		head U
		tail *List[U]
		tag  T
	}
	var l *List[int]
	for i, x := range xs {
		l = &List[int]{i, l, x}
	}
	n := 0
	for p := l; p != nil; p = p.tail {
		n += p.head
	}
	fmt.Println("sum", n, l.tag, Show(*l))
	return *l
}

func Mixed[K, V comparable](k K, v V) {
	type Entry[W any] struct {
		key K
		val V
		aux W
	}
	m := map[Entry[bool]]int{}
	m[Entry[bool]{k, v, true}]++
	m[Entry[bool]{k, v, true}]++
	fmt.Println(len(m), m[Entry[bool]{k, v, true}])
	f := func() Entry[int] {
		type Inner[X any] struct{ e Entry[X] }
		in := Inner[int]{Entry[int]{k, v, 7}}
		fmt.Println(Show(in), in.e.aux)
		return in.e
	}
	fmt.Println(f().key, f().val)
	var a any = Entry[string]{k, v, "s"}
	switch x := a.(type) {
	case Entry[int]:
		fmt.Println("int", x)
	case Entry[string]:
		fmt.Println("string", x.aux)
	}
}

func main() {
	fmt.Println(Show(Chain([]string{"x", "y", "z"})))
	Chain([]float64{1.5})
	Mixed("k", 1.5)
	Mixed(2, "v")
}

// Output:
// sum 3 z main.List[string;int]
// main.List[string;int]
// sum 0 1.5 main.List[float64;int]
// 1 2
// main.Inner[string,float64;int] 7
// main.Inner[string,float64;int] 7
// k 1.5
// string s
// 1 2
// main.Inner[int,string;int] 7
// main.Inner[int,string;int] 7
// 2 v
// string s
