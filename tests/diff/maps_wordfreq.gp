// phase: 03

// A word count, the classic use of a map: split a text into words, count them with m[w]++,
// group the words by their count with m[n] = append(m[n], w), and print both sorted (by count,
// then by word), since a map's order is not Go's (D8).
package main

import "fmt"

const text = `the quick brown fox jumps over the lazy dog the dog barks
and the fox runs away quick quick the end of the story of the fox`

func words(s string) []string {
	var out []string
	start := -1
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ' ' || s[i] == '\n' {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	return out
}

func sortStrs(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func main() {
	counts := map[string]int{}
	for _, w := range words(text) {
		counts[w]++
	}
	byCount := make(map[int][]string)
	most := 0
	for w, n := range counts {
		byCount[n] = append(byCount[n], w)
		if n > most {
			most = n
		}
	}
	for n := most; n > 0; n-- {
		ws, ok := byCount[n]
		if !ok {
			continue
		}
		sortStrs(ws)
		fmt.Print(n, ":")
		for _, w := range ws {
			fmt.Print(" ", w)
		}
		fmt.Println()
	}
	fmt.Println(len(counts), "words,", counts["the"], "times the,", counts["cat"], "cats")
	delete(counts, "the")
	total := 0
	for _, n := range counts {
		total += n
	}
	fmt.Println(total, "without the")
}
