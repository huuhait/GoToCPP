// phase: 05

// A key that holds an unhashable dynamic type (step 5.6): read, delete and assignment in a nil,
// an empty and a filled map give Go's texts, also when the interface is a field of a struct
// key or an element of an array key.
package main

import "fmt"

type SS struct{ s []int }
type SA struct {
	a int
	b any
}

func try(name string, f func()) {
	defer func() {
		r := recover()
		if r == nil {
			fmt.Println(name, "no panic")
			return
		}
		e, ok := r.(error)
		if ok {
			fmt.Println(name, "error:", e.Error())
		} else if s, ok := r.(string); ok {
			fmt.Println(name, "string:", s)
		} else {
			fmt.Println(name, "other")
		}
	}()
	f()
}

func main() {
	keys := []struct {
		n string
		k any
	}{{"slice", []int{1}}, {"map", map[int]int{}}, {"func", func() {}}, {"structslice", SS{}}, {"structany", SA{1, []int{1}}}, {"arrany", [1]any{[]int{1}}}, {"int", 1}}
	for _, kk := range keys {
		k := kk.k
		var nilm map[any]int
		emp := map[any]int{}
		non := map[any]int{"a": 1}
		try(kk.n+" nil read", func() { _ = nilm[k] })
		try(kk.n+" nil commaok", func() { _, _ = nilm[k] })
		try(kk.n+" nil delete", func() { delete(nilm, k) })
		try(kk.n+" nil assign", func() { nilm[k] = 1 })
		try(kk.n+" empty read", func() { _ = emp[k] })
		try(kk.n+" empty delete", func() { delete(emp, k) })
		try(kk.n+" empty assign", func() { emp[k] = 1 })
		try(kk.n+" non read", func() { _ = non[k] })
		try(kk.n+" non delete", func() { delete(non, k) })
		try(kk.n+" non assign", func() { non[k] = 1 })
		try(kk.n+" non incr", func() { non[k]++ })
	}
	// struct / array keyed maps containing interfaces
	ms := map[SA]int{}
	try("SA empty read", func() { _ = ms[SA{1, []int{1}}] })
	try("SA empty delete", func() { delete(ms, SA{1, []int{1}}) })
	try("SA empty assign", func() { ms[SA{1, []int{1}}] = 1 })
	var nms map[SA]int
	try("SA nil read", func() { _ = nms[SA{1, []int{1}}] })
	ma := map[[1]any]int{}
	try("arr empty read", func() { _ = ma[[1]any{[]int{1}}] })
	mn := map[SA]int{{1, 2}: 1}
	try("SA non read", func() { _ = mn[SA{1, []int{1}}] })
	mi := map[any]map[any]int{}
	try("nested", func() { _ = mi[1][[]int{1}] })
	var e1 any
	_ = e1
}

// Output:
// slice nil read error: hash of unhashable type: []int
// slice nil commaok error: hash of unhashable type: []int
// slice nil delete error: hash of unhashable type: []int
// slice nil assign error: assignment to entry in nil map
// slice empty read error: hash of unhashable type: []int
// slice empty delete error: hash of unhashable type: []int
// slice empty assign error: runtime error: hash of unhashable type []int
// slice non read error: runtime error: hash of unhashable type []int
// slice non delete error: runtime error: hash of unhashable type []int
// slice non assign error: runtime error: hash of unhashable type []int
// slice non incr error: runtime error: hash of unhashable type []int
// map nil read error: hash of unhashable type: map[int]int
// map nil commaok error: hash of unhashable type: map[int]int
// map nil delete error: hash of unhashable type: map[int]int
// map nil assign error: assignment to entry in nil map
// map empty read error: hash of unhashable type: map[int]int
// map empty delete error: hash of unhashable type: map[int]int
// map empty assign error: runtime error: hash of unhashable type map[int]int
// map non read error: runtime error: hash of unhashable type map[int]int
// map non delete error: runtime error: hash of unhashable type map[int]int
// map non assign error: runtime error: hash of unhashable type map[int]int
// map non incr error: runtime error: hash of unhashable type map[int]int
// func nil read error: hash of unhashable type: func()
// func nil commaok error: hash of unhashable type: func()
// func nil delete error: hash of unhashable type: func()
// func nil assign error: assignment to entry in nil map
// func empty read error: hash of unhashable type: func()
// func empty delete error: hash of unhashable type: func()
// func empty assign error: runtime error: hash of unhashable type func()
// func non read error: runtime error: hash of unhashable type func()
// func non delete error: runtime error: hash of unhashable type func()
// func non assign error: runtime error: hash of unhashable type func()
// func non incr error: runtime error: hash of unhashable type func()
// structslice nil read error: hash of unhashable type: main.SS
// structslice nil commaok error: hash of unhashable type: main.SS
// structslice nil delete error: hash of unhashable type: main.SS
// structslice nil assign error: assignment to entry in nil map
// structslice empty read error: hash of unhashable type: main.SS
// structslice empty delete error: hash of unhashable type: main.SS
// structslice empty assign error: runtime error: hash of unhashable type main.SS
// structslice non read error: runtime error: hash of unhashable type main.SS
// structslice non delete error: runtime error: hash of unhashable type main.SS
// structslice non assign error: runtime error: hash of unhashable type main.SS
// structslice non incr error: runtime error: hash of unhashable type main.SS
// structany nil read error: hash of unhashable type: []int
// structany nil commaok error: hash of unhashable type: []int
// structany nil delete error: hash of unhashable type: []int
// structany nil assign error: assignment to entry in nil map
// structany empty read error: hash of unhashable type: []int
// structany empty delete error: hash of unhashable type: []int
// structany empty assign error: runtime error: hash of unhashable type []int
// structany non read error: runtime error: hash of unhashable type []int
// structany non delete error: runtime error: hash of unhashable type []int
// structany non assign error: runtime error: hash of unhashable type []int
// structany non incr error: runtime error: hash of unhashable type []int
// arrany nil read error: hash of unhashable type: []int
// arrany nil commaok error: hash of unhashable type: []int
// arrany nil delete error: hash of unhashable type: []int
// arrany nil assign error: assignment to entry in nil map
// arrany empty read error: hash of unhashable type: []int
// arrany empty delete error: hash of unhashable type: []int
// arrany empty assign error: runtime error: hash of unhashable type []int
// arrany non read error: runtime error: hash of unhashable type []int
// arrany non delete error: runtime error: hash of unhashable type []int
// arrany non assign error: runtime error: hash of unhashable type []int
// arrany non incr error: runtime error: hash of unhashable type []int
// int nil read no panic
// int nil commaok no panic
// int nil delete no panic
// int nil assign error: assignment to entry in nil map
// int empty read no panic
// int empty delete no panic
// int empty assign no panic
// int non read no panic
// int non delete no panic
// int non assign no panic
// int non incr no panic
// SA empty read error: hash of unhashable type: []int
// SA empty delete error: hash of unhashable type: []int
// SA empty assign error: runtime error: hash of unhashable type []int
// SA nil read error: hash of unhashable type: []int
// arr empty read error: hash of unhashable type: []int
// SA non read error: runtime error: hash of unhashable type []int
// nested error: hash of unhashable type: []int
