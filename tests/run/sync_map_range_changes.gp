// phase: 08

// Every Range sees the keys that every kind of change left: a Range after Store, LoadOrStore, Swap,
// LoadAndDelete, CompareAndDelete and Clear shows the new contents, also when an earlier Range has
// already walked the map (gocpp keeps the list of the entries between Ranges and must drop it when the
// set of keys changes). A NaN key is shown from that list, so Clear must drop it too.
package main

import (
	"fmt"
	"sync"
)

func keys(m *sync.Map) []int {
	var out []int
	m.Range(func(k, v any) bool {
		if i, ok := k.(int); ok {
			out = append(out, i)
		}
		return true
	})
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func count(m *sync.Map) int {
	n := 0
	m.Range(func(k, v any) bool { n++; return true })
	return n
}

func main() {
	var m sync.Map
	m.Store(1, "a")
	m.Store(2, "b")
	fmt.Println("start:", keys(&m))

	m.Store(3, "c")
	fmt.Println("store new:", keys(&m))

	m.LoadOrStore(4, "d")
	fmt.Println("load-or-store new:", keys(&m))

	m.Swap(5, "e")
	fmt.Println("swap new:", keys(&m))

	m.LoadAndDelete(1)
	fmt.Println("load-and-delete:", keys(&m))

	m.Delete(2)
	fmt.Println("delete:", keys(&m))

	m.CompareAndDelete(3, "c")
	fmt.Println("compare-and-delete:", keys(&m))

	// A changed value shows in the next Range.
	m.Store(4, "D")
	m.Swap(5, "E")
	m.CompareAndSwap(4, "D", "dd")
	got := map[int]any{}
	m.Range(func(k, v any) bool {
		got[k.(int)] = v
		return true
	})
	fmt.Println("values:", got[4], got[5])

	// A NaN key cannot be found or deleted; Clear removes it.
	nan := 0.0
	nan = nan / nan
	m.Store(nan, "nan")
	fmt.Println("with nan:", count(&m))
	m.Clear()
	fmt.Println("after clear:", count(&m))
	m.Store(7, "g")
	fmt.Println("after clear and store:", keys(&m))
}

// Output:
// start: [1 2]
// store new: [1 2 3]
// load-or-store new: [1 2 3 4]
// swap new: [1 2 3 4 5]
// load-and-delete: [2 3 4 5]
// delete: [3 4 5]
// compare-and-delete: [4 5]
// values: dd E
// with nan: 3
// after clear: 0
// after clear and store: [7]
