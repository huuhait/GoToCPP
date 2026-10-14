// phase: 03

// A map entry is not a variable: its address can't be taken, and an element of an array in a
// map can't be assigned (go/types' errors, as in Go).
package main

func main() {
	m := map[string]int{"a": 1}
	p := &m["a"] // ERROR "invalid operation: cannot take address of m\[\x22a\x22\] \(map index expression of type int\)"
	_ = p
	s := map[string][]int{"a": {1}}
	q := &s["a"][0]
	_ = q
	a := map[string][3]int{"a": {1, 2, 3}}
	a["a"][1] = 5 // ERROR "cannot assign to a\[\x22a\x22\]\[1\] \(neither addressable nor a map index expression\)"
	_ = a["a"][1]
}
