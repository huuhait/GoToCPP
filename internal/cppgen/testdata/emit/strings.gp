// Strings: concatenation, comparison, len(s), the byte s[i] (bounds-checked by
// gp::string), and string(s).
package main

import "fmt"

func join(a, b string) string {
	return a + "-" + b
}

func main() {
	s := "héllo"
	t := s + ", " + "world"
	fmt.Println(s, t, len(s), len(t), len("héllo"), len(""), s[0], s[1], s[2], s[len(s)-1])
	fmt.Println(join(s, t), len(join("", "")), s == "héllo", s+"" == s, t > s, "" < s)
	var i uint8 = 4
	var j int16 = 2
	fmt.Println(s[i], s[j], s[i]+s[j], s[i] == 'l', s[0] >= 'a' && s[0] <= 'z', s[1]-'0' <= 9)
	n := 0
	digits := "2026x"
	n = n*10 + int(digits[0]-'0')
	n = n*10 + int(digits[1]-'0')
	n = n*10 + int(digits[2]-'0')
	n = n*10 + int(digits[3]-'0')
	fmt.Println(n, digits[4]-'0' <= 9, digits[4]-'0', string(s) == s, "abc"[i-3])
	var empty string
	fmt.Println(empty == "", len(empty), empty+empty == "", "x"+empty+"y")
	println(s, t+"!", s[0])
	fmt.Println(len(s+t) == len(s)+len(t))
}
