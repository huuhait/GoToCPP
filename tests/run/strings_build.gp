// phase: 03

// Building strings: byte slices grown with append, string(bytes) copies, runes and UTF-8
// (reverse by runes, count runes, string(rune)), string comparison and a word frequency count
// by hand (no package strings until Phase 09).
package main

import "fmt"

func reverse(s string) string {
	rs := []rune(s)
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		rs[i], rs[j] = rs[j], rs[i]
	}
	return string(rs)
}

func words(s string) []string {
	var out []string
	start := -1
	for i, r := range s {
		if r == ' ' || r == ',' || r == '.' {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

func repeat(s string, n int) string {
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}

func main() {
	s := "héllo, wörld"
	fmt.Println(reverse(s), len(s), len([]rune(s)), upper(s))
	b := []byte("abc")
	t := string(b)
	b[0] = 'X'
	fmt.Println(t, string(b), string(rune(0x4e16)), string(rune(-1)) == "�")
	fmt.Println(repeat("ab", 3), repeat("", 5) == "", "abc" < "abd", "Z" < "a")
	count := map[string]int{}
	for _, w := range words("the cat. the dog, the end") {
		count[w]++
	}
	fmt.Println(count, len(words("  ")), words("one"))
	var acc string
	for i := 0; i < 5; i++ {
		acc += string(rune('a'+i)) + "-"
	}
	fmt.Println(acc[:len(acc)-1])
}

// Output:
// dlröw ,olléh 14 12 HéLLO, WöRLD
// abc Xbc 世 true
// ababab true true true
// map[cat:1 dog:1 end:1 the:3] 0 [one]
// a-b-c-d-e
