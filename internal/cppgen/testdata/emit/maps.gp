// Maps (Phase 03 step 3.7): gp::map<K, V> for keys of every comparable type the emitter has
// (bools, integers, floats with NaN and -0, strings, pointers) and values of every type (also
// slices and maps), nil maps (reads, len, delete, clear and range work; m == nil), make with
// and without a size hint, literals (gc stores the constant entries first, nested literals with
// elided types), m[k] read (the zero value, never an insert), m[k] = v, op= and ++ (one entry
// taken), v, ok := m[k] and v, ok = m[k], delete, len, clear, m[k] = append(m[k], x) (the entry
// taken once), range in every form (over a snapshot: a deleted key is skipped, a changed value
// is seen), Go 1.22 per-iteration range variables, maps as parameters, results and package
// variables, a boxed map variable and a pointer to a map. gocpp's iteration order is not Go's
// (D8), so the maps print through show and showInts, which sort the keys (fmt prints maps in
// step 3.15).
package main

import "fmt"

var gnil map[string]int
var gm = map[string]int{"one": 1, "two": 2}
var gcount = make(map[int]int, 4)

func sortStrs(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func sortInts(s []int) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func show(m map[string]int) {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sortStrs(keys)
	fmt.Print("map[")
	for i, k := range keys {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(k, ":", m[k])
	}
	fmt.Println("]", len(m), m == nil)
}

func showInts(m map[int]int) {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortInts(keys)
	fmt.Print("map[")
	for i, k := range keys {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(k, ":", m[k])
	}
	fmt.Println("]", len(m))
}

// count returns a new map of the words' counts: a map is a result like any value.
func count(words []string) map[string]int {
	m := map[string]int{}
	for _, w := range words {
		m[w]++
	}
	return m
}

// fill changes the caller's map: a map is a reference.
func fill(m map[string]int, n int) {
	for i := range n {
		m[string(rune('a'+i))] = i
	}
}

func main() {
	// nil maps: reads, len, delete, clear and range are fine.
	show(gnil)
	fmt.Println(gnil["x"], len(gnil), gnil == nil, nil != gnil)
	delete(gnil, "x")
	clear(gnil)
	for range gnil {
		fmt.Println("never")
	}
	v, ok := gnil["x"]
	fmt.Println(v, ok)
	show(gm)

	// make, set, get, op=, ++, --, delete, len, clear.
	m := make(map[string]int)
	m2 := make(map[string]int, 100)
	fmt.Println(m == nil, len(m), len(m2))
	m["a"] = 1
	m["b"] = 2
	m["a"] += 10
	m["c"]++
	m["d"]--
	m["b"] *= 7
	m["b"] -= 4
	m["e"] |= 6
	m["e"] &^= 2
	m["e"] ^= 1
	m["f"] = 100
	m["f"] /= 7
	m["g"] = 100
	m["g"] %= 7
	m["h"] = 3
	m["h"] <<= 4
	m["h"] >>= 1
	show(m)
	fmt.Println(m["zz"], len(m))
	delete(m, "zz")
	delete(m, "a")
	show(m)
	clear(m)
	show(m)

	// v, ok in every form.
	m["x"] = 5
	if v, ok := m["x"]; ok {
		fmt.Println("x", v)
	}
	var w, found = m["y"]
	fmt.Println(w, found)
	_, found = m["x"]
	w, _ = m["x"]
	fmt.Println(w, found)

	// Small integers wrap; strings append; floats.
	small := map[bool]uint8{}
	small[true] += 200
	small[true] += 100
	small[false]--
	fmt.Println(small[true], small[false], len(small))
	i8 := map[int8]int8{1: 127}
	i8[1]++
	i8[2] -= 127
	i8[2]--
	i8[2]--
	fmt.Println(i8[1], i8[2])
	strs := map[string]string{}
	strs["k"] += "ab"
	strs["k"] += "cd"
	fmt.Println(strs["k"], len(strs["k"]), strs["none"] == "")
	fl := map[float32]float64{0.5: 1}
	fl[0.5] /= 4
	fl[1.5] *= 2
	fmt.Println(fl[0.5], fl[1.5], len(fl))
	u := map[uint64]uint{}
	u[1<<63] = 1 << 63
	u[1<<63] += 1 << 63
	fmt.Println(u[1<<63], len(u))

	// Float keys: +0 and -0 are one key (the later store keeps its key), NaN is never found.
	zero := 0.0
	negz := -zero
	nan := zero / zero
	fk := map[float64]int{}
	fk[negz] = 1
	fk[zero] = 2
	fk[nan] = 3
	fk[nan] = 4
	_, hasNaN := fk[nan]
	fmt.Println(len(fk), fk[0], hasNaN)
	nans := 0
	for k, v := range fk {
		if k != k {
			nans += v
		} else {
			fmt.Println("zero key", 1/k, v)
		}
	}
	fmt.Println(nans)
	delete(fk, nan)
	fmt.Println(len(fk))
	clear(fk)
	fmt.Println(len(fk))

	// Literals: the constant entries are stored first, so -0 wins over 0 here, as in Go.
	lz := map[float64]int{negz: 1, 0: 2}
	for k, v := range lz {
		fmt.Println("literal key", 1/k, v)
	}
	lit := map[int]int{3: 30, 1: 10, 2: 20}
	showInts(lit)
	empty := map[string]int{}
	fmt.Println(len(empty), empty == nil)
	showInts(map[int]int(nil))

	// Pointer keys and values.
	a, b := 1, 2
	pk := map[*int]string{&a: "a", &b: "b", nil: "nil"}
	fmt.Println(pk[&a], pk[&b], pk[nil], len(pk))
	pv := map[string]*int{"a": &a}
	*pv["a"] = 10
	fmt.Println(a, pv["b"] == nil)

	// Maps of slices: the entry is taken once for m[k] = append(m[k], ...).
	groups := map[int][]string{}
	for _, word := range []string{"go", "cpp", "map", "is", "ok", "range"} {
		groups[len(word)] = append(groups[len(word)], word)
	}
	byLen := map[int]int{}
	for n, ws := range groups {
		byLen[n] = len(ws)
	}
	showInts(byLen)
	fmt.Println(groups[2][0], groups[2][1], groups[2][2], groups[5][0], len(groups[4]))
	gs := map[string][]int{"x": {1, 2}}
	gs["x"] = append(gs["x"], gs["x"]...)
	gs["y"] = append(gs["y"])
	fmt.Println(len(gs["x"]), gs["x"][3], gs["y"] == nil, len(gs))
	gs["x"][0] = 100
	fmt.Println(gs["x"][0])

	// Maps of maps, with elided types in the literal.
	mm := map[string]map[string]int{"a": {"b": 1}, "c": {}}
	mm["a"]["z"] = 26
	mm["c"]["d"] += 4
	show(mm["a"])
	show(mm["c"])
	show(mm["none"])
	ms := map[string][]int{"p": {1, 2, 3}, "q": nil}
	fmt.Println(len(ms["p"]), ms["q"] == nil, len(ms))

	// Range: every form; a deleted key is skipped, a changed value is seen.
	r := map[int]int{1: 10, 2: 20, 3: 30, 4: 40}
	total, keys := 0, 0
	for k, v := range r {
		total += v
		keys += k
	}
	fmt.Println(total, keys)
	n := 0
	for range r {
		n++
	}
	fmt.Println(n)
	for _, v := range r {
		total -= v
	}
	fmt.Println(total)
	var rk, rv int
	sumK, sumV := 0, 0
	for rk, rv = range r {
		sumK += rk
		sumV += rv
	}
	fmt.Println(sumK, sumV, rk > 0, rv > 0)
	seen := 0
	for k := range r {
		// Each key deletes the other key of its pair (1 and 2, 3 and 4): only one of them is
		// seen, whatever the order.
		delete(r, k-1+2*(k%2))
		seen++
	}
	fmt.Println(seen, len(r))
	vals := map[string]int{"a": 1, "b": 1}
	got := 0
	for k, v := range vals {
		// The other key's value is changed before it is reached: its v is the new value.
		for o := range vals {
			if o != k {
				vals[o] = 5
			}
		}
		got += v
	}
	fmt.Println(got)

	// Go 1.22: each iteration has its own range variables.
	var ptrs []*int
	for _, v := range map[string]int{"a": 1, "b": 2, "c": 3} {
		ptrs = append(ptrs, &v)
	}
	sum := 0
	for _, p := range ptrs {
		sum += *p
	}
	fmt.Println(sum)

	// Package maps, parameters, results, a boxed map and a pointer to a map.
	gm["three"] = 3
	gm["one"] += 10
	show(gm)
	for i := range 10 {
		gcount[i%3]++
	}
	showInts(gcount)
	show(count([]string{"a", "b", "a", "c", "a"}))
	fm := map[string]int{}
	fill(fm, 3)
	show(fm)
	boxed := map[string]int{"k": 1}
	pb := &boxed
	(*pb)["k"] += 5
	(*pb)["j"] = 2
	show(boxed)
	*pb = nil
	fmt.Println(boxed == nil, len(*pb))
	pm := new(map[int]bool)
	fmt.Println(*pm == nil, (*pm)[1])
	gnil = map[string]int{}
	gnil["now"] = 1
	show(gnil)
}
