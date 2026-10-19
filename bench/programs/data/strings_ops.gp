// bench: strings_ops
// category: data
// size: 100000
// smoke: 500
// twin: no
// about: strings.Fields, Split, Join, Repeat, ToUpper, Contains and a strings.Builder over generated text
package main

import (
	"fmt"
	"strings"
)

func main() {
	text := strings.Repeat("the quick brown fox jumps over the lazy dog ", 20)
	total := 0
	for i := 0; i < benchN; i++ {
		words := strings.Fields(text)
		total += len(words)
		parts := strings.Split(text, "o")
		total += len(parts)
		var b strings.Builder
		for _, w := range words[:8] {
			b.WriteString(strings.ToUpper(w))
			b.WriteByte(' ')
		}
		total += b.Len() + len(strings.Join(words[:12], "-"))
		if strings.Contains(text, "lazy cat") {
			total++
		}
	}
	fmt.Println(total)
}
