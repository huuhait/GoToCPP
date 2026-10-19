// phase: 08

// Operations through core types: ~[]E, ~map[K]V, ~chan E, ~[N]E, ~string | ~[]byte, ~func; type
// parameters constrained by methods plus a core type, with defined types as type arguments.
package main

import "fmt"

type Ints []int
type Names map[string]int
type Pipe chan string
type Grid [3]float64
type Text string
type Raw []byte
type Op func(int) int

func Len[S ~[]E, E any](s S) int { return len(s) + cap(s) }
func First[S ~[]E, E any](s S) (e E, ok bool) {
	if len(s) > 0 {
		return s[0], true
	}
	return
}
func Grow[S ~[]E, E any](s S, es ...E) S { return append(s, es...) }
func Clone[S ~[]E, E any](s S) S         { return append(S(nil), s...) }
func Reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
func Make[S ~[]E, E any](n int, v E) S {
	s := make(S, n)
	for i := range s {
		s[i] = v
	}
	return s
}
func Keys[M ~map[K]V, K comparable, V any](m M) int { return len(m) }
func Put[M ~map[K]V, K comparable, V any](m M, k K, v V) M {
	if m == nil {
		m = make(M)
	}
	m[k] = v
	return m
}
func Drop[M ~map[K]V, K comparable, V any](m M, k K) { delete(m, k) }
func Send[C ~chan E, E any](c C, es ...E) {
	for _, e := range es {
		c <- e
	}
	close(c)
}
func Recv[C ~chan E, E any](c C) (out []E) {
	for e := range c {
		out = append(out, e)
	}
	return
}
func Sum[A ~[3]E, E ~int | ~float64](a A) (s E) {
	for _, e := range a {
		s += e
	}
	return
}
func Bytes[T ~string | ~[]byte](t T) (n int) {
	for i := 0; i < len(t); i++ {
		n += int(t[i])
	}
	return
}
func Chars[T ~string | ~[]byte](t T) (out []rune) {
	for _, c := range string(t) {
		out = append(out, c)
	}
	return
}
func Apply[F ~func(A) A, A any](f F, x A, n int) A {
	for i := 0; i < n; i++ {
		x = f(x)
	}
	return x
}
func main() {
	is := Ints{3, 1, 2}
	fmt.Println(Len(is), Len(Ints(nil)), Len([]string{"a"}))
	fmt.Println(First(is))
	fmt.Println(First(Ints{}))
	g := Grow(is, 4, 5)
	Reverse(g)
	fmt.Printf("%v %T %v %T\n", g, g, Clone(g), Clone(g))
	fmt.Printf("%v %T\n", Make[Ints](3, 7), Make[Ints](0, 1))
	fmt.Printf("%v\n", Make[[]string](2, "x"))
	n := Put(Names(nil), "a", 1)
	n = Put(n, "b", 2)
	Drop(n, "a")
	fmt.Println(n, Keys(n), Keys(map[int]int{1: 1}))
	p := make(Pipe, 3)
	Send(p, "x", "y", "z")
	fmt.Println(Recv(p))
	c := make(chan int, 2)
	Send(c, 1, 2)
	fmt.Println(Recv(c))
	fmt.Println(Sum(Grid{1.5, 2.5, 3}), Sum([3]int{1, 2, 3}))
	fmt.Println(Bytes("abc"), Bytes(Text("日本")), Bytes(Raw{1, 2, 3}), Bytes([]byte("é")))
	fmt.Println(Chars("héllo"), Chars(Text("日本")), Chars(Raw("ab")))
	fmt.Println(Apply(Op(func(i int) int { return i * 2 }), 1, 10), Apply(func(s string) string { return s + "!" }, "hi", 3))
}

// Output:
// 6 0 2
// 3 true
// 0 false
// [5 4 2 1 3] main.Ints [5 4 2 1 3] main.Ints
// [7 7 7] main.Ints
// [x x]
// map[b:2] 1 1
// [x y z]
// [1 2]
// 7 6
// 294 1104 6 364
// [104 233 108 108 111] [26085 26412] [97 98]
// 1024 hi!!!
