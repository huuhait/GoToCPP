// phase: 07

// clear, min and max on type parameters, including NaN and strings.
package main

import "fmt"

type Ordered interface {
	~int | ~int8 | ~uint8 | ~int64 | ~float32 | ~float64 | ~string
}

type Temp float64
type Tag string

func Min2[T Ordered](a, b T) T      { return min(a, b) }
func Max2[T Ordered](a, b T) T      { return max(a, b) }
func Min3[T Ordered](a, b, c T) T   { return min(a, b, c) }
func MaxOf[T Ordered](a T, rest ...T) T {
	m := a
	for _, r := range rest {
		m = max(m, r)
	}
	return m
}
func ClearS[S ~[]E, E any](s S)            { clear(s) }
func ClearM[M ~map[K]V, K comparable, V any](m M) { clear(m) }
func Clamp[T Ordered](x, lo, hi T) T       { return min(max(x, lo), hi) }
func MinConst[T ~int | ~float64](x T) T    { return min(x, 10) }
func MaxConst[T ~int | ~float64](x T) T    { return max(x, 2.5e0*2) }

func zero() float64 { return 0 }
func nan() float64  { z := zero(); return z / z }
func inf() float64  { return 1 / zero() }
func negz() float64 { return -zero() }

func main() {
	fmt.Println(Min2(3, 2), Max2(3, 2), Min2("b", "a"), Max2(Tag("b"), Tag("a")), Min3(int8(5), -3, 9), Max2(uint8(200), 100))
	fmt.Println(Min2(Temp(1.5), 0.5), Max2(float32(1.5), 2.5), MaxOf(1, 5, 3), MaxOf("a"), MaxOf(Temp(-1), -2, -0.5))
	fmt.Println(Min2(nan(), 1), Max2(1, nan()), Min2(0.0, nan()), Min3(1.0, nan(), 2.0))
	fmt.Println(1/Min2(0.0, negz()), 1/Max2(negz(), 0.0), 1/Max2(0.0, negz()))
	s := []int{1, 2, 3}
	ClearS(s)
	f := []string{"a", "b"}
	ClearS(f[:1])
	m := map[string]int{"a": 1}
	ClearM(m)
	fmt.Println(s, f, m, len(m))
	fmt.Println(Clamp(15, 0, 10), Clamp(-3, 0, 10), Clamp("m", "a", "f"), Clamp(Temp(0.5), 0, 1), MinConst(12), MinConst(4.5), MaxConst(3), MaxConst(7.5))
}

// Output:
// 2 3 a b -3 200
// 0.5 2.5 5 a -0.5
// NaN NaN NaN NaN
// -Inf +Inf +Inf
// [0 0 0] [ b] map[] 0
// 10 0 f 0.5 10 4.5 5 7.5
