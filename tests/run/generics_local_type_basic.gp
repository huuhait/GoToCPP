// phase: 07

// A generic type declared inside a function, in a function that is not generic: each instance is a type of its own, it has
// the zero value, == and map keys of a struct, and fmt prints it like a package-level generic type.
package main

import "fmt"

func main() {
	type Box[T any] struct {
		v    T
		next *Box[T]
	}
	type Pair[K comparable, V any] struct {
		Key K
		Val V
	}

	var zero Box[int]
	fmt.Println(zero, zero.next == nil)

	b := Box[string]{v: "s"}
	c := Box[[]int]{v: []int{1, 2}, next: &Box[[]int]{}}
	fmt.Printf("%v %+v\n", b, b)
	fmt.Println(len(c.v), c.next.v == nil)

	p := Pair[string, float64]{"pi", 3.14}
	q := Pair[string, float64]{"pi", 3.14}
	fmt.Println(p == q, p)
	fmt.Printf("%+v %#v %T\n", p, p, p)

	counts := map[Pair[int, bool]]int{}
	counts[Pair[int, bool]{1, true}]++
	counts[Pair[int, bool]{1, true}]++
	counts[Pair[int, bool]{1, false}]++
	fmt.Println(len(counts), counts[Pair[int, bool]{1, true}])

	ps := []Pair[int, int]{{1, 2}, {3, 4}}
	arr := [2]Pair[int, string]{{1, "a"}}
	fmt.Println(ps, arr)
}

// Output:
// {0 <nil>} true
// {s <nil>} {v:s next:<nil>}
// 2 true
// true {pi 3.14}
// {Key:pi Val:3.14} main.Pair[string,float64]{Key:"pi", Val:3.14} main.Pair[string,float64]
// 2 2
// [{1 2} {3 4}] [{1 a} {0 }]
