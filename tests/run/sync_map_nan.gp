// phase: 08

// A NaN key is never equal to itself: every Store adds an entry, Load and Delete cannot find it,
// and Range still shows all of them.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var m sync.Map
	nan := 0.0
	nan = nan / nan
	m.Store(nil, "nil key")
	m.Store(nan, 1)
	m.Store(nan, 2)
	m.Store(nan, 3)
	_, ok := m.Load(nan)
	fmt.Println("load nan:", ok)
	m.Delete(nan)
	n, sum := 0, 0
	m.Range(func(k, v any) bool {
		n++
		if i, ok := v.(int); ok {
			sum += i
		}
		return true
	})
	fmt.Println(n, sum)
}

// Output:
// load nan: false
// 4 6
