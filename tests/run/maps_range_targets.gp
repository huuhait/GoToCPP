// phase: 03

// Range loops whose targets are not variables (s[f()], m[k]) and whose target operands call a
// function: the call runs before the values are stored, also when it deletes the current entry
// of the map being ranged over, clears it or stores a new value into it (the loop reads the
// value after the call, as Go does: the new value, or the zero value of a deleted entry; it
// never reads freed memory). Range over an integer into a map entry and an element.
package main

import "fmt"

var ints = map[int]int{7: 70}
var strs = map[string]string{"a long key that does not fit in a short string": "v"}
var floats = map[float64]bool{1.5: true}
var cur = map[string]int{"k": 1}
var calls int

func delInt() int {
	calls++
	delete(ints, 7)
	return 0
}

func delStr() int {
	calls++
	delete(strs, "a long key that does not fit in a short string")
	return 0
}

func clearFloats() int {
	calls++
	clear(floats)
	return 0
}

func store() int {
	calls++
	cur["k"] = 99
	return 0
}

func name(i int) string {
	return string(rune('0' + i))
}

func main() {
	ks, vs := []int{0}, []int{-1}
	for ks[delInt()], vs[0] = range ints {
	}
	fmt.Println(ks[0], vs[0], len(ints))

	s := []string{""}
	var v string
	for s[delStr()], v = range strs {
		fmt.Println(len(v))
	}
	fmt.Println(len(strs))

	f := []float64{0}
	t := true
	for f[clearFloats()], t = range floats {
	}
	fmt.Println(t, len(floats))

	dst := map[string]int{}
	for _, dst[name(store())] = range cur {
	}
	fmt.Println(dst["0"], cur["k"], calls)

	cnt := map[string]int{}
	for cnt["i"] = range 5 {
		cnt["n"]++
	}
	fmt.Println(cnt["i"], cnt["n"])
	el := []int{0, 0}
	for el[store()] = range 3 {
	}
	fmt.Println(el[0], el[1], calls)
}

// Output:
// 7 0 0
// 0
// 0
// false 0
// 99 99 4
// 4 5
// 2 0 7
