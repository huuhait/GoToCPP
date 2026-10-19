// phase: 09
// math in generic functions (Phase 07): a type-parameter constraint over float32 and float64 calling math through conversions.

package main

import (
	"fmt"
	"math"
)


type Float interface{ ~float32 | ~float64 }

func norm[T Float](x, y T) T { return T(math.Sqrt(float64(x*x + y*y))) }

func sumSqrt[T Float](xs []T) T {
	var s T
	for _, x := range xs {
		s += T(math.Sqrt(float64(x)))
	}
	return s
}

func clamp[T Float](x, lo, hi T) T { return min(max(x, lo), hi) }

func roundTo[T Float](x T, digits int) T {
	p := math.Pow10(digits)
	return T(math.Round(float64(x)*p) / p)
}

func main() {

	fmt.Println(norm(3.0, 4.0), norm(float32(1.5), float32(2.5)), sumSqrt([]float64{1, 4, 9}), sumSqrt([]float32{2, 8}))
	fmt.Println(clamp(5.5, 0.0, 3.0), clamp(float32(-1), 0, 1), roundTo(3.14159, 2), roundTo(float32(2.71828), 3))

}

// Output:
// 5 2.9154758 6 4.2426405
// 3 0 3.14 2.718
