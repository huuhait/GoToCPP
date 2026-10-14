// phase: 03

// Substrings, concatenation in a loop and range over the result.
package main

import "fmt"

func words(s string) {
	start := 0
	for i, r := range s + " " {
		if r == ' ' {
			if i > start {
				fmt.Println(start, s[start:i])
			}
			start = i + 1
		}
	}
}

func reverse(s string) string {
	out := ""
	for _, r := range s {
		out = string(r) + out
	}
	return out
}

func main() {
	words("go  to  C++ 世界 ok")
	fmt.Println(reverse("héllo, 世界"), reverse(""), reverse("\xff\xfe"))
	csv := ""
	for i := 0; i < 5; i++ {
		if i > 0 {
			csv += ","
		}
		csv += string(rune('a' + i))
	}
	fmt.Println(csv, len(csv), csv[2:5])
}

// Output:
// 0 go
// 4 to
// 8 C++
// 12 世界
// 19 ok
// 界世 ,olléh  ��
// a,b,c,d,e 9 b,c
