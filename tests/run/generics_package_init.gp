// phase: 07

// Package-level variables of generic types and initialized by generic functions: the order of
// initialization follows dependencies, init functions use instances, generic constructors fill
// tables, and a package-level func value is an instance.
package main

import "fmt"

type Registry[K comparable, V any] struct {
	items map[K]V
	order []K
}

func NewRegistry[K comparable, V any]() *Registry[K, V] {
	fmt.Println("NewRegistry", fmt.Sprintf("%T", (*Registry[K, V])(nil)))
	return &Registry[K, V]{items: map[K]V{}}
}

func (r *Registry[K, V]) Add(k K, v V) *Registry[K, V] {
	if _, ok := r.items[k]; !ok {
		r.order = append(r.order, k)
	}
	r.items[k] = v
	return r
}

func (r *Registry[K, V]) Each(f func(K, V)) {
	for _, k := range r.order {
		f(k, r.items[k])
	}
}

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}

var (
	names   = NewRegistry[string, int]().Add("one", one).Add("two", two)
	one     = trace("one", 1)
	two     = trace("two", one+1)
	squares = Map([]int{one, two, three}, func(i int) int { return i * i })
	three   = trace("three", two+1)
	labels  = Map(squares, label[int])
	double  = apply[int8]
	maxInt8 = Max[int8](100, 27)
	pairs   = Pairs[string, float64]{{"pi", 3.14}, {"e", 2.72}}
)

type Pairs[K, V any] []struct {
	K K
	V V
}

func trace(name string, v int) int {
	fmt.Println("init var", name, v)
	return v
}

func label[T any](v T) string { return fmt.Sprintf("<%T:%v>", v, v) }

func apply[T ~int8 | ~int](x T) T { return x * 2 }

func Max[T int8 | int](a, b T) T { return max(a, b) }

func init() {
	fmt.Println("init 1", squares, labels)
	names.Add("three", three)
}

func init() {
	fmt.Println("init 2", double(100), maxInt8, double(maxInt8))
	var other = NewRegistry[int, []string]().Add(1, []string{"a"})
	other.Each(func(k int, v []string) { fmt.Println(k, v) })
}

func main() {
	names.Each(func(k string, v int) { fmt.Println(k, v) })
	for _, p := range pairs {
		fmt.Println(p.K, p.V)
	}
	fmt.Printf("%T %T\n", names, pairs)
}

// Output:
// init var one 1
// init var two 2
// NewRegistry *main.Registry[string,int]
// init var three 3
// init 1 [1 4 9] [<int:1> <int:4> <int:9>]
// init 2 -56 100 -56
// NewRegistry *main.Registry[int,[]string]
// 1 [a]
// one 1
// two 2
// three 3
// pi 3.14
// e 2.72
// *main.Registry[string,int] main.Pairs[string,float64]
