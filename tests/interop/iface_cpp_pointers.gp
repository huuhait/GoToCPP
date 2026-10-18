// phase: 05

// Pointers to C/C++ values in interfaces (step 5.4): &cvar, a pointer variable and the address of
// an element are all the dynamic type *C.T (a gp::cptr), whichever way they were made; a Go struct
// that wraps a C/C++ value or handle (the typical interop pattern) implements interfaces, in
// an interface, a type switch and an assertion; a C++ value with == and no std::hash is a map key;
// a map key that can't be hashed panics as Go's map does.
package main

import (
	"fmt"

	"./ifacehdr.hpp"
	"<stdio.h>"
)

type Num interface{ Num() int }
type Incer interface{ Inc() int }
type Summer interface{ Sum() int }

// File wraps a C handle and a C integer.
type File struct {
	n C.int
	f *stdio.FILE
}

func (f *File) Num() int { return int(f.n) }

// Wrap holds C++ values, with methods of its own (one with a C type in its signature, which no
// interface can have).
type Wrap struct {
	c ifh.Counter
	p ifh.Pt
}

func (w *Wrap) Inc() int          { return int(w.c.inc()) }
func (w Wrap) Sum() int           { return int(w.p.x + w.p.y) }
func (w *Wrap) Raw(x C.int) C.int { return x + 1 }

func isCInt(x any) bool {
	_, ok := x.(*C.int)
	return ok
}

func main() {
	p := ifh.make_pt(1, 2)
	var a any = &p
	q, ok := a.(*ifh.Pt)
	fmt.Println(ok)
	if ok {
		q.x = 50
	}
	fmt.Println(p.x)
	n := ifh.make(1)
	var np any = &n
	switch v := np.(type) {
	case *ifh.Counter:
		v.inc()
		fmt.Println("counter", n.get())
	}
	var cn C.int = 4
	pp := &cn
	var b any = pp
	d := any(&cn)
	var arr [2]C.int
	var e any = &arr[1]
	fmt.Println(isCInt(b), isCInt(d), isCInt(e), isCInt(&arr[0]), isCInt(&cn), isCInt(nil), isCInt(&n))
	_, isI32 := b.(*int32)
	fmt.Println(isI32)
	c, okC := d.(*C.int)
	fmt.Println(okC, int(*c))

	var x Num = &File{n: 3}
	fmt.Println(x.Num())
	w := &Wrap{p: ifh.make_pt(3, 4)}
	var i Incer = w
	i.Inc()
	fmt.Println(i.Inc())
	var any1 any = w
	if s, ok := any1.(Summer); ok {
		fmt.Println(s.Sum())
	}
	_, isInc := any1.(Incer)
	_, isNum := any1.(Num)
	fmt.Println(isInc, isNum)
	var v any = *w
	_, isSummer := v.(Summer)
	_, isIncer := v.(Incer)
	fmt.Println(isSummer, isIncer)
	f1, f2 := &File{n: 1}, &File{n: 1}
	var n1, n2 Num = f1, f2
	fmt.Println(n1 == n2, n1 == Num(f1))

	k1, k2 := any(ifh.make(1)), any(ifh.make(1))
	m := map[any]int{k1: 10, any(ifh.make(2)): 20}
	fmt.Println(k1 == k2, m[k2], m[any(ifh.make(2))], m[any(ifh.make(3))], len(m))
}

// Output:
// true
// 50
// counter 2
// true true true true true false false
// false
// true 4
// 3
// 2
// 7
// true false
// true false
// false true
// true 10 20 0 2
