// phase: 08
// divergence: D1

// The list of the entries that Range keeps (gocpp) must not keep a key or a value alive after it was
// deleted, overwritten or cleared: the weak pointers turn nil at once (D1: in Go after a collection,
// maybe later), so the program prints true in gocpp.
package main

import (
	"fmt"
	"sync"
	"weak"
)

type key struct{ n int }

func walk(m *sync.Map) {
	m.Range(func(k, v any) bool { return true })
}

func main() {
	var m sync.Map
	a, b, c := &key{1}, &key{2}, &key{3}
	wa, wb, wc := weak.Make(a), weak.Make(b), weak.Make(c)
	m.Store(a, 1)
	m.Store(b, 2)
	m.Store(c, 3)
	walk(&m)

	m.Delete(a)
	a = nil
	fmt.Println("deleted key freed:", wa.Value() == nil)

	m.CompareAndDelete(b, 2)
	b = nil
	fmt.Println("compare-and-deleted key freed:", wb.Value() == nil)

	m.Clear()
	c = nil
	fmt.Println("cleared key freed:", wc.Value() == nil)

	// A value that Swap replaced is not kept either.
	v1, v2 := &key{10}, &key{11}
	w1 := weak.Make(v1)
	m.Store("k", v1)
	walk(&m)
	m.Swap("k", v2)
	v1 = nil
	fmt.Println("swapped value freed:", w1.Value() == nil)
}

// Output:
// deleted key freed: true
// compare-and-deleted key freed: true
// cleared key freed: true
// swapped value freed: true
