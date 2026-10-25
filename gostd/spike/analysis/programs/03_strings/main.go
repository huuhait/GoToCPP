// Strings: bytes versus runes, indexing, slicing, len, range over a string, comparisons,
// concatenation, []byte/[]rune/string conversions and escape sequences.
package main

func main() {
	s := "héllo, 世界"
	println(len(s))
	println(s[0], s[1], s[len(s)-1])
	println(s[:5], s[6:], s[1:3])
	println(s == "héllo", s < "z")
	u := s + "!"
	println(u)
	b := []byte("abc")
	b[0] = 'X'
	println(string(b))
	rs := []rune("aé")
	println(len(rs), rs[1], string(rs))
	count := 0
	for i, r := range "aé中" {
		println(i, r)
		count++
	}
	println(count)
	println("escapes", "tab\tsep", "quote\"", "back\\slash", "\x41", "\101", "é")
	var sb []byte
	for i := 0; i < 3; i++ {
		sb = append(sb, byte('a'+i))
	}
	println(string(sb))
}
