// bench: revcomp
// category: cpu
// size: 18000000
// smoke: 20000
// twin: yes
// about: reverse-complement of a generated sequence, repeated (byte slice swaps and a lookup table)
package main

import "fmt"

func main() {
	n := benchN
	const letters = "ACGTUMRWSYKVHDBN"
	const pairs = "TGCAAKYWSRMBDHVN"
	var comp [256]byte
	for i := range comp {
		comp[i] = byte(i)
	}
	for i := 0; i < len(letters); i++ {
		comp[letters[i]] = pairs[i]
	}
	buf := make([]byte, n)
	seed := uint64(99)
	for i := range buf {
		seed = seed*6364136223846793005 + 1442695040888963407
		buf[i] = letters[(seed>>33)&15]
	}
	for pass := 0; pass < 40; pass++ {
		for i, j := 0, n-1; i <= j; i, j = i+1, j-1 {
			a, b := buf[i], buf[j]
			buf[i] = comp[b]
			buf[j] = comp[a]
		}
	}
	h := uint64(14695981039346656037)
	for _, c := range buf {
		h = (h ^ uint64(c)) * 1099511628211
	}
	fmt.Printf("%s %d\n", string(buf[:16]), h)
}
