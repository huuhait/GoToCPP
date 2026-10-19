// phase: 09
// Differential sweep of package maps: Keys, Values, All, Collect, Insert, Clone, Copy, Equal, EqualFunc and
// DeleteFunc over pseudo-random maps of ints, strings and structs, with NaN keys, named map types and nil maps; the
// results are hashed in sorted order (a map's own order is random), one hash per function is printed. The expected
// output comes from real Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
)

type rngT struct{ s uint64 }

func (r *rngT) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) intn(n int) int { return int(r.next()>>1) % n }

type hasher struct{ h uint64 }

func (x *hasher) u64(v uint64) {
	for i := 0; i < 8; i++ {
		x.h ^= (v >> (8 * i)) & 0xff
		x.h *= 1099511628211
	}
}
func (x *hasher) str(s string) {
	x.u64(uint64(len(s)))
	for i := 0; i < len(s); i++ {
		x.h ^= uint64(s[i])
		x.h *= 1099511628211
	}
}

type IntMap map[int]string
type key struct {
	a int
	b string
}

func sorted[K comparable, V any](m map[K]V, less func(a, b K) int) []K {
	return slices.SortedFunc(maps.Keys(m), less)
}

func genIntMap(r *rngT, n int) map[int]string {
	if r.intn(10) == 0 {
		return nil
	}
	m := make(map[int]string)
	for i := 0; i < n; i++ {
		k := r.intn(n*2 + 1)
		m[k] = fmt.Sprint("v", k*3)
	}
	return m
}

func hashIntMap(h *hasher, m map[int]string) {
	h.u64(uint64(len(m)))
	if m == nil {
		h.u64(7)
	}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		h.u64(uint64(k))
		h.str(m[k])
	}
}

func main() {
	names := []string{"Keys", "Values", "All", "Collect", "Insert", "Clone", "Copy", "Equal", "EqualFunc", "DeleteFunc", "struct keys", "NaN keys", "named type", "early exit"}
	hs := make([]*hasher, len(names))
	for i := range hs {
		hs[i] = &hasher{14695981039346656037}
	}
	r := &rngT{777}
	for iter := 0; iter < 3000; iter++ {
		n := r.intn(40)
		if iter%50 == 0 {
			n = 500 + r.intn(2000)
		}
		a := genIntMap(r, n)
		b := genIntMap(r, n)
		for _, k := range slices.Sorted(maps.Keys(a)) {
			hs[0].u64(uint64(k))
		}
		vals := slices.Sorted(maps.Values(a))
		hs[1].u64(uint64(len(vals)))
		for _, v := range vals {
			hs[1].str(v)
		}
		sum := 0
		for k, v := range maps.All(a) {
			sum += k + len(v)
		}
		hs[2].u64(uint64(sum))
		c := maps.Collect(maps.All(a))
		hashIntMap(hs[3], c)
		d := maps.Clone(a)
		hs[5].u64(uint64(len(d)))
		hs[5].u64(uint64(len(a)))
		if a == nil {
			if d != nil {
				hs[5].u64(99)
			}
		} else {
			d[-1] = "extra"
			hs[5].u64(uint64(len(a)))
			hashIntMap(hs[5], a)
			delete(d, -1)
		}
		hs[7].u64(uint64(len(a)))
		eq := maps.Equal(a, d)
		eq2 := maps.Equal(a, b)
		if eq {
			hs[7].u64(1)
		}
		if eq2 {
			hs[7].u64(2)
		}
		if maps.EqualFunc(a, b, func(x, y string) bool { return len(x) == len(y) }) {
			hs[8].u64(1)
		} else {
			hs[8].u64(0)
		}
		e := maps.Clone(a)
		if e != nil {
			maps.Copy(e, b)
			hashIntMap(hs[6], e)
			maps.DeleteFunc(e, func(k int, v string) bool { return k%3 == 0 })
			hashIntMap(hs[9], e)
		}
		f := IntMap{}
		maps.Insert(f, maps.All(a))
		maps.Insert(f, maps.All(b))
		hs[4].u64(uint64(len(f)))
		hashIntMap(hs[4], f)
		g := maps.Clone(f)
		maps.DeleteFunc(g, func(k int, v string) bool { return strings.HasSuffix(v, "5") })
		hashIntMap(hs[12], g)
		// struct keys
		sk := map[key][]int{}
		for i := 0; i < n%30; i++ {
			k := key{r.intn(5), words[r.intn(len(words))]}
			sk[k] = append(sk[k], i)
		}
		cl := maps.Clone(sk)
		for _, k := range sorted(sk, func(x, y key) int {
			if x.a != y.a {
				return x.a - y.a
			}
			return strings.Compare(x.b, y.b)
		}) {
			hs[10].u64(uint64(k.a))
			hs[10].str(k.b)
			hs[10].u64(uint64(len(cl[k])))
		}
		// NaN keys: each one is its own entry; Clone, Copy and Collect keep them
		nm := map[float64]int{}
		for i := 0; i < n%8; i++ {
			nm[math.NaN()] = i
			nm[float64(i)] = i * 2
		}
		nc := maps.Clone(nm)
		hs[11].u64(uint64(len(nm)))
		hs[11].u64(uint64(len(nc)))
		cnt := 0
		for k := range nc {
			if k != k {
				cnt++
			}
		}
		hs[11].u64(uint64(cnt))
		nd := map[float64]int{}
		maps.Copy(nd, nm)
		hs[11].u64(uint64(len(nd)))
		maps.DeleteFunc(nd, func(k float64, v int) bool { return k != k })
		hs[11].u64(uint64(len(nd)))
		hs[11].u64(uint64(len(maps.Collect(maps.All(nm)))))
		if maps.Equal(nm, nm) {
			hs[11].u64(1)
		}
		// early exit
		got := 0
		for k := range maps.Keys(a) {
			got++
			if got == 3 {
				_ = k
				break
			}
		}
		hs[13].u64(uint64(got))
		for range maps.All(nm) {
			got++
			break
		}
		for range maps.Values(nm) {
			got++
			break
		}
		hs[13].u64(uint64(got))
	}
	for i, n := range names {
		fmt.Printf("%-14s %016x\n", n, hs[i].h)
	}
}

var words = []string{"", "a", "b", "ab", "ba", "foo", "bar", "☺", "10"}

// Output:
// Keys           e5c9e8af55401874
// Values         3523db3b2e0baffc
// All            caa10ed9e929cbad
// Collect        de08ba46330b34ff
// Insert         a2eebe1efd85071e
// Clone          b1560f667d4d1db6
// Copy           9fa444215a3840e6
// Equal          f29d822b352bcde4
// EqualFunc      83d37692872f02c4
// DeleteFunc     05fdc4dfbc17d27f
// struct keys    824d0b04bdb680b7
// NaN keys       95a79a21638ff18d
// named type     78cde747e922cdb7
// early exit     c96f2eeadb0bcdc3
