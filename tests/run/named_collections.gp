// phase: 03

// Named strings, slices, arrays and maps (step 3.9): a word counter over a named slice of named
// strings, a named map with named keys, a named array as a small matrix, and the operations that
// keep the named type (append, s[a:b], +).
package main

import "fmt"

type Word string
type Words []Word
type Counts map[Word]int
type Vec [3]int

func split(s string) Words {
	var ws Words
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ' ' {
			if i > start {
				ws = append(ws, Word(s[start:i]))
			}
			start = i + 1
		}
	}
	return ws
}

func add(a, b Vec) Vec {
	var r Vec
	for i := range r {
		r[i] = a[i] + b[i]
	}
	return r
}

func main() {
	ws := split("the cat and the hat and the bat")
	counts := Counts{}
	for _, w := range ws {
		counts[w]++
	}
	fmt.Println(len(ws), len(counts), counts["the"], counts["and"], counts["dog"])
	longest := ws[0]
	for _, w := range ws[1:] {
		if len(w) > len(longest) || len(w) == len(longest) && w > longest {
			longest = w
		}
	}
	fmt.Println(longest, longest+"!", longest[1:], longest[0] == 't')
	first := ws[:3]
	first = append(first, "!")
	fmt.Println(len(first), first[3], ws[3])
	v := add(Vec{1, 2, 3}, Vec{10, 20, 30})
	fmt.Println(v[0], v[1], v[2], v == Vec{11, 22, 33})
	s := v[1:]
	s[0] = 0
	fmt.Println(v[1], len(s))
}

// Output:
// 8 5 3 2 0
// the the! he true
// 4 ! !
// 11 22 33 true
// 0 2
