// phase: 07

// Generic types over arrays and numbers: a matrix with methods returning matrices.
package main

import "fmt"

type Number interface{ ~int | ~int64 | ~float64 }

type Mat2[T Number] [2][2]T

func (m Mat2[T]) Mul(o Mat2[T]) Mat2[T] {
	var r Mat2[T]
	for i := range 2 {
		for j := range 2 {
			for k := range 2 {
				r[i][j] += m[i][k] * o[k][j]
			}
		}
	}
	return r
}

func (m Mat2[T]) Det() T { return m[0][0]*m[1][1] - m[0][1]*m[1][0] }

func Identity[T Number]() Mat2[T] { return Mat2[T]{{1, 0}, {0, 1}} }

func (m Mat2[T]) Pow(n int) Mat2[T] {
	r := Identity[T]()
	for range n {
		r = r.Mul(m)
	}
	return r
}

type Vec[T Number] struct{ X, Y T }

func (v Vec[T]) Add(o Vec[T]) Vec[T] { return Vec[T]{v.X + o.X, v.Y + o.Y} }
func (v Vec[T]) Scale(k T) Vec[T]    { return Vec[T]{v.X * k, v.Y * k} }
func (v Vec[T]) Dot(o Vec[T]) T      { return v.X*o.X + v.Y*o.Y }

func main() {
	fib := Mat2[int]{{1, 1}, {1, 0}}
	fmt.Println(fib.Pow(10), fib.Det(), Identity[float64]())
	f := Mat2[float64]{{0.5, 0}, {0, 2}}
	fmt.Println(f.Mul(f), f.Det())
	v := Vec[int]{1, 2}
	fmt.Println(v.Add(Vec[int]{3, 4}).Scale(2), v.Dot(Vec[int]{5, 6}))
	w := Vec[float64]{0.5, 0.25}
	fmt.Printf("%v %T\n", w.Scale(4), w)
}

// Output:
// [[89 55] [55 34]] -1 [[1 0] [0 1]]
// [[0.25 0] [0 4]] 1
// {8 12} 17
// {2 1} main.Vec[float64]
