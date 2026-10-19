// phase: 07

// Method values and method expressions of instances: bound receivers, (*T).M, T.M, stored and passed.
package main

import "fmt"

type Counter[T ~int | ~float64] struct{ n T }

func (c *Counter[T]) Inc(d T) T { c.n += d; return c.n }
func (c Counter[T]) Value() T   { return c.n }

func Apply[T any](f func(T) T, v T, times int) T {
	for range times {
		v = f(v)
	}
	return v
}

func main() {
	c := &Counter[int]{}
	inc := c.Inc
	fmt.Println(inc(2), inc(3), c.Value())
	val := c.Value
	c.n = 100
	fmt.Println(val(), c.Value())
	incExpr := (*Counter[int]).Inc
	valExpr := Counter[int].Value
	fmt.Println(incExpr(c, 1), valExpr(*c))
	f := &Counter[float64]{}
	fmt.Println(Apply(func(d float64) float64 { return f.Inc(d) }, 0.5, 3))
	g := f.Inc
	fmt.Println(g(1), Apply(g, 1, 2))
	fs := []func(int) int{c.Inc, (&Counter[int]{n: 7}).Inc}
	fmt.Println(fs[0](1), fs[1](1))
	fmt.Printf("%T %T\n", inc, valExpr)
}

// Output:
// 2 5 5
// 5 100
// 101 101
// 2
// 3 8
// 102 8
// func(int) int func(main.Counter[int]) int
