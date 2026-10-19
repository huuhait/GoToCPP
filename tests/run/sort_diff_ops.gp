// phase: 09
// Differential sweep of package sort: pseudo-random data of many shapes (random, few distinct keys, sorted, reversed,
// sawtooth, organ pipe, all equal, with NaN) and sizes (0 to 300, and a few thousand) go through Sort, Stable, Slice,
// SliceStable, Ints, Strings, Float64s, Reverse, IsSorted, the Search family and Find. The data carries an id, so the
// order of equal keys after the unstable sorts (Go's pattern-defeating quicksort is deterministic) is part of the hash.
// One hash per function is printed. The expected output comes from real Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"fmt"
	"math"
	"sort"
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

func newHasher() *hasher { return &hasher{14695981039346656037} }
func (x *hasher) u64(v uint64) {
	for i := 0; i < 8; i++ {
		x.h ^= v & 0xff
		x.h *= 1099511628211
		v >>= 8
	}
}
func (x *hasher) str(s string) {
	for i := 0; i < len(s); i++ {
		x.h ^= uint64(s[i])
		x.h *= 1099511628211
	}
	x.h ^= 0xff
	x.h *= 1099511628211
}

type item struct {
	key int
	id  int
}

type items []item

func (a items) Len() int           { return len(a) }
func (a items) Less(i, j int) bool { return a[i].key < a[j].key }
func (a items) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// counted wraps Interface and counts the calls: the number of Less and Swap calls is part of the hash.
type counted struct {
	sort.Interface
	less, swap int
}

func (c *counted) Less(i, j int) bool { c.less++; return c.Interface.Less(i, j) }
func (c *counted) Swap(i, j int)      { c.swap++; c.Interface.Swap(i, j) }

func shape(r *rngT, kind, n int) []int {
	a := make([]int, n)
	for i := range a {
		switch kind {
		case 0:
			a[i] = r.intn(1 << 30)
		case 1:
			a[i] = r.intn(4)
		case 2:
			a[i] = i
		case 3:
			a[i] = n - i
		case 4:
			a[i] = i % 7
		case 5:
			if i < n/2 {
				a[i] = i
			} else {
				a[i] = n - i
			}
		case 6:
			a[i] = 5
		case 7:
			a[i] = r.intn(n/3 + 1)
		case 8:
			a[i] = i
			if r.intn(10) == 0 {
				a[i] = r.intn(n + 1)
			}
		}
	}
	return a
}

const kinds = 9

func mk(a []int) items {
	it := make(items, len(a))
	for i, k := range a {
		it[i] = item{k, i}
	}
	return it
}

func hashItems(h *hasher, it items) {
	for _, x := range it {
		h.u64(uint64(x.id))
	}
	h.u64(0xdead)
}

func ff(r *rngT, k int) float64 {
	switch r.intn(12) {
	case 0:
		return math.NaN()
	case 1:
		return math.Inf(1)
	case 2:
		return math.Inf(-1)
	case 3:
		return math.Copysign(0, -1)
	case 4:
		return 0
	}
	return float64(r.intn(2000)-1000) / 8
}

func hashFloats(h *hasher, f []float64) {
	for _, x := range f {
		if x != x {
			h.u64(0x7ff8)
		} else {
			h.u64(math.Float64bits(x))
		}
	}
	h.u64(0xbeef)
}

func main() {
	r := &rngT{2024}
	hs := map[string]*hasher{}
	var names []string
	get := func(n string) *hasher {
		h := hs[n]
		if h == nil {
			h = newHasher()
			hs[n] = h
			names = append(names, n)
		}
		return h
	}
	sizes := []int{}
	for n := 0; n <= 100; n++ {
		sizes = append(sizes, n)
	}
	sizes = append(sizes, 127, 128, 129, 255, 256, 257, 300, 1000, 2047, 2048, 2049, 5000)
	for _, n := range sizes {
		for kind := 0; kind < kinds; kind++ {
			base := shape(r, kind, n)

			a := mk(base)
			c := &counted{Interface: a}
			sort.Sort(c)
			hashItems(get("Sort"), a)
			get("SortCalls").u64(uint64(c.less)<<20 | uint64(c.swap))
			get("IsSorted").u64(b2u(sort.IsSorted(a)))

			a = mk(base)
			c = &counted{Interface: a}
			sort.Stable(c)
			hashItems(get("Stable"), a)
			get("StableCalls").u64(uint64(c.less)<<20 | uint64(c.swap))

			a = mk(base)
			sort.Sort(sort.Reverse(a))
			hashItems(get("SortReverse"), a)

			a = mk(base)
			sort.Slice(a, func(i, j int) bool { return a[i].key < a[j].key })
			hashItems(get("Slice"), a)
			get("SliceIsSorted").u64(b2u(sort.SliceIsSorted(a, func(i, j int) bool { return a[i].key < a[j].key })))

			a = mk(base)
			sort.SliceStable(a, func(i, j int) bool { return a[i].key > a[j].key })
			hashItems(get("SliceStable"), a)

			ints := append([]int(nil), base...)
			sort.Ints(ints)
			h := get("Ints")
			for _, v := range ints {
				h.u64(uint64(v))
			}
			get("IntsAreSorted").u64(b2u(sort.IntsAreSorted(ints)))
			sort.Sort(sort.Reverse(sort.IntSlice(ints)))
			h = get("IntSliceReverse")
			for _, v := range ints {
				h.u64(uint64(v))
			}

			// the Search family on the sorted ints
			sort.Ints(ints)
			hsr := get("Search")
			for q := -1; q <= n+1; q += 1 + n/20 {
				hsr.u64(uint64(sort.SearchInts(ints, q)))
				i, found := sort.Find(len(ints), func(i int) int { return q - ints[i] })
				hsr.u64(uint64(i)<<1 | b2u(found))
				hsr.u64(uint64(sort.Search(len(ints), func(i int) bool { return ints[i] >= q })))
				hsr.u64(uint64(sort.IntSlice(ints).Search(q)))
			}

			strs := make([]string, n)
			for i, k := range base {
				strs[i] = fmt.Sprintf("%x", k*2654435761%1000)
			}
			sort.Strings(strs)
			h = get("Strings")
			for _, s := range strs {
				h.str(s)
			}
			get("StringsAreSorted").u64(b2u(sort.StringsAreSorted(strs)))
			hq := get("SearchStrings")
			for _, q := range []string{"", "0", "5", "ff", "zz", "3e7"} {
				hq.u64(uint64(sort.SearchStrings(strs, q)))
				hq.u64(uint64(sort.StringSlice(strs).Search(q)))
			}

			fl := make([]float64, n)
			for i := range fl {
				fl[i] = ff(r, kind)
			}
			fs := append([]float64(nil), fl...)
			sort.Float64s(fs)
			hashFloats(get("Float64s"), fs)
			get("Float64sAreSorted").u64(b2u(sort.Float64sAreSorted(fs)))
			fs = append([]float64(nil), fl...)
			sort.Sort(sort.Reverse(sort.Float64Slice(fs)))
			hashFloats(get("Float64Reverse"), fs)
			fs = append([]float64(nil), fl...)
			sort.Stable(sort.Float64Slice(fs))
			hashFloats(get("Float64Stable"), fs)
			sort.Float64s(fs)
			hf := get("SearchFloat64s")
			for _, q := range []float64{math.Inf(-1), -1000, -5.5, 0, 7.125, 1000, math.Inf(1), math.NaN()} {
				hf.u64(uint64(sort.SearchFloat64s(fs, q)))
				hf.u64(uint64(sort.Float64Slice(fs).Search(q)))
			}
		}
	}

	// a Less that is not a strict weak order must not crash or loop, and must not read out of range; Go's order is the result
	for n := 0; n < 200; n += 7 {
		a := mk(shape(r, 0, n))
		sort.Slice(a, func(i, j int) bool { return a[i].key&1 == 0 })
		hashItems(get("BadLess"), a)
		a = mk(shape(r, 1, n))
		sort.SliceStable(a, func(i, j int) bool { return a[i].key != a[j].key })
		hashItems(get("BadLessStable"), a)
	}

	// panics with Go's text: Slice with a non-slice
	for i, v := range []any{5, "str", nil, struct{}{}, map[int]int{}, [3]int{}, &[]int{1}} {
		func() {
			defer func() { get("SlicePanic").str(fmt.Sprint(i, ":", recover())) }()
			sort.Slice(v, func(i, j int) bool { return false })
		}()
	}
	for _, n := range []int{0, 1, 2, 3} {
		s := make([]string, n)
		sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
		sort.SliceStable(s, func(i, j int) bool { return s[i] < s[j] })
		get("SliceSmall").u64(uint64(n))
	}

	// Search with huge n and the contract of the predicate
	for _, n := range []int{0, 1, 1 << 20, 1<<31 - 1, math.MaxInt64 / 2} {
		for _, k := range []int{0, 1, n / 3, n - 1, n} {
			get("SearchHuge").u64(uint64(sort.Search(n, func(i int) bool { return i >= k })))
		}
	}

	for _, n := range names {
		fmt.Printf("%-18s %016x\n", n, hs[n].h)
	}
}

func b2u(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// Output:
// Sort               dc5edc59eeb9396d
// SortCalls          16ec74260be5d7b1
// IsSorted           0694ef671e5ca324
// Stable             2a1fb2b72d3f3af9
// StableCalls        085cf439a2b6de8b
// SortReverse        f59fcea211b771f5
// Slice              dc5edc59eeb9396d
// SliceIsSorted      0694ef671e5ca324
// SliceStable        99cb4e2125b76515
// Ints               cff07342899decbf
// IntsAreSorted      0694ef671e5ca324
// IntSliceReverse    20173e6d335768f3
// Search             dfb6e12b6f564737
// Strings            99a70546e141682a
// StringsAreSorted   0694ef671e5ca324
// SearchStrings      41c86a293081f1c5
// Float64s           ccde16744c48802b
// Float64sAreSorted  0694ef671e5ca324
// Float64Reverse     2224896bfb5a2a9f
// Float64Stable      2b6f165cc895cf2b
// SearchFloat64s     d3bce120ee56c6f9
// BadLess            254571238e97d9fc
// BadLessStable      5f13123e8bc9af9c
// SlicePanic         7d67b490e80aecf7
// SliceSmall         64dbcbc3ab5bf1a5
// SearchHuge         caa96332c4f7dad2
