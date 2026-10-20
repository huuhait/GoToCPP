// phase: 09
// Conversions between a string and a slice whose element is a named byte or rune type (type Uint8 uint8): []Uint8("abc"), string(u8s), []MyRune(s).

package main

import "fmt"

type U8 uint8
type R rune
type Bytes []U8
type Runes []R

func main() {
	a := []U8("hello")
	fmt.Println(a, len(a), cap(a) >= 5)
	a[0] = 'J'
	fmt.Println(string(a))

	s := "héllo, 世界"
	r := []R(s)
	fmt.Println(r, len(r))
	r[1] = 'e'
	fmt.Println(string(r))

	b := Bytes("xyz")
	fmt.Println(b, string(b))
	rs := Runes("日本")
	fmt.Println(rs, string(rs))

	var empty []U8 = []U8("")
	fmt.Println(len(empty), empty == nil, string(empty) == "")

	c := []U8(s)
	c = append(c, '!')
	fmt.Println(string(c))

	const k = "const"
	d := []U8(k)
	fmt.Println(len(d), cap(d), string(d))

	var u []U8
	fmt.Printf("%q\n", string(u))
	fmt.Println(string([]U8{0xe4, 0xb8, 0x96}), string([]R{0x4e16, -1, 0x10ffff + 1}))
}

// Output:
// [104 101 108 108 111] 5 true
// Jello
// [104 233 108 108 111 44 32 19990 30028] 9
// hello, 世界
// [120 121 122] xyz
// [26085 26412] 日本
// 0 false true
// héllo, 世界!
// 5 5 const
// ""
// 世 世��
