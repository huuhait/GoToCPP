// phase: 09
// maps.Clone on big and awkward maps: 100000 entries with string keys and values (counted values copied slot by
// slot), a table full of tombstones, int maps (the block is copied as bytes), struct and slice and pointer values,
// NaN keys, named map types, nil and empty maps, a clone made while a range is in progress, and clones that are
// changed or emptied while the original stays as it was. The expected output comes from real Go 1.25
// (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
)

type rngT struct{ s uint64 }

func (r *rngT) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

type hasher struct{ h uint64 }

func (x *hasher) str(s string) {
	for i := 0; i < len(s); i++ {
		x.h ^= uint64(s[i])
		x.h *= 1099511628211
	}
	x.h ^= 0xff
	x.h *= 1099511628211
}

func hashSS(m map[string]string) uint64 {
	h := &hasher{14695981039346656037}
	keys := slices.Sorted(maps.Keys(m))
	h.str(strconv.Itoa(len(m)))
	for _, k := range keys {
		h.str(k)
		h.str(m[k])
	}
	return h.h
}

func hashII(m map[int]int) uint64 {
	h := &hasher{14695981039346656037}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		h.str(strconv.Itoa(k))
		h.str(strconv.Itoa(m[k]))
	}
	return h.h
}

type Named map[string][]int
type pt struct {
	a, b int
	s    string
}
type IntMap map[int]int

func main() {
	r := &rngT{2024}
	big := make(map[string]string)
	for i := 0; i < 100000; i++ {
		k := "key-" + strconv.FormatUint(r.next()%300000, 36)
		big[k] = "value-" + strconv.Itoa(i) + "-" + k
	}
	// make tombstones: delete a third of the keys
	for n, k := range slices.Sorted(maps.Keys(big)) {
		if n%3 == 0 {
			delete(big, k)
		}
	}
	c := maps.Clone(big)
	fmt.Println("big:", len(big), len(c), hashSS(big) == hashSS(c), hashSS(big))
	// independence
	c["new"] = "x"
	delete(c, slices.Sorted(maps.Keys(c))[0])
	clear(c)
	fmt.Println("after changes:", len(big), len(c), hashSS(big))
	c2 := maps.Clone(big)
	for k := range big {
		delete(big, k)
		break
	}
	fmt.Println("original changed:", len(big), len(c2), len(big) == len(c2)-1)
	// grow the clone: the block is full-sized and must still grow and rehash correctly
	for i := 0; i < 50000; i++ {
		c2["g"+strconv.Itoa(i)] = strconv.Itoa(i)
	}
	fmt.Println("grown clone:", len(c2), c2["g49999"], c2["g0"])

	// ints: a block of plain bytes
	ints := map[int]int{}
	for i := 0; i < 200000; i++ {
		ints[int(r.next()%1000000)] = i
	}
	ic := maps.Clone(ints)
	fmt.Println("ints:", len(ints), len(ic), hashII(ints) == hashII(ic), hashII(ic))
	im := IntMap{1: 2, 3: 4}
	imc := maps.Clone(im)
	imc[5] = 6
	fmt.Printf("%T %v %v\n", imc, im, imc)

	// struct, slice and pointer values: the copy shares what the values refer to, as an assignment does
	sv := map[pt]*pt{}
	for i := 0; i < 1000; i++ {
		sv[pt{i, i * 2, strconv.Itoa(i)}] = &pt{i, 0, "v"}
	}
	svc := maps.Clone(sv)
	k0 := pt{7, 14, "7"}
	svc[k0].b = 99
	fmt.Println("pointers shared:", sv[k0].b, len(svc))
	nm := Named{"a": {1, 2}, "b": {3}}
	nmc := maps.Clone(nm)
	nmc["a"][0] = 100
	nmc["c"] = nil
	fmt.Printf("%T %v %v\n", nmc, nm, len(nmc))

	// nil and empty maps
	var nilmap map[string]int
	fmt.Println(maps.Clone(nilmap) == nil, maps.Clone(map[string]int{}) != nil, len(maps.Clone(map[string]int{})))
	e := maps.Clone(map[int]string{})
	e[1] = "one"
	fmt.Println(e)

	// NaN keys stay separate entries
	f := map[float64]int{}
	for i := 0; i < 10; i++ {
		f[math.NaN()] = i
		f[float64(i)] = i * i
	}
	fc := maps.Clone(f)
	nans, sum := 0, 0
	for k, v := range fc {
		if k != k {
			nans++
			sum += v
		}
	}
	fmt.Println("NaN:", len(f), len(fc), nans, sum, fc[3])
	clear(fc)
	fmt.Println(len(fc), len(f))

	// a clone made in the middle of a range over the original; the original grows meanwhile
	m := map[int]int{}
	for i := 0; i < 1000; i++ {
		m[i] = i
	}
	var snapshot map[int]int
	seen := 0
	for k := range m {
		seen++
		if seen == 10 {
			snapshot = maps.Clone(m)
		}
		if seen < 500 {
			m[k+100000] = 1
		}
	}
	fmt.Println("range:", len(snapshot), len(m) > 1000, snapshot[999])

	// many small clones, each changed
	total := 0
	for i := 0; i < 20000; i++ {
		s := map[string]int{"a": i, "b": i + 1}
		d := maps.Clone(s)
		d["c"] = i
		total += len(s) + len(d) + d["a"] - s["b"]
	}
	fmt.Println("small:", total)
}

// Output:
// big: 56677 56677 true 11690800732796213138
// after changes: 56677 0 11690800732796213138
// original changed: 56676 56677 true
// grown clone: 106677 49999 0
// ints: 181261 181261 true 16682979367242905914
// main.IntMap map[1:2 3:4] map[1:2 3:4 5:6]
// pointers shared: 99 1000
// main.Named map[a:[100 2] b:[3]] 3
// true true 0
// map[1:one]
// NaN: 20 20 10 45 9
// 0 20
// range: 1009 true 999
// small: 80000
