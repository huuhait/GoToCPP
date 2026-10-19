// phase: 09
// std-internal: tests internal/stringslite, which only the standard library may import

// internal/stringslite is a Go package of Go's own (strings without the dependencies that os must
// not have), ported unchanged. This runs its functions through the Rabin-Karp switch of Index (the
// strings.Index algorithm: bytealg.MaxLen, MaxBruteForce, Cutover and IndexRabinKarp, which are all
// native or generic here) on 12,000 seeded cases against a naive search, and the small functions on
// their edge cases. tests/twin_test.go (GOCPP_RUN_VS_GO=1) builds the same file with real Go, with bytealg replaced
// by the public bytes and strings.
package main

import (
	"fmt"
	"internal/stringslite"
)

type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s ^= r.s << 13
	r.s ^= r.s >> 7
	r.s ^= r.s << 17
	return r.s
}

func (r *rng) intn(n int) int { return int(r.next() % uint64(n)) }

func naiveIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func main() {
	var checks, failures int
	check := func(what string, got, want any) {
		checks++
		if got != want {
			failures++
			fmt.Printf("FAIL %s: got %v, want %v\n", what, got, want)
		}
	}

	// Index: haystacks of 0 to 300 bytes over a small alphabet, sometimes much longer (the part of
	// Index that gives up on IndexByte and goes to Rabin-Karp needs a long haystack with many false
	// starts), needles from 0 to 70 bytes, half of them cut out of the haystack.
	r := &rng{0x2545f4914f6cdd1d}
	alpha := []string{"ab", "abc", "a", "abcdefgh"}
	for i := 0; i < 12000; i++ {
		a := alpha[r.intn(len(alpha))]
		n := r.intn(300)
		if r.intn(8) == 0 {
			n = 1000 + r.intn(3000)
		}
		b := make([]byte, n)
		for j := range b {
			b[j] = a[r.intn(len(a))]
		}
		s := string(b)
		var sub string
		if n > 0 && r.intn(2) == 0 {
			lo := r.intn(n)
			hi := lo + r.intn(min(n-lo, 70)+1)
			sub = s[lo:hi]
		} else {
			m := r.intn(70)
			c := make([]byte, m)
			for j := range c {
				c[j] = a[r.intn(len(a))]
			}
			sub = string(c)
		}
		want := naiveIndex(s, sub)
		check("Index", stringslite.Index(s, sub), want)
		before, after, found := stringslite.Cut(s, sub)
		check("Cut found", found, want >= 0)
		if want >= 0 {
			check("Cut before", before, s[:want])
			check("Cut after", after, s[want+len(sub):])
		} else {
			check("Cut not found: the whole string", before, s)
			check("Cut not found: nothing after", after, "")
		}
	}

	check("IndexByte", stringslite.IndexByte("héllo", 'l'), 3)
	check("IndexByte missing", stringslite.IndexByte("hello", 'z'), -1)
	check("HasPrefix", stringslite.HasPrefix("prefix.txt", "prefix"), true)
	check("HasPrefix too long", stringslite.HasPrefix("pre", "prefix"), false)
	check("HasPrefix empty", stringslite.HasPrefix("", ""), true)
	check("HasSuffix", stringslite.HasSuffix("prefix.txt", ".txt"), true)
	check("HasSuffix too long", stringslite.HasSuffix("txt", ".txt"), false)
	check("TrimPrefix", stringslite.TrimPrefix("prefix.txt", "prefix"), ".txt")
	check("TrimPrefix not there", stringslite.TrimPrefix("prefix.txt", "x"), "prefix.txt")
	check("TrimSuffix", stringslite.TrimSuffix("prefix.txt", ".txt"), "prefix")
	check("TrimSuffix not there", stringslite.TrimSuffix("prefix.txt", ".go"), "prefix.txt")
	after, ok := stringslite.CutPrefix("key=value", "key=")
	check("CutPrefix", after, "value")
	check("CutPrefix ok", ok, true)
	after, ok = stringslite.CutPrefix("key=value", "val")
	check("CutPrefix not there", after, "key=value")
	check("CutPrefix not there ok", ok, false)
	before, ok := stringslite.CutSuffix("file.txt", ".txt")
	check("CutSuffix", before, "file")
	check("CutSuffix ok", ok, true)
	before, ok = stringslite.CutSuffix("file.txt", ".go")
	check("CutSuffix not there", before, "file.txt")
	check("CutSuffix not there ok", ok, false)

	// Clone is a copy: changing the bytes of the original slice it came from does not change it.
	b := []byte("clone me")
	c := stringslite.Clone(string(b))
	b[0] = 'X'
	check("Clone", c, "clone me")
	check("Clone empty", stringslite.Clone(""), "")
	big := make([]byte, 100000)
	for i := range big {
		big[i] = byte('a' + i%26)
	}
	check("Clone big", stringslite.Clone(string(big)) == string(big), true)

	fmt.Println(checks, "checks,", failures, "failures")
}

// Output:
// 48022 checks, 0 failures
