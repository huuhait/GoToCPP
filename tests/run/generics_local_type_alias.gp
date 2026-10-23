// phase: 07

// A generic alias declared inside a function is the same type as what it names: of a slice, and of an instance of a generic
// type declared in the same (generic) function.
package main

import "fmt"

func G[T any](v T) {
	type Box[U any] struct {
		u U
		t T
	}
	type Boxed[U any] = Box[U]
	type List[U any] = []Box[U]
	var l List[int] = []Box[int]{{1, v}}
	var b Boxed[string] = Box[string]{"s", v}
	l = append(l, Box[int]{2, v})
	fmt.Printf("%v %v %T %T\n", l, b, l, b)
}

func main() {
	type Pair[K comparable, V any] struct {
		k K
		v V
	}
	type IntPair[V any] = Pair[int, V]
	var p IntPair[string] = Pair[int, string]{1, "x"}
	fmt.Printf("%v %T\n", p, p)
	G(true)
	G(2.5)
}

// Output:
// {1 x} main.Pair[int,string]
// [{1 true} {2 true}] {s true} []main.Box[bool;int] main.Box[bool;string]
// [{1 2.5} {2 2.5}] {s 2.5} []main.Box[float64;int] main.Box[float64;string]
