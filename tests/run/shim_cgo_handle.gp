// phase: 09

// runtime/cgo.Handle: NewHandle counts up from 1, Value gives back the value that was stored (any
// type, a pointer shares its target), Delete removes it, a deleted or never-issued handle panics
// ("runtime/cgo: misuse of an invalid Handle"), and the zero Handle is never valid.
package main

import (
	"fmt"
	"runtime/cgo"
)

type point struct{ x, y int }

func try(f func()) {
	defer func() { fmt.Println("recovered:", recover()) }()
	f()
}

func main() {
	h1 := cgo.NewHandle("hello")
	h2 := cgo.NewHandle(42)
	p := &point{1, 2}
	h3 := cgo.NewHandle(p)
	h4 := cgo.NewHandle(nil)
	fmt.Println(h1 != 0, h2 != 0, h2 == h1+1, h3 == h2+1, h4 == h3+1)
	fmt.Println(h1.Value().(string), h2.Value().(int))
	p.x = 10
	fmt.Println(h3.Value().(*point).x, h3.Value().(*point) == p)
	fmt.Println(h4.Value() == nil)

	// The handle goes through an integer type, as it does through C.
	u := uintptr(h2)
	fmt.Println(cgo.Handle(u).Value())

	h1.Delete()
	try(func() { h1.Value() })
	try(func() { h1.Delete() })
	try(func() { cgo.Handle(0).Value() })
	try(func() { cgo.Handle(1 << 40).Delete() })
	fmt.Println(h2.Value())

	// A handle is not reused after Delete.
	h5 := cgo.NewHandle("again")
	fmt.Println(h5 == h4+1, h5.Value())

	// Many goroutines, each with its own handles.
	done := make(chan bool)
	results := make([]int, 8)
	for g := 0; g < 8; g++ {
		go func() {
			hs := make([]cgo.Handle, 100)
			for i := range hs {
				hs[i] = cgo.NewHandle(g*1000 + i)
			}
			sum := 0
			for i, h := range hs {
				sum += h.Value().(int) - g*1000 - i
				h.Delete()
			}
			results[g] = sum
			done <- true
		}()
	}
	for g := 0; g < 8; g++ {
		<-done
	}
	fmt.Println(results)
	h2.Delete()
	h3.Delete()
	h4.Delete()
	h5.Delete()
}

// Output:
// true true true true true
// hello 42
// 10 true
// true
// 42
// recovered: runtime/cgo: misuse of an invalid Handle
// recovered: runtime/cgo: misuse of an invalid Handle
// recovered: runtime/cgo: misuse of an invalid Handle
// recovered: runtime/cgo: misuse of an invalid Handle
// 42
// true again
// [0 0 0 0 0 0 0 0]
