// phase: 03

// Wordfreq-core counts the words of a text and prints the most frequent ones.
// It uses only the core language (no strings, sort or unicode packages), so
// splitting, lower-casing and sorting are done by hand.
package main

import "fmt"

const text = `Go is simple. C++ is fast. gocpp is Go that compiles to C++,
so a gocpp program is simple AND fast. Is it? The compiler is written in Go;
the runtime is written in C++. Café, naïve and Zürich are words too, and so
are "the" and THE.`

type entry struct {
	word  string
	count int
}

// isLetter treats ASCII letters, digits and '+' (for C++), and every
// non-ASCII rune, as part of a word.
func isLetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '+' || r >= 0x80
}

// lower turns ASCII upper-case letters into lower case.
func lower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 'a' - 'A'
	}
	return r
}

// words splits s into lower-case words. range over a string decodes UTF-8.
func words(s string) []string {
	var out []string
	var cur []rune
	for _, r := range s {
		if isLetter(r) {
			cur = append(cur, lower(r))
			continue
		}
		if len(cur) > 0 {
			out = append(out, string(cur))
			cur = cur[:0]
		}
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// less orders entries by count (highest first), then by word.
func less(a, b entry) bool {
	if a.count != b.count {
		return a.count > b.count
	}
	return a.word < b.word
}

// sortEntries is an insertion sort: the core language has no sort package.
func sortEntries(es []entry) {
	for i := 1; i < len(es); i++ {
		for j := i; j > 0 && less(es[j], es[j-1]); j-- {
			es[j], es[j-1] = es[j-1], es[j]
		}
	}
}

func reverse(s string) string {
	rs := []rune(s)
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		rs[i], rs[j] = rs[j], rs[i]
	}
	return string(rs)
}

func main() {
	ws := words(text)
	counts := map[string]int{}
	for _, w := range ws {
		counts[w]++
	}
	fmt.Println("words:", len(ws), "distinct:", len(counts))

	// Map iteration order is not fixed, so collect the entries and sort them.
	entries := make([]entry, 0, len(counts))
	for w, n := range counts {
		entries = append(entries, entry{w, n})
	}
	sortEntries(entries)
	top := entries
	if len(top) > 8 {
		top = top[:8]
	}
	for i, e := range top {
		fmt.Printf("%d. %s %d\n", i+1, e.word, e.count)
	}

	// The longest word, counted in characters: a []rune has one per character.
	longest := ""
	for _, e := range entries {
		if n, m := len([]rune(e.word)), len([]rune(longest)); n > m || n == m && e.word < longest {
			longest = e.word
		}
	}
	fmt.Println("longest:", longest, reverse(longest))

	// How many distinct words have each length? fmt prints maps with sorted keys.
	byLen := map[int]int{}
	for w := range counts {
		byLen[len([]rune(w))]++
	}
	fmt.Println("by length:", byLen)

	// Words that are not ASCII, found byte by byte.
	var special []string
	for _, e := range entries {
		for i := 0; i < len(e.word); i++ {
			if e.word[i] >= 0x80 {
				special = append(special, e.word)
				break
			}
		}
	}
	fmt.Println("not ASCII:", special)
	for _, w := range special {
		fmt.Println(" ", w, len(w), "bytes,", len([]rune(w)), "runes, reversed", reverse(w))
	}

	// copy and delete: drop the words that appear once.
	keep := make([]entry, len(entries))
	n := copy(keep, entries)
	for _, e := range keep[:n] {
		if e.count == 1 {
			delete(counts, e.word)
		}
	}
	_, hasCafe := counts["café"]
	fmt.Println("repeated words:", len(counts), "café repeated:", hasCafe, "is:", counts["is"])
}
