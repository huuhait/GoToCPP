// phase: 07

// Indexing and slicing over type sets of byte strings, slices, arrays; out-of-range panics.
package main

import "fmt"

type Ints []int

func First[S ~[]E, E any](s S) E       { return s[0] }
func Last[S ~[]E, E any](s S) E        { return s[len(s)-1] }
func SetAt[S ~[]E, E any](s S, i int, v E) { s[i] = v }
func Window[S ~[]E, E any](s S, i, j int) S { return s[i:j] }
func Window3[S ~[]E, E any](s S, i, j, k int) S { return s[i:j:k] }
func Swap[S ~[]E, E any](s S, i, j int)  { s[i], s[j] = s[j], s[i] }
func ByteAt[T ~string | ~[]byte](t T, i int) byte { return t[i] }
func Tail[T ~string | ~[]byte](t T) T    { return t[1:] }
func Reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
func Ptr[S ~[]E, E any](s S, i int) *E { return &s[i] }

func try(f func()) {
	defer func() { fmt.Println("recovered:", recover()) }()
	f()
}

func main() {
	a := Ints{5, 6, 7, 8}
	fmt.Println(First(a), Last(a), Window(a, 1, 3), cap(Window(a, 1, 3)), cap(Window3(a, 1, 3, 3)))
	SetAt(a, 0, 50)
	Swap(a, 1, 2)
	fmt.Println(a)
	Reverse(a)
	fmt.Println(a, ByteAt("xyz", 2), ByteAt([]byte("q"), 0), Tail("abc"), Tail([]byte("abc")))
	p := Ptr(a, 1)
	*p = 99
	fmt.Println(a)
	try(func() { First(Ints{}) })
	try(func() { SetAt(a, 9, 1) })
	try(func() { Window(a, 3, 9) })
	try(func() { Window(a, 3, 2) })
	try(func() { Window3(a, 0, 2, 9) })
	try(func() { ByteAt("ab", 2) })
	try(func() { Tail("") })
	try(func() { Last([]string(nil)) })
}

// Output:
// 5 8 [6 7] 3 2
// [50 7 6 8]
// [8 6 7 50] 122 113 bc [98 99]
// [8 99 7 50]
// recovered: runtime error: index out of range [0] with length 0
// recovered: runtime error: index out of range [9] with length 4
// recovered: runtime error: slice bounds out of range [:9] with capacity 4
// recovered: runtime error: slice bounds out of range [3:2]
// recovered: runtime error: slice bounds out of range [::9] with capacity 4
// recovered: runtime error: index out of range [2] with length 2
// recovered: runtime error: slice bounds out of range [1:0]
// recovered: runtime error: index out of range [-1]
