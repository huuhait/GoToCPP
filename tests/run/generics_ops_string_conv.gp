// phase: 07

// Conversions to and from string in generic code: runes, bytes and defined types.
package main

import "fmt"

type Str interface{ ~string }
type Bytes interface{ ~[]byte | ~string }
type Rune interface{ ~int32 | ~int | ~uint8 | ~uint32 }

type Name string
type Buf []byte

func FromRune[R Rune](r R) string          { return string(rune(r)) }
func ToBytes[S Str](s S) []byte            { return []byte(s) }
func ToRunes[S Str](s S) []rune            { return []rune(s) }
func FromBytes[S Str](b []byte) S          { return S(b) }
func FromRunes[S Str](r []rune) S          { return S(r) }
func Cast[To, From Str](x From) To         { return To(x) }
func BytesOf[B Bytes](x B) []byte          { return []byte(x) }
func Len[B Bytes](x B) int                 { return len(x) }
func At[B Bytes](x B, i int) byte          { return x[i] }
func Sub[B Bytes](x B, i, j int) B         { return x[i:j] }
func Concat[S Str](a, b S) S               { return a + b + "!" }
func AddTo[S Str](a S, b ...S) S {
	for _, s := range b {
		a += s
	}
	return a
}

func main() {
	fmt.Println(FromRune(rune(65)), FromRune(int(0x4e16)), FromRune(uint8(97)), FromRune(uint32(0x1F600)), FromRune(int32(-1)))
	fmt.Println(ToBytes("héllo"), ToBytes(Name("ab")), ToRunes("héllo"), ToRunes(Name("日本")))
	fmt.Println(FromBytes[string]([]byte{104, 105}), FromBytes[Name]([]byte("go")), FromRunes[string]([]rune{0x65e5, 0x672c}), FromRunes[Name]([]rune("ok")))
	fmt.Println(Cast[Name]("plain"), Cast[string](Name("named")), Cast[Name](Name("same")))
	fmt.Println(BytesOf("xyz"), BytesOf(Buf{1, 2}), BytesOf([]byte(nil)) == nil, Len("héllo"), Len(Buf{1, 2, 3}), Len(Name("")))
	fmt.Println(At("abc", 1), At(Buf{7, 8, 9}, 2), Sub("hello", 1, 3), Sub(Buf{1, 2, 3, 4}, 1, 3), Sub(Name("world"), 0, 2))
	fmt.Println(Concat("a", "b"), Concat(Name("x"), Name("y")), AddTo("a", "b", "c"), AddTo(Name("n")))
	fmt.Printf("%T %T %T\n", Sub(Name("ab"), 0, 1), Sub(Buf{1}, 0, 1), Concat("", ""))
}

// Output:
// A 世 a 😀 �
// [104 195 169 108 108 111] [97 98] [104 233 108 108 111] [26085 26412]
// hi go 日本 ok
// plain named same
// [120 121 122] [1 2] true 6 3 0
// 98 9 el [2 3] wo
// ab! xy! abc n
// main.Name main.Buf string
