// bench: fasta
// category: cpu
// size: 20000000
// smoke: 20000
// twin: yes
// about: fasta: DNA generation from a weighted table with a linear congruential generator, plus a repeated sequence
package main

import "fmt"

const (
	im = 139968
	ia = 3877
	ic = 29573
)

const alu = "GGCCGGGCGCGGTGGCTCACGCCTGTAATCCCAGCACTTTGG" +
	"GAGGCCGAGGCGGGCGGATCACCTGAGGTCAGGAGTTCGAGA" +
	"CCAGCCTGGCCAACATGGTGAAACCCCGTCTCTACTAAAAAT" +
	"ACAAAAATTAGCCGGGCGTGGTGGCGCGCGCCTGTAATCCCA" +
	"GCTACTCGGGAGGCTGAGGCAGGAGAATCGCTTGAACCCGGG" +
	"AGGCGGAGGTTGCAGTGAGCCGAGATCGCGCCACTGCACTCC" +
	"AGCCTGGGCGACAGAGCGAGACTCCGTCTCAAAAA"

type acid struct {
	c byte
	p float64
}

var last = 42

func random(max float64) float64 {
	last = (last*ia + ic) % im
	return max * float64(last) / im
}

func cumulative(a []acid) {
	p := 0.0
	for i := range a {
		p += a[i].p
		a[i].p = p
	}
}

func selectRandom(a []acid) byte {
	r := random(1.0)
	for i := range a {
		if r < a[i].p {
			return a[i].c
		}
	}
	return a[len(a)-1].c
}

type sum struct {
	hash   uint64
	counts [4]int
}

func (s *sum) add(c byte) {
	s.hash = (s.hash ^ uint64(c)) * 1099511628211
	switch c {
	case 'A':
		s.counts[0]++
	case 'C':
		s.counts[1]++
	case 'G':
		s.counts[2]++
	case 'T':
		s.counts[3]++
	}
}

func (s *sum) print(name string) {
	fmt.Printf("%s %d %d %d %d %d\n", name, s.hash, s.counts[0], s.counts[1], s.counts[2], s.counts[3])
}

func main() {
	iub := []acid{
		{'a', 0.27}, {'c', 0.12}, {'g', 0.12}, {'t', 0.27},
		{'B', 0.02}, {'D', 0.02}, {'H', 0.02}, {'K', 0.02}, {'M', 0.02},
		{'N', 0.02}, {'R', 0.02}, {'S', 0.02}, {'V', 0.02}, {'W', 0.02}, {'Y', 0.02},
	}
	for i := 0; i < 4; i++ {
		iub[i].c -= 'a' - 'A'
	}
	homosapiens := []acid{
		{'A', 0.3029549426680}, {'C', 0.1979883004921},
		{'G', 0.1975473066391}, {'T', 0.3015094502008},
	}
	cumulative(iub)
	cumulative(homosapiens)

	n := benchN
	var s1, s2, s3 sum
	s1.hash, s2.hash, s3.hash = 14695981039346656037, 14695981039346656037, 14695981039346656037
	k := 0
	for i := 0; i < 2*n/10; i++ {
		s1.add(alu[k])
		k++
		if k == len(alu) {
			k = 0
		}
	}
	for i := 0; i < 3*n/10; i++ {
		s2.add(selectRandom(iub))
	}
	for i := 0; i < 5*n/10; i++ {
		s3.add(selectRandom(homosapiens))
	}
	s1.print("ONE")
	s2.print("TWO")
	s3.print("THREE")
}
