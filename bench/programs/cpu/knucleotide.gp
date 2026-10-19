// bench: knucleotide
// category: cpu
// size: 1000000
// smoke: 5000
// twin: yes
// about: k-nucleotide: count k-mers of a generated DNA string in maps with substring keys
package main

import "fmt"

func main() {
	n := benchN
	buf := make([]byte, n)
	seed := uint64(7)
	for i := range buf {
		seed = seed*6364136223846793005 + 1442695040888963407
		buf[i] = "ACGT"[(seed>>33)&3]
	}
	seq := string(buf)

	patterns := []string{"A", "C", "G", "T", "AA", "AC", "AG", "AT", "CA", "CC", "CG", "CT",
		"GA", "GC", "GG", "GT", "TA", "TC", "TG", "TT",
		"GGT", "GGTA", "GGTATT", "GGTATTTTAATT", "GGTATTTTAATTTATAGT"}
	for _, k := range []int{1, 2, 3, 4, 6, 12, 18} {
		m := make(map[string]int)
		for i := 0; i+k <= n; i++ {
			m[seq[i:i+k]]++
		}
		max := 0
		for _, c := range m {
			if c > max {
				max = c
			}
		}
		fmt.Printf("k=%d distinct=%d max=%d\n", k, len(m), max)
		for _, p := range patterns {
			if len(p) == k {
				fmt.Printf("%s %d\n", p, m[p])
			}
		}
	}
}
