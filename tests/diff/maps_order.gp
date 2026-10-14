// phase: 03

// Go's order of evaluation (gc's) with maps: m[k] is read after the calls of its statement
// (also of a local map that shares its table with a package map), but a bool is copied in its
// place as an argument of fmt, and so is m[string(b)]; len(m) is evaluated in its place; a store
// evaluates the value (and the key's calls) before it takes the entry, and reads the map variable
// after them; op= takes the entry after its value; a map literal stores its entries one by one;
// m[k1], m[k2] = m[k2], m[k1] swaps, and m, m[k] = n, v stores into the old m. Maps print sorted.
package main

import "fmt"

var gm = map[string]int{"a": 1, "b": 2}
var gx = 10
var gb = map[string]bool{"t": true}
var shared map[string]int

func f() int {
	fmt.Print("f ")
	gm["a"] += 100
	gx++
	gb["t"] = !gb["t"]
	if shared != nil {
		shared["n"] = len(shared) * 7
		delete(shared, "gone")
	}
	return gx
}

func del() int {
	fmt.Print("del ")
	delete(gm, "a")
	return 5
}

func swap() int {
	fmt.Print("swap ")
	gm = map[string]int{"new": 1}
	return 3
}

func key(s string) string {
	fmt.Print("key:", s, " ")
	return s
}

func show(m map[string]int) {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	fmt.Print("{")
	for i, k := range keys {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(k, ":", m[k])
	}
	fmt.Println("}")
}

func reset() {
	gm = map[string]int{"a": 1, "b": 2}
	gx = 10
}

func main() {
	fmt.Println(gm["a"], f())
	reset()
	fmt.Println(gm["a"]+1, f())
	reset()
	fmt.Println(gb["t"], f())
	m := map[string]int{"x": 1, "gone": 2}
	shared = m
	fmt.Println(len(m), f(), len(m))
	fmt.Println(m["n"], f(), m["n"])
	b := []byte("x")
	fmt.Println(m[string(b)], f())
	shared = nil

	reset()
	gm["a"] += del()
	show(gm)
	reset()
	gm["z"] = swap()
	show(gm)
	reset()
	gm["a"] = gm["b"] + del()
	show(gm)

	reset()
	lit := map[string]int{"a": gx, "b": f(), "c": gx}
	show(lit)
	reset()
	lit2 := map[string]int{key("k1"): gx, key("k2"): f()}
	show(lit2)
	reset()
	ls := map[string][]int{"a": {gx, f()}, "b": {gx}}
	fmt.Println(ls["a"][0], ls["a"][1], ls["b"][0])
	reset()
	fmt.Println(len(map[string]int{"q": gx}), f(), gx)

	reset()
	mm := map[string]int{"p": 1, "q": 2}
	mm["p"], mm["q"] = mm["q"], mm["p"]
	show(mm)
	k := "p"
	k, mm[k] = "q", 50
	show(mm)
	old := mm
	mm, mm["r"] = map[string]int{}, 7
	show(old)
	show(mm)
	x := 0
	x, gm[key("w")] = 1, f()
	fmt.Println(x)
	show(gm)

	reset()
	ms := map[string][]int{"s": {1}}
	ms["s"] = append(ms["s"], gx, len(ms["s"]))
	fmt.Println(len(ms["s"]), ms["s"][0], ms["s"][1], ms["s"][2])
}
