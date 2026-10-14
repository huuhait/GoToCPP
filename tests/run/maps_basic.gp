// phase: 03

// The basic map operations: make (with and without a size hint, also a negative or huge one,
// which Go ignores), literals, m[k] (the zero value for a missing key, and a read never
// inserts), m[k] = v, len, delete (also of a missing key), clear, v, ok in each form, nil maps
// (reads, len, delete, clear and range work), m == nil, and a map as a reference: copies, a
// parameter and a result share one table.
package main

import "fmt"

func sorted(m map[string]int) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func show(name string, m map[string]int) {
	fmt.Print(name, " len=", len(m), " nil=", m == nil, " map[")
	for i, k := range sorted(m) {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(k, ":", m[k])
	}
	fmt.Println("]")
}

func add(m map[string]int, k string) {
	m[k] = len(k)
}

// hintOf returns n: a size hint that is not a constant.
func hintOf(n int) int {
	return n
}

func fresh() map[string]int {
	return map[string]int{"new": 1}
}

func main() {
	var n map[string]int
	show("nil", n)
	fmt.Println(n["x"], len(n))
	delete(n, "x")
	clear(n)
	for k, v := range n {
		fmt.Println("never", k, v)
	}
	v, ok := n["x"]
	fmt.Println(v, ok)

	m := make(map[string]int)
	show("make", m)
	m["a"] = 1
	m["b"] = 2
	m["a"] = 10
	show("set", m)
	fmt.Println(m["zz"], len(m))
	show("read", m)
	delete(m, "zz")
	delete(m, "a")
	show("delete", m)
	clear(m)
	show("clear", m)

	hint := -3
	h1 := make(map[string]int, hint)
	h2 := make(map[string]int, 1<<62)
	var big uint64 = 1 << 63
	h3 := make(map[string]int, big)
	h4 := make(map[string]int, 8)
	h5 := make(map[string]int, hintOf(1<<40))
	h1["x"], h2["y"], h3["z"], h4["w"], h5["v"] = 1, 2, 3, 4, 5
	fmt.Println(len(h1), len(h2), len(h3), len(h4), len(h5), h1["x"]+h2["y"]+h3["z"]+h4["w"]+h5["v"])

	lit := map[string]int{"one": 1, "two": 2, "three": 3}
	show("lit", lit)
	show("empty", map[string]int{})
	show("conv", map[string]int(nil))

	if v, ok := lit["two"]; ok {
		fmt.Println("two is", v)
	}
	var w, found = lit["four"]
	fmt.Println(w, found)
	_, found = lit["one"]
	w, _ = lit["three"]
	fmt.Println(w, found)
	_, ok = n["x"]
	fmt.Println(ok)

	alias := lit
	alias["four"] = 4
	add(lit, "five")
	show("alias", alias)
	alias = fresh()
	show("lit", lit)
	show("fresh", alias)
	alias = nil
	fmt.Println(alias == nil, lit != nil, nil == alias)
}

// Output:
// nil len=0 nil=true map[]
// 0 0
// 0 false
// make len=0 nil=false map[]
// set len=2 nil=false map[a:10 b:2]
// 0 2
// read len=2 nil=false map[a:10 b:2]
// delete len=1 nil=false map[b:2]
// clear len=0 nil=false map[]
// 1 1 1 1 1 15
// lit len=3 nil=false map[one:1 three:3 two:2]
// empty len=0 nil=false map[]
// conv len=0 nil=true map[]
// two is 2
// 0 false
// 3 true
// false
// alias len=5 nil=false map[five:4 four:4 one:1 three:3 two:2]
// lit len=5 nil=false map[five:4 four:4 one:1 three:3 two:2]
// fresh len=1 nil=false map[new:1]
// true true true
