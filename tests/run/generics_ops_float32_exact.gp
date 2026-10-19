// phase: 07

// float32 and float64 arithmetic in generic code must round exactly like the typed code.
package main

import "fmt"

type Float interface{ ~float32 | ~float64 }

func zero() float64 { return 0 }

func Sum[T Float](n int, step T) T {
	var s T
	for range n {
		s += step
	}
	return s
}

func Mean[S ~[]T, T Float](s S) T {
	var t T
	for _, v := range s {
		t += v
	}
	return t / T(len(s))
}

func Hyp2[T Float](a, b T) T { return a*a + b*b }
func Lerp[T Float](a, b, t T) T { return a + (b-a)*t }
func Horner[T Float](x T, cs ...T) T {
	var r T
	for _, c := range cs {
		r = r*x + c
	}
	return r
}
func Div[T Float](a, b T) T { return a / b }
func Floor[T Float](x T) T  { return T(int64(x)) }
func Inv[T Float](x T) T    { return 1 / x }

func main() {
	fmt.Println(Sum[float32](10, 0.1), Sum[float64](10, 0.1), Sum[float32](1000, 0.01), Sum[float64](1000, 0.01))
	fmt.Println(Mean([]float32{1.1, 2.2, 3.3}), Mean([]float64{1.1, 2.2, 3.3}))
	fmt.Println(Hyp2[float32](1.1, 2.2), Hyp2(1.1, 2.2), Lerp[float32](0, 1, 0.3), Lerp(0.0, 1, 0.3))
	fmt.Println(Horner[float32](0.1, 1, 2, 3, 4), Horner(0.1, 1, 2, 3, 4))
	fmt.Println(Div[float32](1, 3), Div(1.0, 3), Div[float32](16777216, 3), Div(float32(zero()), 0), Div(1, zero()), Div(-1, zero()))
	fmt.Println(Floor[float32](3.99), Floor(-3.99), Inv[float32](3), Inv(3.0), Inv(zero()))
	var big float32 = 16777216
	fmt.Println(Sum(2, big+1), big+1 == big, Lerp(big, big+2, 0.5))
	fmt.Printf("%v %g %.3f %e %8.2f|\n", Inv[float32](7), Inv[float32](7), Inv(7.0), Inv[float32](7), Inv[float32](7))
}

// Output:
// 1.0000001 0.9999999999999999 10.0001335 9.999999999999831
// 2.2 2.1999999999999997
// 6.05 6.050000000000001 0.3 0.3
// 4.321 4.321
// 0.33333334 0.3333333333333333 5.5924055e+06 NaN +Inf -Inf
// 3 -3 0.33333334 0.3333333333333333 +Inf
// 3.3554432e+07 true 1.6777216e+07
// 0.14285715 0.14285715 0.143 1.428571e-01     0.14|
