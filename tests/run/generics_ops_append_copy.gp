// phase: 07

// append and copy over slice type parameters, including strings appended to byte slices.
package main

import "fmt"

type Bytes []byte
type Ints []int

func Push[S ~[]E, E any](s S, v ...E) S { return append(s, v...) }
func Join[S ~[]E, E any](a, b S) S      { return append(a, b...) }
func AppendStr[B ~[]byte](b B, s string) B { return append(b, s...) }
func CopyOf[S ~[]E, E any](s S) S {
	out := make(S, len(s))
	n := copy(out, s)
	_ = n
	return out
}
func Insert[S ~[]E, E any](s S, i int, v E) S {
	s = append(s, v)
	copy(s[i+1:], s[i:])
	s[i] = v
	return s
}
func CopyStr[B ~[]byte](dst B, s string) int { return copy(dst, s) }
func Grow[S ~[]E, E any](s S, n int) S {
	return append(s[:cap(s)], make(S, n)...)[:len(s)]
}
func Clip[S ~[]E, E any](s S) S { return s[:len(s):len(s)] }

func main() {
	a := Ints{1, 2}
	b := Push(a, 3, 4)
	c := Push(a)
	fmt.Println(a, b, c, Join(a, b), Join(Ints(nil), nil) == nil)
	fmt.Printf("%T %T\n", Push(a), Join(a, a))
	fmt.Println(AppendStr(Bytes("ab"), "cd"), AppendStr([]byte(nil), "héllo"), string(AppendStr(Bytes("x"), "yz")))
	o := CopyOf(Ints{7, 8, 9})
	fmt.Println(o, len(o), Insert(Ints{1, 2, 4}, 2, 3), Insert([]string{"a", "c"}, 1, "b"))
	d := make(Bytes, 3)
	fmt.Println(CopyStr(d, "hello"), string(d), CopyStr(make(Bytes, 8), "hi"))
	g := Grow(Ints{1, 2, 3}, 100)
	fmt.Println(len(g), cap(g) >= 103, cap(Clip(make(Ints, 2, 9))))
	x := make(Ints, 2, 4)
	y := Push(x, 1)
	z := Push(x, 2)
	fmt.Println(y, z)
}

// Output:
// [1 2] [1 2 3 4] [1 2] [1 2 1 2 3 4] true
// main.Ints main.Ints
// [97 98 99 100] [104 195 169 108 108 111] xyz
// [7 8 9] 3 [1 2 3 4] [a b c]
// 3 hel 2
// 3 true 2
// [0 0 2] [0 0 2]
