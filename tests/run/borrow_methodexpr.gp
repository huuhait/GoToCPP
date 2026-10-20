// phase: 08

// A method expression that is called directly: its arguments are evaluated once and kept alive
// for the call, like a Go call copies them. The method drops the package variable that the caller
// passed (a pointer that the callee only reads, a string that it only reads, a receiver that it
// only reads, an interface value), then uses its parameter (ADR-086).
package main

import "fmt"

type T struct {
	n    int
	name string
}

type Namer interface{ Name() string }

func (t *T) Name() string { return t.name }

func (t *T) isNil() bool           { return t == nil }
func (t *T) nilArg(p *T) bool      { return p == nil }
func (t T) valueNil(p *T) bool     { return p == nil }
func (t *T) nilValue(s []int) bool { return s == nil }

var g *T
var gs = "a"
var gn Namer
var gv T
var gt *T

func (t *T) useP(o *T) int {
	g = nil
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	return t.n + o.n + len(o.name) + len(junk)
}

func (t *T) useS(s string) string {
	gs = "b"
	return s + "!"
}

func (t *T) useN(n Namer) string {
	gn = nil
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	return n.Name() + fmt.Sprint(len(junk))
}

// the receiver is a pointer that the method only reads: the caller's variable is dropped
func (t *T) self() int {
	gt = nil
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	return t.n + len(t.name) + len(junk)
}

// a value receiver that is read and that the method then changes through the package variable
func (v T) val() string {
	gv.name = "changed"
	return v.name
}

type refInner struct{ n int }
type refOuter struct{ *refInner }

var refRoot *refOuter
var refSink *refInner

// Storing the receiver makes it const gp::ptr<refInner>& rather than a borrowed pointer.
// Its embedded source must be snapshotted even while an adapter holds the outer object.
func (p *refInner) take() int {
	refRoot.refInner = nil
	refSink = p
	return p.n
}

func resetRef() { refRoot = &refOuter{&refInner{n: 7}}; refSink = nil }

func promotedRefs() {
	resetRef()
	fmt.Println(refRoot.take())
	resetRef()
	method := refRoot.take
	fmt.Println(method())
	resetRef()
	fmt.Println((*refOuter).take(refRoot))
	resetRef()
	fmt.Println(refOuter.take(*refRoot))
	resetRef()
	var ptrIface interface{ take() int } = refRoot
	fmt.Println(ptrIface.take())
	resetRef()
	var valIface interface{ take() int } = *refRoot
	fmt.Println(valIface.take())
}

func main() {
	g = &T{n: 4, name: "ccc"}
	fmt.Println((*T).useP(&T{n: 1}, g))
	fmt.Println(g == nil)
	fmt.Println((*T).useS(&T{}, gs))
	fmt.Println(gs)
	gn = &T{name: "named"}
	fmt.Println((*T).useN(&T{}, gn))
	fmt.Println(gn == nil)
	gt = &T{n: 3, name: "recv"}
	fmt.Println((*T).self(gt))
	fmt.Println(gt == nil)
	gv = T{name: "orig"}
	fmt.Println(T.val(gv))
	fmt.Println(gv.name)

	// the same through a variable that holds the method expression
	g = &T{n: 5, name: "ccc"}
	f := (*T).useP
	fmt.Println(f(&T{n: 1}, g))
	gs = "a"
	h := (*T).useS
	fmt.Println(h(&T{}, gs))
	fmt.Println(gs)
	promotedRefs()

	// The adapters need the Go pointer type even when C++ would deduce nullptr_t.
	fmt.Println((*T).isNil(nil))
	fmt.Println((*T).nilArg(&T{}, nil))
	fmt.Println(T.valueNil(T{}, nil))
	fmt.Println((*T).nilValue(nil, nil))
	nilMethod := (*T).isNil
	fmt.Println(nilMethod(nil))
}

// Output:
// 18
// true
// a!
// b
// named10
// true
// 17
// true
// orig
// changed
// 19
// a!
// b
// 7
// 7
// 7
// 7
// 7
// 7
// true
// true
// true
// true
// true
