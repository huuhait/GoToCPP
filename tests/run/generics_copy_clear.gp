// phase: 07

// copy, clear, delete, len and cap on type parameters whose core types are slices, maps and arrays.
package main

import "fmt"

func CopyAll[S ~[]E, E any](dst, src S) (int, S) { return copy(dst, src), dst }

func Clear[S ~[]E, E any](s S) S {
	clear(s)
	return s
}

func ClearMap[M ~map[K]V, K comparable, V any](m M) int {
	clear(m)
	return len(m)
}

func Delete[M ~map[K]V, K comparable, V any](m M, ks ...K) int {
	for _, k := range ks {
		delete(m, k)
	}
	return len(m)
}

func Caps[S ~[]E, E any](s S) (int, int) { return len(s), cap(s) }

func ArrLen[A ~[4]E, E any](a A) int { return len(a) }

func Bytes[T ~string](s T) []byte { return []byte(s) }

func Append[S ~[]E, E any](s S, tail ...E) S { return append(s, tail...) }

func AppendString[B ~[]byte](b B, s string) B { return append(b, s...) }

func CopyString[B ~[]byte](b B, s string) int { return copy(b, s) }

type Names []string
type Bag map[string]int
type Quad [4]string
type Buf []byte

func main() {
	d := make(Names, 2)
	fmt.Println(CopyAll(d, Names{"a", "b", "c"}))
	fmt.Println(Clear([]int{1, 2, 3}), Clear(Names{"x"}) != nil, Clear([]bool(nil)) == nil)
	fmt.Println(ClearMap(Bag{"a": 1}), ClearMap(map[int]bool{}), ClearMap[map[string]int](nil))
	fmt.Println(Delete(Bag{"a": 1, "b": 2, "c": 3}, "a", "z"), Delete(map[int]int{1: 1}, 1))
	fmt.Println(Caps(make(Names, 2, 5)))
	fmt.Println(Caps([]int(nil)))
	fmt.Println(ArrLen(Quad{}), ArrLen([4]int{}), Bytes("héy"), Bytes(Names2("x")))
	fmt.Println(Append(Names{"a"}, "b", "c"), Append([]int(nil)), AppendString(Buf("ab"), "cd"), CopyString(make(Buf, 3), "xy"))
}

type Names2 string

// Output:
// 2 [a b]
// [0 0 0] true true
// 0 0 0
// 2 0
// 2 5
// 0 0
// 4 4 [104 195 169 121] [120]
// [a b c] [] [97 98 99 100] 2
