// Generics: a constraint interface with type terms, a generic function with inferred and
// explicit type arguments, a generic type with a method, and a generic higher-order function
// (the shape slices.Sort and cmp use in Go 1.25's sort package).
package main

type Number interface {
	~int | ~int64 | ~float64
}

func add[T Number](a, b T) T { return a + b }

type Pair[T any] struct {
	First, Second T
}

func (p Pair[T]) Swap() Pair[T] { return Pair[T]{p.Second, p.First} }

func mapSlice[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func main() {
	println(add(1, 2), add[int64](3, 4), add(1.5, 2.5))
	p := Pair[string]{"a", "b"}
	q := p.Swap()
	println(q.First, q.Second)
	lengths := mapSlice([]string{"go", "cpp"}, func(s string) int { return len(s) })
	println(lengths[0], lengths[1])
	pi := Pair[int]{First: 1, Second: 2}
	println(pi.First + pi.Second)
}
