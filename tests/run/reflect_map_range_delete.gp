// phase: 09

// MapIter over a map that changes during the iteration (step 9.3 lows): as in Go and in the
// range statement, an entry deleted before the iterator reaches it is not produced, Value reads
// the current value, and a NaN key is produced although it cannot be looked up.
package main

import (
	"fmt"
	"math"
	"reflect"
)

func main() {
	// deleting every key in the first step ends the iteration after one entry
	m := map[int]int{1: 1, 2: 2, 3: 3, 4: 4}
	it := reflect.ValueOf(m).MapRange()
	count := 0
	for it.Next() {
		count++
		for key := range m {
			delete(m, key)
		}
	}
	fmt.Println("produced after deleting all:", count)

	// a new value for the current key is seen by Value
	m = map[int]int{1: 10}
	it = reflect.ValueOf(m).MapRange()
	for it.Next() {
		m[int(it.Key().Int())] = 99
		fmt.Println("value after set:", it.Value().Int())
	}

	// A key deleted before it is reached is skipped, in whichever order the keys come: an
	// entry deletes both neighbours of its key, so two neighbours are never both produced.
	m = map[int]int{}
	for i := 0; i < 100; i++ {
		m[i] = i
	}
	it = reflect.ValueOf(m).MapRange()
	seen := map[int]bool{}
	for it.Next() {
		key := int(it.Key().Int())
		seen[key] = true
		delete(m, key+1)
		delete(m, key-1)
	}
	neighbours := 0
	for key := range seen {
		if seen[key+1] {
			neighbours++
		}
	}
	fmt.Println("some keys were skipped:", len(seen) < 100)
	fmt.Println("neighbours produced:", neighbours)

	// clear during the iteration
	m = map[int]int{1: 1, 2: 2, 3: 3}
	it = reflect.ValueOf(m).MapRange()
	count = 0
	for it.Next() {
		count++
		clear(m)
	}
	fmt.Println("produced after clear:", count)

	// NaN keys stay in the map and are all produced
	nan := map[float64]int{math.NaN(): 1, math.NaN(): 2, 1: 3}
	count = 0
	total := 0
	for it := reflect.ValueOf(nan).MapRange(); it.Next(); {
		count++
		total += int(it.Value().Int())
	}
	fmt.Println("NaN keys:", count, total)

	// clear in the loop over NaN keys: they go with the clear, only the first is produced
	nan = map[float64]int{math.NaN(): 1, math.NaN(): 2}
	count = 0
	for it := reflect.ValueOf(nan).MapRange(); it.Next(); {
		count++
		clear(nan)
	}
	fmt.Println("NaN keys and clear:", count)

	// Value of an entry that was deleted after Next is the zero value of the element type
	ints := map[int]int{1: 10}
	it = reflect.ValueOf(ints).MapRange()
	it.Next()
	delete(ints, 1)
	fmt.Println("deleted int:", it.Value().IsValid(), it.Value())
	slices := map[string][]int{"a": {1, 2}}
	it = reflect.ValueOf(slices).MapRange()
	it.Next()
	delete(slices, "a")
	fmt.Println("deleted slice:", it.Value().IsValid(), it.Value())
	arrays := map[int][3]int{1: {1, 2, 3}}
	it = reflect.ValueOf(arrays).MapRange()
	it.Next()
	delete(arrays, 1)
	fmt.Println("deleted array:", it.Value().IsValid(), it.Value())

	// MapKeys is a copy taken at once
	m = map[int]int{1: 1, 2: 2}
	keys := reflect.ValueOf(m).MapKeys()
	clear(m)
	fmt.Println("keys taken before clear:", len(keys))
}

// Output:
// produced after deleting all: 1
// value after set: 99
// some keys were skipped: true
// neighbours produced: 0
// produced after clear: 1
// NaN keys: 3 6
// NaN keys and clear: 1
// deleted int: true 0
// deleted slice: true []
// deleted array: true [0 0 0]
// keys taken before clear: 2
