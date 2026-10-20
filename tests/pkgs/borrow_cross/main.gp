// phase: 08

// Pointer parameters and receivers that are borrowed (ADR-086) across packages: the summary of a
// package's functions decides the signature that other packages call, through promoted methods
// of embedded types of another package (by value and through a pointer), interfaces, func values,
// generic instances, goroutines and a callee that clears the variable that holds its argument.
package main

import (
	"fmt"
	"sync"

	"example.com/borrow/inner"
	"example.com/borrow/lib"
)

type Local struct {
	lib.Wrapper
	tag string
}

func main() {
	w := &lib.Wrapper{Name: "w"}
	fmt.Println(w.Inc(), w.Get(), lib.Bump(w, 5), w.Get())
	w.Reset()
	fmt.Println(w.Get(), w.Keep() == &w.Counter)

	pw := lib.PtrWrapper{Counter: &inner.Counter{}}
	pw.Add(3)
	fmt.Println(pw.Inc(), lib.ViaIface(&pw, 10), lib.ViaIface(w, 1), lib.ViaIface(pw.Counter, 2))

	l := Local{tag: "l"}
	l.Inc()
	l.Add(4)
	fmt.Println(l.Get(), lib.Bump(&l.Wrapper, 1), l.Wrapper.Counter.Get())

	ws := make([]lib.Wrapper, 3)
	lib.Each(ws, func(c *inner.Counter) { c.Add(7) })
	lib.Each(ws[1:], func(c *inner.Counter) { c.Inc() })
	fmt.Println(ws[0].Get(), ws[1].Get(), ws[2].Get())

	var st lib.Stack[*inner.Counter]
	fmt.Println(lib.Fill(&st, &ws[0].Counter, &ws[1].Counter), lib.PeekAll([]*inner.Counter{&ws[2].Counter, &ws[1].Counter}))

	peek := inner.Peek
	rem := inner.Remember
	rem(&ws[0].Counter)
	fmt.Println(peek(inner.Last), lib.Both(inner.Last))
	fmt.Println(inner.Drop(inner.Last), inner.Last == nil)

	f := w.Inc
	g := (*inner.Counter).Add
	f()
	g(&w.Counter, 10)
	fmt.Println(w.Get())

	var wg sync.WaitGroup
	cnt := &inner.Counter{}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = inner.Peek(cnt)
		}()
	}
	wg.Wait()
	go inner.Remember(cnt)
	cnt = nil
	fmt.Println(cnt == nil)
}

// Output:
// 1 1 14 7
// 0 true
// 4 15 2 18
// 5 14 7
// 7 8 8
// 2 16
// 7 14
// 107 true
// 13
// true
