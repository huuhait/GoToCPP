// phase: 08

// Every method of sync.Map on one goroutine.
package main

import (
	"fmt"
	"sync"
)

func sorted(m *sync.Map) []string {
	var keys []string
	m.Range(func(k, v any) bool {
		keys = append(keys, fmt.Sprint(k, "=", v))
		return true
	})
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func main() {
	var m sync.Map
	v, ok := m.Load("a")
	fmt.Println(v, ok)
	m.Store("a", 1)
	m.Store("b", 2)
	m.Store(3, "three")
	v, ok = m.Load("a")
	fmt.Println(v, ok)
	act, loaded := m.LoadOrStore("a", 100)
	fmt.Println(act, loaded)
	act, loaded = m.LoadOrStore("c", 30)
	fmt.Println(act, loaded)
	prev, loaded := m.Swap("b", 20)
	fmt.Println(prev, loaded)
	prev, loaded = m.Swap("zz", 0)
	fmt.Println(prev, loaded)
	fmt.Println(m.CompareAndSwap("b", 2, 99), m.CompareAndSwap("b", 20, 21), m.CompareAndSwap("nokey", nil, 1))
	fmt.Println(m.CompareAndDelete("c", 31), m.CompareAndDelete("c", 30), m.CompareAndDelete("c", 30))
	v, loaded = m.LoadAndDelete("zz")
	fmt.Println(v, loaded)
	v, loaded = m.LoadAndDelete("zz")
	fmt.Println(v, loaded)
	m.Delete("nokey")
	m.Delete(3)
	fmt.Println(sorted(&m))
	m.Clear()
	fmt.Println(sorted(&m), len(sorted(&m)))
	m.Store(1, 1)
	fmt.Println(sorted(&m))
}

// Output:
// <nil> false
// 1 true
// 1 true
// 30 false
// 2 true
// <nil> false
// false true false
// false true false
// 0 true
// <nil> false
// [a=1 b=21]
// [] 0
// [1=1]
