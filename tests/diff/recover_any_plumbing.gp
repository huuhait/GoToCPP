// phase: 05

package main

import "fmt"

type records struct {
	tags   []string
	values []any
}

func keep(f func(), into *records, tag string) {
	defer func() {
		var r any
		r = recover()
		into.tags = append(into.tags, tag)
		into.values = append(into.values, r)
	}()
	f()
}

func pass(v any) any { return v }

func main() {
	var recs records
	keep(func() { panic("a") }, &recs, "one")
	keep(func() {}, &recs, "two")
	keep(func() { var m map[int]int; m[1] = 2 }, &recs, "three")
	for i, r := range recs.values {
		fmt.Println(recs.tags[i], r, r == nil, r != nil)
	}
	byTag := map[string]any{}
	for i, r := range recs.values {
		byTag[recs.tags[i]] = pass(r)
	}
	fmt.Println(len(byTag), byTag["one"], byTag["two"] == nil)
	list := []any{recs.values[0], recs.values[1], recs.values[2]}
	fmt.Println(len(list), list[0], list[1])
	fn := func() any { return recs.values[0] }
	fmt.Println(fn())
}
