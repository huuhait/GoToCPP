// phase: 07

// Floating point type arguments: float32 rounding, NaN, infinities and comparisons.
package main

import "fmt"

type Float interface{ ~float32 | ~float64 }

type Meters float32

func Add[T Float](a, b T) T { return a + b }
func Mul[T Float](a, b T) T { return a * b }

func Lt[T Float](a, b T) bool { return a < b }
func Eq[T Float](a, b T) bool { return a == b }

func Sum[T Float](xs ...T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}

func IsNaN[T Float](x T) bool { return x != x }

func Clamp[T Float](x, lo, hi T) T { return min(max(x, lo), hi) }

func ToInt[T Float](x T) int { return int(x) }

func main() {
	fmt.Println(Add(0.1, 0.2), Add[float32](0.1, 0.2), Add[Meters](0.1, 0.2))
	fmt.Println(Mul(1e200, 1e200), Mul[float32](1e30, 1e30), Mul[float32](3, 1.0/3))
	zero := 0.0
	nan := zero / zero
	inf := 1 / zero
	fmt.Println(IsNaN(nan), IsNaN(1.5), IsNaN(float32(nan)), Eq(nan, nan), Lt(nan, 1), Lt(1.0, 2))
	fmt.Println(Sum(0.1, 0.2, 0.3), Sum[float32](0.1, 0.2, 0.3), Sum[float64]())
	fmt.Println(Clamp(5.5, 1, 3), Clamp(-1.0, 0, 1), Clamp[float32](0.5, 0, 1))
	fmt.Println(ToInt(3.99), ToInt(-3.99), ToInt(Meters(2.5)), ToInt[float32](1e9))
	fmt.Println(Add(inf, 1), Add(inf, -inf) != Add(inf, -inf), Lt(-inf, inf), Mul(inf, 0) != 0)
	fmt.Printf("%.3f %.3f %T\n", Add[float32](1.0005, 0), Add(1.0005, 0), Add[Meters](1, 2))
}

// Output:
// 0.30000000000000004 0.3 0.3
// +Inf +Inf 1
// true false true false false true
// 0.6000000000000001 0.6 0
// 3 0 0.5
// 3 -3 2 1000000000
// +Inf true true true
// 1.000 1.000 main.Meters
