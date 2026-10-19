// phase: 08

// Keys are `any`: an unhashable key panics like a map[any]any does, and CompareAndSwap panics when
// the stored value can't be compared. (Real Go leaves an internal lock held after a panic in the
// middle of Store or CompareAndSwap, so each of those runs on a map of its own, once.)
package main

import (
	"fmt"
	"sync"
)

func try(name string, f func()) {
	defer func() { fmt.Println(name+":", recover()) }()
	f()
}

func main() {
	var m sync.Map
	try("load slice key", func() { m.Load([]int{1}) })
	try("load map key", func() { m.Load(map[string]int{}) })
	try("delete func key", func() { m.Delete(func() {}) })
	m.Store("k", []int{1, 2})
	try("cas missing key", func() { fmt.Println(m.CompareAndSwap("none", []int{1}, 1)) })

	var m1 sync.Map
	m1.Store("k", []int{1, 2})
	try("cas uncomparable old", func() { m1.CompareAndSwap("k", []int{1, 2}, 1) })
	var m2 sync.Map
	try("store slice key", func() { m2.Store([]int{1}, 1) })

	var m3 sync.Map
	m3.Store(struct{ a, b int }{1, 2}, "struct key")
	v, ok := m3.Load(struct{ a, b int }{1, 2})
	fmt.Println(v, ok)
	m3.Store(1, "int")
	m3.Store(int64(1), "int64")
	m3.Store(1.0, "float")
	for _, k := range []any{1, int64(1), 1.0} {
		v, _ := m3.Load(k)
		fmt.Println(v)
	}
}

// Output:
// load slice key: runtime error: hash of unhashable type []int
// load map key: runtime error: hash of unhashable type map[string]int
// delete func key: runtime error: hash of unhashable type func()
// false
// cas missing key: <nil>
// cas uncomparable old: runtime error: comparing uncomparable type []int
// store slice key: runtime error: hash of unhashable type []int
// struct key true
// int
// int64
// float
