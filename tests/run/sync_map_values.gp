// phase: 08

// Values are `any`: nil and zero values are values, and the old/new of CompareAndSwap compare
// dynamic types too.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var m sync.Map
	m.Store("nil", nil)
	v, ok := m.Load("nil")
	fmt.Println(v, ok, v == nil)
	_, ok = m.Load("absent")
	fmt.Println(ok)
	act, loaded := m.LoadOrStore("nil", 1)
	fmt.Println(act, loaded)
	fmt.Println(m.CompareAndSwap("nil", nil, 5))
	fmt.Println(m.CompareAndSwap("nil", 5, int64(6)))
	fmt.Println(m.CompareAndSwap("nil", 5, 7))
	v, _ = m.Load("nil")
	fmt.Println(v)
	fmt.Println(m.CompareAndSwap("nil", int64(6), 8), m.CompareAndSwap("nil", 6, 8))
	m.Store("zero", 0)
	m.Store("empty", "")
	m.Store("false", false)
	for _, k := range []string{"zero", "empty", "false"} {
		v, ok := m.Load(k)
		fmt.Printf("%s %v %v %T\n", k, v, ok, v)
	}
	type point struct{ x, y int }
	m.Store(point{1, 2}, &point{3, 4})
	pv, _ := m.Load(point{1, 2})
	fmt.Println(*pv.(*point))
}

// Output:
// <nil> true true
// false
// <nil> true
// true
// true
// false
// 6
// true false
// zero 0 true int
// empty  true string
// false false true bool
// {3 4}
