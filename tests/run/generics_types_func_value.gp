// phase: 07

// Instances in function values, closures stored in maps and slices, and methods passed as callbacks.
package main

import "fmt"

type Op[T any] struct {
	Name string
	Fn   func(a, b T) T
}

type Calc[T ~int | ~float64] struct {
	ops map[string]Op[T]
	log []string
}

func NewCalc[T ~int | ~float64]() *Calc[T] {
	c := &Calc[T]{ops: map[string]Op[T]{}}
	c.Register(Op[T]{"add", func(a, b T) T { return a + b }})
	c.Register(Op[T]{"mul", func(a, b T) T { return a * b }})
	return c
}

func (c *Calc[T]) Register(o Op[T]) { c.ops[o.Name] = o }

func (c *Calc[T]) Do(name string, a, b T) (T, bool) {
	o, ok := c.ops[name]
	if !ok {
		var z T
		return z, false
	}
	c.log = append(c.log, name)
	return o.Fn(a, b), true
}

func Fold[T any](xs []T, init T, f func(T, T) T) T {
	for _, x := range xs {
		init = f(init, x)
	}
	return init
}

func main() {
	c := NewCalc[int]()
	fmt.Println(c.Do("add", 2, 3))
	fmt.Println(c.Do("mul", 4, 5))
	fmt.Println(c.Do("pow", 1, 1))
	d := NewCalc[float64]()
	d.Register(Op[float64]{"div", func(a, b float64) float64 { return a / b }})
	fmt.Println(d.Do("div", 1, 4))
	do := d.Do
	fmt.Println(do("mul", 1.5, 2))
	fmt.Println(Fold([]int{1, 2, 3}, 0, c.ops["add"].Fn), Fold([]float64{1, 2}, 1, d.ops["mul"].Fn))
	fmt.Println(c.log, d.log)
}

// Output:
// 5 true
// 20 true
// 0 false
// 0.25 true
// 3 true
// 6 2
// [add mul] [div mul]
