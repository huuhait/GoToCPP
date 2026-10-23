// phase: 07

// A generic type declared inside a function can be embedded in another one (its fields are promoted), embeds a package-level
// type, and is a type argument of package-level generic types and functions, as is a plain type declared in the function.
package main

import "fmt"

type Base struct{ n int }

type Holder[T any] struct{ v T }

func (h Holder[T]) Get() T { return h.v }

func Keys[K comparable, V any](m map[K]V) int { return len(m) }

func main() {
	type Pair[K comparable, V any] struct {
		k K
		v V
	}
	type Wrap[T any] struct {
		Pair[string, T]
		Base
		extra int
	}
	type Local struct{ a int }

	w := Wrap[float64]{Pair[string, float64]{"k", 1.5}, Base{7}, 3}
	fmt.Println(w.k, w.v, w.n, w.extra)

	h := Holder[Pair[int, int]]{Pair[int, int]{1, 2}}
	fmt.Println(h.Get(), h.Get().k)
	hl := Holder[Local]{Local{5}}
	fmt.Println(hl.Get().a)
	hh := Holder[Holder[Wrap[bool]]]{}
	fmt.Println(hh.Get().Get().extra)

	m := map[Pair[Local, int]]string{{Local{1}, 2}: "x"}
	fmt.Println(Keys(m), m[Pair[Local, int]{Local{1}, 2}])
	fmt.Println(Pair[Local, Pair[int, string]]{Local{1}, Pair[int, string]{2, "s"}})
}

// Output:
// k 1.5 7 3
// {1 2} 1
// 5
// 0
// 1 x
// {{1} {2 s}}
