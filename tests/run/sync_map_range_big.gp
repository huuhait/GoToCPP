// phase: 08

// A Range of a map with more entries than gocpp keeps a list for (4096) is right too: it shows every
// entry, stops when f says so, sees keys added or removed between two Ranges and a value that changed,
// and a key deleted by f itself is not shown afterwards.
package main

import (
	"fmt"
	"sync"
)

func count(m *sync.Map) int {
	n := 0
	m.Range(func(k, v any) bool {
		n++
		return true
	})
	return n
}

func main() {
	var m sync.Map
	for i := 0; i < 6000; i++ {
		m.Store(i, i)
	}
	fmt.Println("all:", count(&m))

	stopped := 0
	m.Range(func(k, v any) bool {
		stopped++
		return stopped < 3
	})
	fmt.Println("stop after 3:", stopped)

	m.Store(6000, 6000)
	fmt.Println("after store:", count(&m))

	m.Delete(0)
	m.Delete(1)
	fmt.Println("after delete:", count(&m))

	m.Store(2, "two")
	sawTwo := false
	m.Range(func(k, v any) bool {
		if k == 2 {
			sawTwo = v == "two"
		}
		return true
	})
	fmt.Println("changed value seen:", sawTwo)

	// f deletes every key it has not yet seen at the end of the key space: they must not be shown.
	for i := 0; i < 6000; i++ {
		m.Delete(i)
	}
	seen := 0
	m.Range(func(k, v any) bool {
		seen++
		m.Delete(6000)
		return true
	})
	fmt.Println("delete inside Range:", seen)

	m.Clear()
	fmt.Println("after clear:", count(&m))
}

// Output:
// all: 6000
// stop after 3: 3
// after store: 6001
// after delete: 5999
// changed value seen: true
// delete inside Range: 1
// after clear: 0
