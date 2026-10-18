// phase: 07

// Operations on a type parameter whose core type is a slice: S ~[]E.
package main

import "fmt"

func Clone[S ~[]E, E any](s S) S {
	if s == nil {
		return nil
	}
	return append(S(nil), s...)
}

func Grow[S ~[]E, E any](s S, n int) S {
	out := make(S, len(s), len(s)+n)
	copy(out, s)
	return out
}

func Insert[S ~[]E, E any](s S, i int, vs ...E) S {
	tail := Clone(s[i:])
	s = append(s[:i], vs...)
	return append(s, tail...)
}

func Delete[S ~[]E, E any](s S, i int) S { return append(s[:i], s[i+1:]...) }

func Reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func Last[S ~[]E, E any](s S) (E, bool) {
	if len(s) == 0 {
		var z E
		return z, false
	}
	return s[len(s)-1], true
}

func Window[S ~[]E, E any](s S, n int) []S {
	var out []S
	for i := 0; i+n <= len(s); i++ {
		out = append(out, s[i:i+n])
	}
	return out
}

func Fill[S ~[]E, E any](s S, v E) {
	for i := range s {
		s[i] = v
	}
	clear(s[:0])
}

func Equal[S ~[]E, E comparable](a, b S) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func main() {
	a := []int{1, 2, 3}
	b := Clone(a)
	b[0] = 99
	fmt.Println(a, b, Clone([]string(nil)) == nil)
	g := Grow(a, 10)
	fmt.Println(len(g), cap(g) >= 13, g)
	fmt.Println(Insert([]int{1, 2, 5}, 2, 3, 4), Insert([]string{"a"}, 0, "x"), Insert([]int{}, 0))
	fmt.Println(Delete([]int{1, 2, 3, 4}, 1), Delete([]string{"a", "b"}, 1))
	r := []float64{1.5, 2.5, 3.5}
	Reverse(r)
	fmt.Println(r)
	fmt.Println(Last(a))
	fmt.Println(Last([]string(nil)))
	fmt.Println(Window([]int{1, 2, 3, 4}, 2), len(Window([]int{1}, 3)))
	f := make([]bool, 3)
	Fill(f, true)
	fmt.Println(f, Equal(a, []int{1, 2, 3}), Equal(a, b), Equal([]string{"a"}, []string{"a"}))
}

// Output:
// [1 2 3] [99 2 3] true
// 3 true [1 2 3]
// [1 2 3 4 5] [x a] []
// [1 3 4] [a]
// [3.5 2.5 1.5]
// 3 true
//  false
// [[1 2] [2 3] [3 4]] 0
// [true true true] true false true
