// phase: 03

// Maps of structs and of pointers to structs: m[k].f can't be assigned, so a value is read,
// changed and stored back; through pointers the changes are in place. Nested maps made on
// demand, and counting with struct keys.
package main

import "fmt"

type Stat struct {
	n   int
	sum float64
}

type Key struct {
	city string
	year int
}

func main() {
	vals := map[string]Stat{}
	for _, r := range []struct {
		k string
		v float64
	}{{"a", 1.5}, {"b", 2}, {"a", 2.5}, {"a", 1}} {
		s := vals[r.k]
		s.n++
		s.sum += r.v
		vals[r.k] = s
	}
	fmt.Println(vals, vals["a"].sum/float64(vals["a"].n))

	ptrs := map[string]*Stat{}
	for _, k := range []string{"x", "y", "x", "x"} {
		if ptrs[k] == nil {
			ptrs[k] = &Stat{}
		}
		ptrs[k].n++
	}
	fmt.Println(ptrs["x"].n, ptrs["y"].n, len(ptrs), ptrs["z"] == nil)

	nested := map[string]map[string]int{}
	add := func(a, b string) {
		inner, ok := nested[a]
		if !ok {
			inner = map[string]int{}
			nested[a] = inner
		}
		inner[b]++
	}
	add("go", "fast")
	add("go", "fast")
	add("cpp", "fast")
	add("go", "simple")
	fmt.Println(nested, len(nested["go"]), nested["rust"]["x"])

	visits := map[Key]int{}
	for _, k := range []Key{{"hn", 2024}, {"sg", 2025}, {"hn", 2024}, {"hn", 2025}} {
		visits[k]++
	}
	fmt.Println(visits, visits[Key{"hn", 2024}])
}

// Output:
// map[a:{3 5} b:{1 2}] 1.6666666666666667
// 3 1 2 true
// map[cpp:map[fast:1] go:map[fast:2 simple:1]] 2 0
// map[{hn 2024}:2 {hn 2025}:1 {sg 2025}:1] 2
