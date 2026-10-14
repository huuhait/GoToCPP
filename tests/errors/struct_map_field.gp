// phase: 03

// A struct read from a map is a copy, not a variable: its fields can't be assigned or have
// their address taken (go/types' errors, as in Go). Through a map of pointers they can.
package main

type P struct{ x int }

func main() {
	m := map[string]P{"a": {1}}
	m["a"].x = 2  // ERROR "cannot assign to struct field m\[\x22a\x22\]\.x in map"
	m["a"].x++    // ERROR "cannot assign to struct field m\[\x22a\x22\]\.x in map"
	_ = &m["a"].x // ERROR "invalid operation: cannot take address of m\[\x22a\x22\]\.x \(value of type int\)"
	pm := map[string]*P{"a": {1}}
	pm["a"].x = 2
	_ = &pm["a"].x
}
