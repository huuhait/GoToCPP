// sort (Go's own package): sort.Ints/Strings/Float64s (which call the generic slices.Sort),
// sort.Sort with an Interface implementation, sort.IsSorted, sort.Search and SearchInts,
// sort.Reverse (which embeds Interface and promotes Len), and sort.Slice/SliceStable with a
// less closure (which go through any and internal/reflectlite).
package main

import "sort"

type byLen []string

func (b byLen) Len() int           { return len(b) }
func (b byLen) Less(i, j int) bool { return len(b[i]) < len(b[j]) }
func (b byLen) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

type pair struct {
	key string
	n   int
}

func main() {
	a := []int{5, 2, 4, 1, 3}
	sort.Ints(a)
	println(a[0], a[1], a[2], a[3], a[4])
	println(sort.IntsAreSorted(a))
	println(sort.SearchInts(a, 3), sort.SearchInts(a, 6))
	ss := []string{"pear", "apple", "fig"}
	sort.Strings(ss)
	println(ss[0], ss[1], ss[2])
	fs := []float64{2.5, 0.5, 1.5}
	sort.Float64s(fs)
	println(fs[0], fs[2])
	bl := byLen{"ccc", "a", "bb"}
	sort.Sort(bl)
	println(bl[0], bl[1], bl[2])
	println(sort.IsSorted(sort.IntSlice(a)))
	r := sort.Reverse(sort.IntSlice(a))
	println(r.Less(0, 1))
	ps := []pair{{"b", 2}, {"a", 1}, {"c", 3}}
	sort.Slice(ps, func(i, j int) bool { return ps[i].key < ps[j].key })
	println(ps[0].key, ps[1].key, ps[2].key)
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].n < ps[j].n })
	println(ps[0].n, ps[1].n, ps[2].n)
	println(sort.Search(5, func(i int) bool { return i >= 3 }))
}
