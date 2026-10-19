// bench: fmt_sprintf
// category: runtime
// size: 1000000
// smoke: 3000
// twin: yes
// about: fmt.Sprintf with %d, %s, %v and %.3f in a loop, summing the lengths and a hash of the results
package main

import "fmt"

func main() {
	total := 0
	h := uint64(14695981039346656037)
	for i := 0; i < benchN; i++ {
		s := fmt.Sprintf("n=%d name=%s v=%v f=%.3f", i, "gopher", i*3, float64(i)/7)
		total += len(s)
		h = (h ^ uint64(s[len(s)-1])) * 1099511628211
		h = (h ^ uint64(s[2])) * 1099511628211
	}
	fmt.Println(total, h)
}
