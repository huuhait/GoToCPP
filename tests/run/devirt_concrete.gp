// phase: 08

// Concrete dispatch still calls the interface's thunk on its owning snapshot. Promoted,
// reassigned, aliased, generic and deferred receivers keep dynamic dispatch.
package main

import (
	"fmt"
	"runtime"
)

type Value struct {
	n int
}

func (v Value) Read(x int) int {
	v.n += x
	return v.n
}

func (v *Value) Add(x int) int {
	if v == nil {
		return -1
	}
	v.n += x
	return v.n
}

func (v *Value) Keep() *Value {
	owner = nil
	return v
}

type Other struct{}

func (Other) Read(x int) int { return -x }

type Reader interface{ Read(int) int }
type Adder interface{ Add(int) int }
type Keeper interface{ Keep() *Value }
type Promoted struct{ Value }
type Embedded interface{ Read(int) int }
type Wrapper struct{ Embedded }

type Generic[T any] struct{ n int }

func (v Generic[T]) Read(x int) int { return v.n + x }

func generic[T Reader](v T) int { return Reader(v).Read(2) }

var owner *Value
var trace string

func receiver() *Value {
	trace += "r"
	return owner
}

func argument() int {
	trace += "a"
	owner = &Value{100}
	return 3
}

func direct() {
	v := Value{4}
	var r Reader = v
	v.n = 100
	fmt.Println("copy", r.Read(2), r.Read(2), v.n)

	p := &Value{5}
	var a Adder = p
	p.n++
	fmt.Println("pointer", a.Add(2), p.n)
	p = nil
	fmt.Println("held", a.Add(3))

	owner = &Value{9}
	var k Keeper = owner
	saved := k.Keep()
	fmt.Println("owning", saved.n, owner == nil)
}

func order() {
	owner = &Value{7}
	trace = ""
	fmt.Println("calls", Adder(receiver()).Add(argument()), trace, owner.n)

	owner = &Value{7}
	trace = ""
	fmt.Println("read", Adder(owner).Add(argument()), trace, owner.n)
}

func replace(p *Reader) { *p = Other{} }

func fallback() {
	var a Reader = Value{4}
	a = Other{}
	fmt.Println("reassigned", a.Read(2))

	var b Reader = Value{4}
	replace(&b)
	fmt.Println("alias", b.Read(2))

	var c Reader = Value{4}
	func() { c = Other{} }()
	fmt.Println("closure", c.Read(2))

	var promoted Reader = Promoted{Value{8}}
	fmt.Println("promoted", promoted.Read(2))
	wrapped := Wrapper{Other{}}
	fmt.Println("embedded", wrapped.Read(2))
	fmt.Println("generic", generic(Generic[int]{8}))
}

func nilPointer() {
	var p *Value
	var a Adder = p
	fmt.Println("typed nil", a == nil, a.Add(2))
}

func nilInterface() int {
	var r Reader
	return r.Read(2)
}

func nilValueMethod() int {
	return Reader((*Value)(nil)).Read(2)
}

func nilPromotedMethod() int {
	return Reader((*Promoted)(nil)).Read(2)
}

func try(name string, f func() int) {
	defer func() {
		if e, ok := recover().(runtime.Error); ok {
			fmt.Println(name, "runtime error", e != nil)
		}
	}()
	fmt.Println(name, f())
}

func later() {
	var r Reader = Value{5}
	defer func(x int) { fmt.Println("defer", x) }(r.Read(2))
	defer r.Read(3)
	r = Other{}
	fmt.Println("later", r.Read(2))
}

func spawned() {
	done := make(chan int)
	var a Adder = &Value{6}
	go func() { done <- a.Add(2) }()
	fmt.Println("spawn", <-done)
}

type Recoverer struct{}
type Recovery interface{ Recover() }

func (Recoverer) Recover() {
	fmt.Println("recover", recover() != nil)
}

func deferredRecovery() {
	var r Recovery = Recoverer{}
	defer r.Recover()
	panic("expected")
}

func indirectRecovery() {
	defer func() {
		var r Recovery = Recoverer{}
		r.Recover()
		fmt.Println("outer recover", recover() != nil)
	}()
	panic("expected")
}

func main() {
	direct()
	order()
	fallback()
	nilPointer()
	try("nil interface", nilInterface)
	try("nil value", nilValueMethod)
	try("nil promoted", nilPromotedMethod)
	later()
	spawned()
	deferredRecovery()
	indirectRecovery()
}

// Output:
// copy 6 6 100
// pointer 8 8
// held 11
// owning 9 true
// calls 10 ra 100
// read 103 a 103
// reassigned -2
// alias -2
// closure -2
// promoted 10
// embedded -2
// generic 10
// typed nil false -1
// nil interface runtime error true
// nil value runtime error true
// nil promoted runtime error true
// later -2
// defer 7
// spawn 8
// recover true
// recover false
// outer recover true
