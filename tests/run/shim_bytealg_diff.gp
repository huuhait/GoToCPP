// phase: 09
// std-internal: tests internal/bytealg, which only the standard library may import

// internal/bytealg against Go: 10,000 seeded random cases for each function, with a small alphabet
// (so that bytes and substrings repeat) that includes 0 and 255, lengths that cross the 8-byte
// words and the 8160-byte blocks of Count, and nil and empty arguments. Each function prints how
// many cases it ran, how many differ from a naive loop written here, and a digest of all its
// results. tests/twin_test.go (GOCPP_RUN_VS_GO=1) builds this same file with real Go, with the functions of
// bytealg replaced by calls of bytes and strings (their public equivalents), and checks that the
// digests are the same, so the output below is what Go computes.
package main

import (
	"fmt"
	"internal/bytealg"
)

// A xorshift generator, so that gocpp and Go see the same cases.
type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s ^= r.s << 13
	r.s ^= r.s >> 7
	r.s ^= r.s << 17
	return r.s
}

func (r *rng) intn(n int) int { return int(r.next() % uint64(n)) }

// A digest is FNV-1a over the results.
type digest struct{ h uint64 }

func newDigest() *digest { return &digest{14695981039346656037} }

func (d *digest) add(v int) {
	for i := 0; i < 8; i++ {
		d.h ^= uint64(byte(uint64(v) >> (8 * i)))
		d.h *= 1099511628211
	}
}

const cases = 14000 // rounds; a function that needs a longer needle runs on fewer of them, but on more than 10,000

var alphabets = [][]byte{{0, 1}, {'a', 'b', 'c'}, {0, 255, 7, 128}, {'x'}, {'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 0, 255}}

// genBytes makes a random slice: nil half the time when it is empty, with a length that is mostly
// short, sometimes long enough for the blocks of Count (8160 bytes).
func genBytes(r *rng, alpha []byte) []byte {
	var n int
	switch r.intn(10) {
	case 0:
		n = 0
	case 1:
		if r.intn(2) == 0 {
			n = 8100 + r.intn(200)
		} else {
			n = 2000 + r.intn(400)
		}
	case 2, 3:
		n = 60 + r.intn(200)
	default:
		n = r.intn(40)
	}
	if n == 0 && r.intn(2) == 0 {
		return nil
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = alpha[r.intn(len(alpha))]
	}
	return b
}

// subOf cuts an unaligned part out of b, or makes a slice that is not in b.
func subOf(r *rng, b []byte, alpha []byte) []byte {
	if len(b) > 0 && r.intn(4) != 0 {
		lo := r.intn(len(b))
		hi := lo + r.intn(len(b)-lo+1)
		if hi-lo > 70 {
			hi = lo + 70
		}
		return b[lo:hi]
	}
	n := r.intn(8)
	s := make([]byte, n)
	for i := range s {
		s[i] = alpha[r.intn(len(alpha))]
	}
	return s
}

func naiveIndexByte(b []byte, c byte) int {
	for i := 0; i < len(b); i++ {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func naiveLastIndexByte(b []byte, c byte) int {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func naiveCount(b []byte, c byte) int {
	n := 0
	for _, x := range b {
		if x == c {
			n++
		}
	}
	return n
}

func naiveCompare(a, b []byte) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

func naiveIndex(a, b []byte) int {
	for i := 0; i+len(b) <= len(a); i++ {
		if naiveCompare(a[i:i+len(b)], b) == 0 {
			return i
		}
	}
	return -1
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}


func main() {
	r := &rng{0x9e3779b97f4a7c15}

	type fn struct {
		name string
		n    int // cases run
		bad  int // cases that differ from the naive loop
		d    *digest
	}
	names := []string{"IndexByte", "IndexByteString", "LastIndexByte", "LastIndexByteString", "Count", "CountString",
		"Equal", "Compare", "CompareString", "Index", "IndexString", "IndexRabinKarp", "IndexRabinKarpString",
		"LastIndexRabinKarp", "LastIndexRabinKarpString", "HashStr", "HashStrBytes", "HashStrRev", "MakeNoZero"}
	fns := make(map[string]*fn)
	for _, n := range names {
		fns[n] = &fn{n, 0, 0, newDigest()}
	}
	rec := func(name string, got, want int) {
		f := fns[name]
		f.n++
		f.d.add(got)
		if got != want {
			f.bad++
		}
	}

	for i := 0; i < cases; i++ {
		alpha := alphabets[r.intn(len(alphabets))]
		hay := genBytes(r, alpha)
		needle := subOf(r, hay, alpha)
		c := alpha[r.intn(len(alpha))]
		if r.intn(8) == 0 {
			c = byte(r.intn(256))
		}
		hayS, needleS := string(hay), string(needle)

		rec("IndexByte", bytealg.IndexByte(hay, c), naiveIndexByte(hay, c))
		rec("IndexByteString", bytealg.IndexByteString(hayS, c), naiveIndexByte(hay, c))
		rec("LastIndexByte", bytealg.LastIndexByte(hay, c), naiveLastIndexByte(hay, c))
		rec("LastIndexByteString", bytealg.LastIndexByteString(hayS, c), naiveLastIndexByte(hay, c))
		rec("Count", bytealg.Count(hay, c), naiveCount(hay, c))
		rec("CountString", bytealg.CountString(hayS, c), naiveCount(hay, c))

		// Equal and Compare: the second slice is a copy with at most one byte changed or cut off.
		other := append([]byte(nil), hay...)
		switch r.intn(4) {
		case 0:
			if len(other) > 0 {
				other[r.intn(len(other))] ^= byte(1 + r.intn(255))
			}
		case 1:
			other = other[:r.intn(len(other)+1)]
		case 2:
			other = append(other, alpha[r.intn(len(alpha))])
		}
		rec("Equal", boolInt(bytealg.Equal(hay, other)), boolInt(naiveCompare(hay, other) == 0))
		rec("Compare", bytealg.Compare(hay, other), naiveCompare(hay, other))
		rec("CompareString", bytealg.CompareString(hayS, string(other)), naiveCompare(hay, other))

		if len(needle) >= 2 && len(needle) <= bytealg.MaxLen {
			rec("Index", bytealg.Index(hay, needle), naiveIndex(hay, needle))
			rec("IndexString", bytealg.IndexString(hayS, needleS), naiveIndex(hay, needle))
		}
		if len(needle) >= 1 && len(needle) <= len(hay) {
			want := naiveIndex(hay, needle)
			rec("IndexRabinKarp", bytealg.IndexRabinKarp(hay, needle), want)
			rec("IndexRabinKarpString", bytealg.IndexRabinKarp(hayS, needleS), want)
			// The last occurrence: scan from the right with the naive loop.
			wantLast := -1
			for j := len(hay) - len(needle); j >= 0; j-- {
				if naiveCompare(hay[j:j+len(needle)], needle) == 0 {
					wantLast = j
					break
				}
			}
			rec("LastIndexRabinKarp", bytealg.LastIndexRabinKarp(hay, needle), wantLast)
			rec("LastIndexRabinKarpString", bytealg.LastIndexRabinKarp(hayS, needleS), wantLast)
		}

		// The hashes: the same loop as Go's, written out here (the multiplier is the power of the
		// prime for the length).
		h, pow := bytealg.HashStr(needleS)
		hb, powb := bytealg.HashStr(needle)
		var wantH, wantHRev, wantPow uint32 = 0, 0, 1
		for _, x := range needle {
			wantH = wantH*bytealg.PrimeRK + uint32(x)
			wantPow *= bytealg.PrimeRK
		}
		for j := len(needle) - 1; j >= 0; j-- {
			wantHRev = wantHRev*bytealg.PrimeRK + uint32(needle[j])
		}
		hr, powr := bytealg.HashStrRev(needleS)
		rec("HashStr", int(h)^int(pow)<<32, int(wantH)^int(wantPow)<<32)
		rec("HashStrBytes", int(hb)^int(powb)<<32, int(wantH)^int(wantPow)<<32)
		rec("HashStrRev", int(hr)^int(powr)<<32, int(wantHRev)^int(wantPow)<<32)

		// MakeNoZero: a slice of the length with room for it. Its bytes are not set, so only the
		// length counts.
		n := r.intn(300)
		m := bytealg.MakeNoZero(n)
		rec("MakeNoZero", len(m)*1000+boolInt(cap(m) >= n), n*1000+1)
	}
	for _, n := range names {
		f := fns[n]
		fmt.Printf("%s: %d cases, %d differ from the naive loop, digest %016x\n", n, f.n, f.bad, f.d.h)
	}
}

// Output:
// IndexByte: 14000 cases, 0 differ from the naive loop, digest 4699d61a120000c8
// IndexByteString: 14000 cases, 0 differ from the naive loop, digest 4699d61a120000c8
// LastIndexByte: 14000 cases, 0 differ from the naive loop, digest 5b9d718be547c623
// LastIndexByteString: 14000 cases, 0 differ from the naive loop, digest 5b9d718be547c623
// Count: 14000 cases, 0 differ from the naive loop, digest 8a92befc2ae9acba
// CountString: 14000 cases, 0 differ from the naive loop, digest 8a92befc2ae9acba
// Equal: 14000 cases, 0 differ from the naive loop, digest 26fd0057c6587264
// Compare: 14000 cases, 0 differ from the naive loop, digest 32bd8689d279d3ed
// CompareString: 14000 cases, 0 differ from the naive loop, digest 32bd8689d279d3ed
// Index: 9131 cases, 0 differ from the naive loop, digest 225c9ef54d65932f
// IndexString: 9131 cases, 0 differ from the naive loop, digest 225c9ef54d65932f
// IndexRabinKarp: 10730 cases, 0 differ from the naive loop, digest 1472b732063d31c6
// IndexRabinKarpString: 10730 cases, 0 differ from the naive loop, digest 1472b732063d31c6
// LastIndexRabinKarp: 10730 cases, 0 differ from the naive loop, digest dc8a18dd58fa4d78
// LastIndexRabinKarpString: 10730 cases, 0 differ from the naive loop, digest dc8a18dd58fa4d78
// HashStr: 14000 cases, 0 differ from the naive loop, digest 71f5f0cfb5efad81
// HashStrBytes: 14000 cases, 0 differ from the naive loop, digest 71f5f0cfb5efad81
// HashStrRev: 14000 cases, 0 differ from the naive loop, digest 8088fb369fbf32b2
// MakeNoZero: 14000 cases, 0 differ from the naive loop, digest 6f6cd2d0d59dc2cf
