// phase: 08

// The built-in functions on type parameters: len, cap, copy, append, delete, clear, close, make,
// new, min, max, complex, real and imag, through core types ~[]E, ~[N]E, *[N]E, ~map[K]V, ~chan E,
// ~string, with defined types as arguments; the results and the panics.
package main

import "fmt"

type Ints []int
type Set map[string]struct{}
type Chan chan float64
type Arr [4]int8
type Word string

func Sizes[S ~[]E, E any](s S) (int, int) { return len(s), cap(s) }

func ArrSizes[A ~[4]E, E any](a A) (int, int) { return len(a), cap(a) }

func PtrSizes[P ~*[4]E, E any](p P) (int, int) { return len(p), cap(p) }

func Copy[D, S ~[]E, E any](d D, s S) int { return copy(d, s) }

func Grow[S ~[]E, E any](s S, n int) S {
	for i := 0; i < n; i++ {
		var z E
		s = append(s, z)
	}
	return s
}

func Del[M ~map[K]V, K comparable, V any](m M, ks ...K) int {
	for _, k := range ks {
		delete(m, k)
	}
	return len(m)
}

func ClearSlice[S ~[]E, E any](s S) { clear(s) }

func ClearMap[M ~map[K]V, K comparable, V any](m M) { clear(m) }

func Make[S ~[]E, E any](n, c int) S { return make(S, n, c) }

func MakeMap[M ~map[K]V, K comparable, V any](n int) M { return make(M, n) }

func MakeChan[C ~chan E, E any](n int) C { return make(C, n) }

func Fan[C ~chan E, E any](c C, es ...E) (got []E) {
	for _, e := range es {
		c <- e
	}
	close(c)
	for e := range c {
		got = append(got, e)
	}
	return
}

func MinMax[T ~int | ~float64 | ~string](a, b, c T) (T, T) { return min(a, b, c), max(a, b, c) }

func Cplx[F ~float32 | ~float64](re, im F) (complex128, F, F) {
	var c complex128 = complex128(complex(float64(re), float64(im)))
	return c, F(real(c)), F(imag(c))
}

func Strlen[T ~string](s T) (int, byte) { return len(s), s[0] }

func NewOf[T any](v T) *T {
	p := new(T)
	*p = v
	return p
}

func try(name string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(name, "panics:", r)
		}
	}()
	f()
}

func main() {
	fmt.Println(Sizes(make(Ints, 2, 5)))
	fmt.Println(Sizes([]string(nil)))
	fmt.Println(ArrSizes(Arr{}))
	fmt.Println(PtrSizes(&[4]string{}))
	dst := make([]int, 3)
	fmt.Println(Copy(dst, Ints{7, 8, 9, 10}), dst, Copy(Ints(nil), dst))
	fmt.Println(Grow(Ints{1}, 3), len(Grow([]string{}, 2)))
	s := Set{"a": {}, "b": {}, "c": {}}
	fmt.Println(Del(s, "a", "x"), Del(map[int]bool{1: true}, 1, 1))
	ms := map[int]string{1: "x", 2: "y"}
	ClearMap(ms)
	sl := Ints{1, 2, 3}
	ClearSlice(sl)
	fmt.Println(len(ms), sl)
	m1 := map[string]int{"a": 1}
	ClearMap(m1)
	fmt.Println(m1, len(Make[Ints](2, 10)), cap(Make[[]string](0, 7)))
	mm := MakeMap[Set](4)
	mm["k"] = struct{}{}
	fmt.Println(mm, len(MakeMap[map[int]int](0)))
	fmt.Println(Fan(MakeChan[Chan](3), 1.5, 2.5), Fan(MakeChan[chan string](1), "only"))
	fmt.Println(MinMax(3, 1, 2))
	fmt.Println(MinMax(2.5, -1.0, 7.25))
	fmt.Println(MinMax("b", "ab", "c"))
	fmt.Println(MinMax[Word]("q", "r", "p"))
	fmt.Println(Cplx(float32(1.5), 2.5))
	fmt.Println(Strlen("héllo"))
	fmt.Println(*NewOf(Arr{1, 2}), *NewOf[any](nil), *NewOf("s"))
	try("nil map write", func() { var m Set; m["x"] = struct{}{} })
	try("make negative", func() { n := -1; Make[Ints](n, 5) })
	try("make cap<len", func() { n := 3; Make[Ints](n, 1) })
	try("close nil", func() { var c Chan; close(c) })
	try("close twice", func() { c := make(Chan); close(c); close(c) })
	try("send closed", func() { c := make(Chan, 1); close(c); c <- 1 })
	try("empty string index", func() { Strlen("") })
}

// Output:
// 2 5
// 0 0
// 4 4
// 4 4
// 3 [7 8 9] 0
// [1 0 0 0] 2
// 2 0
// 0 [0 0 0]
// map[] 2 7
// map[k:{}] 0
// [1.5 2.5] [only]
// 1 3
// -1 7.25
// ab c
// p r
// (1.5+2.5i) 1.5 2.5
// 6 104
// [1 2 0 0] <nil> s
// nil map write panics: assignment to entry in nil map
// make negative panics: runtime error: makeslice: len out of range
// make cap<len panics: runtime error: makeslice: cap out of range
// close nil panics: close of nil channel
// close twice panics: close of closed channel
// send closed panics: send on closed channel
// empty string index panics: runtime error: index out of range [0] with length 0
