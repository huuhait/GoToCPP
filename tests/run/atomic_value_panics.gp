// phase: 08

// atomic.Value insists on one concrete type, and on non-nil values: the panics are Go's.
package main

import (
	"fmt"
	"sync/atomic"
)

func try(name string, f func()) {
	defer func() { fmt.Println(name+":", recover()) }()
	f()
}

func main() {
	var v atomic.Value
	try("store nil", func() { v.Store(nil) })
	try("swap nil", func() { v.Swap(nil) })
	try("cas nil new", func() { v.CompareAndSwap(nil, nil) })
	v.Store(1)
	try("store other type", func() { v.Store("s") })
	try("swap other type", func() { v.Swap(1.5) })
	try("cas other type of new", func() { v.CompareAndSwap(1, "x") })
	try("cas inconsistent old and new", func() { v.CompareAndSwap("a", 2) })
	try("cas same types, ok", func() { fmt.Println(v.CompareAndSwap(1, 2), v.Load()) })
	// int and int64 are different types.
	try("int64 into int", func() { v.Store(int64(3)) })
	var w atomic.Value
	try("store typed nil pointer", func() { var p *int; w.Store(p); fmt.Println(w.Load() == (*int)(nil)) })
	try("store another pointer type", func() { var q *string; w.Store(q) })
	var x atomic.Value
	x.Store([]int{1})
	try("cas of uncomparable values", func() { x.CompareAndSwap([]int{1}, []int{2}) })
	fmt.Println(x.Load())
}

// Output:
// store nil: sync/atomic: store of nil value into Value
// swap nil: sync/atomic: swap of nil value into Value
// cas nil new: sync/atomic: compare and swap of nil value into Value
// store other type: sync/atomic: store of inconsistently typed value into Value
// swap other type: sync/atomic: swap of inconsistently typed value into Value
// cas other type of new: sync/atomic: compare and swap of inconsistently typed values
// cas inconsistent old and new: sync/atomic: compare and swap of inconsistently typed values
// true 2
// cas same types, ok: <nil>
// int64 into int: sync/atomic: store of inconsistently typed value into Value
// true
// store typed nil pointer: <nil>
// store another pointer type: sync/atomic: store of inconsistently typed value into Value
// cas of uncomparable values: runtime error: comparing uncomparable type []int
// [1]
