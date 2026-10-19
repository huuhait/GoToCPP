// phase: 07

// Closures, defer and recover inside methods of generic types, capturing the receiver and type-typed values.
package main

import "fmt"

type Counter[T ~int | ~float64] struct {
	n    T
	logs []string
}

func (c *Counter[T]) Adder(d T) func() T {
	return func() T {
		c.n += d
		return c.n
	}
}

func (c *Counter[T]) Do(f func(T) T) (res T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
			c.logs = append(c.logs, "panic")
		}
	}()
	defer func() { c.logs = append(c.logs, "done") }()
	res = f(c.n)
	return
}

func (c Counter[T]) Each(xs []T, f func(T)) {
	for _, x := range xs {
		defer f(x + c.n)
	}
}

func main() {
	c := &Counter[int]{}
	add := c.Adder(5)
	fmt.Println(add(), add(), c.n)
	r, err := c.Do(func(n int) int { return n * 2 })
	fmt.Println(r, err, c.logs)
	r, err = c.Do(func(n int) int { panic("boom") })
	fmt.Println(r, err, c.logs)
	f := &Counter[float64]{n: 0.5}
	f.Each([]float64{1, 2, 3}, func(x float64) { fmt.Print(x, ";") })
	fmt.Println()
}

// Output:
// 5 10 10
// 20 <nil> [done]
// 0 recovered: boom [done done panic]
// 3.5;2.5;1.5;
