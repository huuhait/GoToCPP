// phase: 09
// Differential sweep of package slices: pseudo-random slices of ints, strings, float64 (with NaN, -0 and infinities)
// and structs with many equal keys go through the functions of the package; one hash per function is printed.
// slices.Sort, SortFunc and SortStableFunc are Go's pattern-defeating quicksort with its xorshift pivot choice, so
// even the order of equal elements that the unstable sort leaves must be Go's: the structs carry a serial number
// that goes into the hash. The expected output comes from real Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"cmp"
	"fmt"
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

func newHasher() *hasher { return &hasher{14695981039346656037} }
func (x *hasher) u64(v uint64) {
	for i := 0; i < 8; i++ {
		x.h ^= (v >> (8 * i)) & 0xff
		x.h *= 1099511628211
	}
}
func (x *hasher) int(n int) { x.u64(uint64(int64(n))) }
func (x *hasher) bool(b bool) {
	if b {
		x.u64(1)
	} else {
		x.u64(0)
	}
}
func (x *hasher) f64(f float64) {
	if f != f {
		x.u64(0x7ff8000000000001) // every NaN is one value
		return
	}
	x.u64(math.Float64bits(f))
}
func (x *hasher) str(s string) {
	x.int(len(s))
	for i := 0; i < len(s); i++ {
		x.h ^= uint64(s[i])
		x.h *= 1099511628211
	}
}
func (x *hasher) ints(a []int) {
	x.int(len(a))
	if a == nil {
		x.int(-1)
	}
	for _, v := range a {
		x.int(v)
	}
}
func (x *hasher) strs(a []string) {
	x.int(len(a))
	for _, v := range a {
		x.str(v)
	}
}
func (x *hasher) f64s(a []float64) {
	x.int(len(a))
	for _, v := range a {
		x.f64(v)
	}
}
func (x *hasher) recs(a []rec) {
	x.int(len(a))
	for _, v := range a {
		x.int(v.key)
		x.int(v.id)
		x.str(v.name)
	}
}

type rec struct {
	key  int
	id   int
	name string
}

// shapes of the inputs: random, few distinct values, sorted, reversed, organ pipe, sawtooth, nearly sorted, all equal
func genInts(r *rngT, n int) []int {
	a := make([]int, n)
	switch r.intn(9) {
	case 0:
		for i := range a {
			a[i] = r.intn(1000) - 500
		}
	case 1:
		for i := range a {
			a[i] = r.intn(4)
		}
	case 2:
		for i := range a {
			a[i] = i
		}
	case 3:
		for i := range a {
			a[i] = n - i
		}
	case 4:
		for i := range a {
			if i < n/2 {
				a[i] = i
			} else {
				a[i] = n - i
			}
		}
	case 5:
		m := r.intn(20) + 1
		for i := range a {
			a[i] = i % m
		}
	case 6:
		for i := range a {
			a[i] = i
		}
		for k := 0; k < n/20+1 && n > 1; k++ {
			a[r.intn(n)], a[r.intn(n)] = a[r.intn(n)], a[r.intn(n)]
		}
	case 7:
		for i := range a {
			a[i] = 7
		}
	default:
		for i := range a {
			a[i] = int(int64(r.next()))
		}
	}
	return a
}

var words = []string{"", "a", "b", "ab", "ba", "abc", "Abc", "zzz", "é", "☺", "foo", "bar", "baz", "foo ", "10", "9", "100"}

func genStrs(r *rngT, n int) []string {
	a := make([]string, n)
	k := 3 + r.intn(len(words)-3)
	for i := range a {
		a[i] = words[r.intn(k)]
		if r.intn(4) == 0 {
			a[i] += words[r.intn(len(words))]
		}
	}
	return a
}

var specials = []float64{0, math.Copysign(0, -1), 1, -1, 0.5, math.Inf(1), math.Inf(-1), math.NaN(), 3, 2, 1e300, -1e-300}

func genF64s(r *rngT, n int) []float64 {
	a := make([]float64, n)
	for i := range a {
		if r.intn(3) == 0 {
			a[i] = specials[r.intn(len(specials))]
		} else {
			a[i] = float64(r.intn(2000)-1000) / 8
		}
	}
	return a
}

func genRecs(r *rngT, n int) []rec {
	a := make([]rec, n)
	keys := 1 + r.intn(12)
	if r.intn(5) == 0 {
		keys = 1 + n
	}
	for i := range a {
		a[i] = rec{key: r.intn(keys), id: i, name: words[r.intn(len(words))]}
	}
	return a
}

func recCmp(a, b rec) int { return cmp.Compare(a.key, b.key) }

func main() {
	type row struct {
		name string
		h    *hasher
	}
	var rows []row
	add := func(name string) *hasher {
		h := newHasher()
		rows = append(rows, row{name, h})
		return h
	}
	hSort := add("Sort ints")
	hSortS := add("Sort strings")
	hSortF := add("Sort floats")
	hSortFunc := add("SortFunc structs")
	hStable := add("SortStableFunc structs")
	hSortFuncRev := add("SortFunc descending")
	hSortedQ := add("IsSorted*")
	hBS := add("BinarySearch")
	hBSF := add("BinarySearchFunc")
	hIndex := add("Index*, Contains*")
	hEq := add("Equal, Compare")
	hMinMax := add("Min, Max")
	hEdit := add("Insert, Delete, Replace")
	hCompact := add("Compact*")
	hRev := add("Reverse, Clone, Clip, Grow")
	hCat := add("Concat, Repeat")
	hIter := add("All, Backward, Values, Collect, Sorted, Chunk")

	r := &rngT{12345}
	for iter := 0; iter < 6000; iter++ {
		n := r.intn(60)
		switch iter % 10 {
		case 8:
			n = 60 + r.intn(400)
		case 9:
			n = 460 + r.intn(3000)
		}
		a := genInts(r, n)
		// Sort
		s := slices.Clone(a)
		slices.Sort(s)
		hSort.ints(s)
		hSortedQ.bool(slices.IsSorted(s))
		hSortedQ.bool(slices.IsSorted(a))
		hSortedQ.bool(slices.IsSortedFunc(a, func(x, y int) int { return cmp.Compare(y, x) }))
		if n > 0 {
			x := a[r.intn(n)]
			i, ok := slices.BinarySearch(s, x)
			hBS.int(i)
			hBS.bool(ok)
			i, ok = slices.BinarySearch(s, x+1)
			hBS.int(i)
			hBS.bool(ok)
			i, ok = slices.BinarySearchFunc(s, x, func(e, t int) int { return cmp.Compare(e, t) })
			hBSF.int(i)
			hBSF.bool(ok)
			hIndex.int(slices.Index(a, x))
			hIndex.bool(slices.Contains(a, x))
			hIndex.int(slices.IndexFunc(a, func(v int) bool { return v > x }))
			hIndex.bool(slices.ContainsFunc(a, func(v int) bool { return v < x-3 }))
		}
		hMinMax.int(len(a))
		if n > 0 {
			hMinMax.int(slices.Min(a))
			hMinMax.int(slices.Max(a))
			hMinMax.int(slices.MinFunc(a, func(x, y int) int { return cmp.Compare(x%7, y%7) }))
			hMinMax.int(slices.MaxFunc(a, func(x, y int) int { return cmp.Compare(x%7, y%7) }))
		}
		b := slices.Clone(a)
		slices.Reverse(b)
		hRev.ints(b)
		hRev.int(cap(slices.Clip(b[:n/2])) - n/2)
		hRev.int(len(slices.Grow(b, 10)))
		hEq.bool(slices.Equal(a, b))
		hEq.int(slices.Compare(a, b))
		hEq.int(slices.Compare(s, a))
		hEq.bool(slices.EqualFunc(a, a, func(x, y int) bool { return x == y }))
		if n > 2 {
			i := r.intn(n)
			j := i + r.intn(n-i+1)
			e1 := slices.Insert(slices.Clone(a), i, 1, 2, 3)
			hEdit.ints(e1)
			e2 := slices.Delete(slices.Clone(a), i, j)
			hEdit.ints(e2)
			e3 := slices.Replace(slices.Clone(a), i, j, 9, 8)
			hEdit.ints(e3)
			e4 := slices.DeleteFunc(slices.Clone(a), func(v int) bool { return v%3 == 0 })
			hEdit.ints(e4)
			hIndex.int(slices.IndexFunc(a, func(v int) bool { return v == a[i] }))
		}
		hCompact.ints(slices.Compact(slices.Clone(s)))
		hCompact.ints(slices.CompactFunc(slices.Clone(a), func(x, y int) bool { return x/10 == y/10 }))
		if n < 100 {
			hCat.ints(slices.Concat(a, s, b[:n/2]))
			hCat.ints(slices.Repeat(a, 1+r.intn(3)))
		}
		for _, c := range slices.Collect(slices.Chunk(a, 1+r.intn(7))) {
			hIter.ints(c)
		}
		for i, v := range slices.All(a) {
			hIter.int(i ^ v)
		}
		for i, v := range slices.Backward(a) {
			hIter.int(i + v)
		}
		hIter.ints(slices.Collect(slices.Values(a)))
		hIter.ints(slices.Sorted(slices.Values(a)))
		hIter.ints(slices.AppendSeq([]int{1}, slices.Values(a)))
		hIter.ints(slices.SortedFunc(slices.Values(a), func(x, y int) int { return cmp.Compare(y, x) }))
		hIter.ints(slices.SortedStableFunc(slices.Values(a), func(x, y int) int { return cmp.Compare(x%5, y%5) }))

		// strings
		if iter%2 == 0 {
			ss := genStrs(r, n%200)
			slices.Sort(ss)
			hSortS.strs(ss)
			if len(ss) > 0 {
				i, ok := slices.BinarySearch(ss, words[r.intn(len(words))])
				hBS.int(i)
				hBS.bool(ok)
				i, ok = slices.BinarySearchFunc(ss, "foo", strings.Compare)
				hBSF.int(i)
				hBSF.bool(ok)
				hMinMax.str(slices.Min(ss))
				hMinMax.str(slices.Max(ss))
			}
			s2 := genStrs(r, n%100)
			slices.SortFunc(s2, func(x, y string) int { return cmp.Compare(len(x), len(y)) })
			hSortFunc.strs(s2)
		}
		// floats with NaN
		if iter%3 == 0 {
			f := genF64s(r, n%300)
			slices.Sort(f)
			hSortF.f64s(f)
			hSortedQ.bool(slices.IsSorted(f))
			g := genF64s(r, n%100)
			if len(g) > 0 {
				hMinMax.f64(slices.Min(g))
				hMinMax.f64(slices.Max(g))
			}
			slices.SortFunc(g, func(x, y float64) int { return cmp.Compare(y, x) })
			hSortF.f64s(g)
			i, ok := slices.BinarySearch(f, 1)
			hBS.int(i)
			hBS.bool(ok)
			hIndex.int(slices.Index(g, math.NaN()))
		}
		// structs, unstable and stable
		recs := genRecs(r, n)
		u := slices.Clone(recs)
		slices.SortFunc(u, recCmp)
		hSortFunc.recs(u)
		st := slices.Clone(recs)
		slices.SortStableFunc(st, recCmp)
		hStable.recs(st)
		rv := slices.Clone(recs)
		slices.SortFunc(rv, func(x, y rec) int { return cmp.Or(cmp.Compare(y.key, x.key), strings.Compare(x.name, y.name)) })
		hSortFuncRev.recs(rv)
		hSortedQ.bool(slices.IsSortedFunc(u, recCmp))
		hSortedQ.bool(slices.IsSortedFunc(recs, recCmp))
		i, ok := slices.BinarySearchFunc(st, 3, func(e rec, t int) int { return cmp.Compare(e.key, t) })
		hBSF.int(i)
		hBSF.bool(ok)
		hCompact.recs(slices.CompactFunc(slices.Clone(st), func(x, y rec) bool { return x.key == y.key }))
	}
	for _, r := range rows {
		fmt.Printf("%-52s %016x\n", r.name, r.h.h)
	}
}

// Output:
// Sort ints                                            70b3ba5486594575
// Sort strings                                         3544490bda970133
// Sort floats                                          9aaf13e836b056c6
// SortFunc structs                                     0b8f4389fe476b93
// SortStableFunc structs                               33b971c0ed73c454
// SortFunc descending                                  a5e4764b142a2608
// IsSorted*                                            fdccbe7be625cd04
// BinarySearch                                         5a060c37f67e2aaa
// BinarySearchFunc                                     cbde9b4f191c1bc9
// Index*, Contains*                                    a1f44a3b61067258
// Equal, Compare                                       5a3ebacb8d33952c
// Min, Max                                             3b0e59c961b847c1
// Insert, Delete, Replace                              c0422bac07272bc0
// Compact*                                             80df50f94cd39e9e
// Reverse, Clone, Clip, Grow                           a484ff86fdd0ab79
// Concat, Repeat                                       aed8794070553f9b
// All, Backward, Values, Collect, Sorted, Chunk        21fc57db3c6c1f6a
