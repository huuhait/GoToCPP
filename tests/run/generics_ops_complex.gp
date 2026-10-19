// phase: 07

// Complex arithmetic and comparison on a type set of both complex types.
package main

import "fmt"

type Cx interface{ ~complex64 | ~complex128 }

type Phasor complex128

func Add[T Cx](a, b T) T { return a + b }
func Sub[T Cx](a, b T) T { return a - b }
func Mul[T Cx](a, b T) T { return a * b }
func Div[T Cx](a, b T) T { return a / b }
func Neg[T Cx](a T) T    { return -a }
func Eq[T Cx](a, b T) bool { return a == b }
func Pow[T Cx](a T, n int) T {
	r := T(1)
	for range n {
		r *= a
	}
	return r
}
func Sum[S ~[]T, T Cx](s S) (t T) {
	for _, v := range s {
		t += v
	}
	return
}

func main() {
	a, b := complex(1, 2), complex(3, -1)
	fmt.Println(Add(a, b), Sub(a, b), Mul(a, b), Div(a, b), Neg(a), Eq(a, a), Eq(a, b))
	c, d := complex64(complex(1.5, 0.5)), complex64(complex(-2, 4))
	fmt.Println(Add(c, d), Sub(c, d), Mul(c, d), Div(c, d), Neg(c))
	fmt.Println(Pow(complex(0, 1), 2), Pow(complex(0, 1), 4), Pow(complex64(complex(1, 1)), 8), Pow(Phasor(complex(0, 1)), 3))
	fmt.Println(Sum([]complex128{1, 2i, 3 + 4i}), Sum([]Phasor{1, 1i}), Sum([]complex64(nil)))
	fmt.Printf("%T %T %v %.2f\n", Mul(Phasor(1i), 1i), Add(c, d), Eq(Phasor(2), 2), Div(a, b))
}

// Output:
// (4+1i) (-2+3i) (5+5i) (0.1+0.7000000000000001i) (-1-2i) true false
// (-0.5+4.5i) (3.5-3.5i) (-5+5i) (-0.05-0.35i) (-1.5-0.5i)
// (-1+0i) (1-0i) (16+0i) (-0-1i)
// (4+6i) (1+1i) (0+0i)
// main.Phasor complex64 true (0.10+0.70i)
