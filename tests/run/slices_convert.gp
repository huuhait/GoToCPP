// phase: 03

// The conversions between strings and slices copy: []byte(s) and string(b) byte by byte,
// []rune(s) decodes UTF-8 (an invalid byte is U+FFFD), string(runes) encodes each rune (a
// value that is no code point is U+FFFD); empty strings give empty slices that are not nil.
package main

import "fmt"

func main() {
	s := "héllo, 世界"
	b := []byte(s)
	b[0] = 'H'
	fmt.Println(len(b), string(b), s)
	r := []rune(s)
	r[1] = 'e'
	fmt.Println(len(r), string(r), string(r[7:]), r[7])
	bad := []rune("a\xffb\xe2\x82")
	fmt.Println(len(bad), bad[1], bad[3], string(bad))
	fmt.Println(string([]rune{-1, 0xD800, 0x110000, 0x10FFFF, 'z'}))
	fmt.Println([]byte("") == nil, []rune("") == nil, len([]byte("")), string([]rune(nil)), string([]byte(nil)) == "")
	t := string(b[1:3])
	b[1] = 'X'
	fmt.Println(t, string(b[:2]), b[2])
	var bs []byte
	for i := range 5 {
		bs = append(bs, byte('a'+i))
	}
	bs = append(bs, " and "...)
	bs = append(bs, s...)
	fmt.Println(string(bs))
	n := 0
	for _, c := range []byte("abc") {
		n += int(c)
	}
	fmt.Println(n)
}

// Output:
// 14 Héllo, 世界 héllo, 世界
// 9 hello, 世界 世界 19990
// 5 65533 65533 a�b��
// ���􏿿z
// false false 0  true
// é HX 169
// abcde and héllo, 世界
// 294
