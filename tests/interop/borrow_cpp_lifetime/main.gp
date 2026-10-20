// phase: 04

// Implicit C++ operations can call back into Go and clear the last source owner.
// Each read must keep its own owner, just as a Go pointer parameter is a copy.
package main

import (
	"./borrow_lifetime.hpp"
	"fmt"
)

type T struct {
	n   int
	pad [8]int
}

type Aggregate struct{ guard lifetime.Default }

func defaultRead(p *T) int {
	var guard lifetime.Default
	_ = guard
	return p.n
}

func destroyRead(p *T) int {
	{
		var guard lifetime.Destroy
		_ = guard
	}
	return p.n
}

func copyRead(p *T) int {
	var guard lifetime.Copy
	copy := guard
	_ = copy
	return p.n
}

func assignRead(p *T) int {
	var first lifetime.Copy
	var second lifetime.Copy
	first = second
	_ = first
	return p.n
}

func aggregateRead(p *T) int {
	var holder Aggregate
	_ = holder
	return p.n
}

func arrayRead(p *T) int {
	var holders [1]Aggregate
	_ = holders
	return p.n
}

func sliceRead(p *T) int {
	holders := make([]Aggregate, 1)
	_ = holders
	return p.n
}

func mapRead(p *T) int {
	holders := make(map[int]Aggregate)
	holders[0] = Aggregate{}
	_ = holders
	return p.n
}

func valueRead(s string) int {
	var guard lifetime.Default
	_ = guard
	return len(s)
}

func resultRead(p *T) (n int, guard lifetime.Default) {
	n = p.n
	return
}

func caseDefault() {
	x := &T{n: 7}
	lifetime.set(func() { x = nil })
	n := defaultRead(x)
	fmt.Println(n, x == nil)
}

func caseDestroy() {
	x := &T{n: 7}
	lifetime.set(func() { x = nil })
	n := destroyRead(x)
	fmt.Println(n, x == nil)
}

func caseCopy() {
	x := &T{n: 7}
	lifetime.set(func() { x = nil })
	n := copyRead(x)
	fmt.Println(n, x == nil)
}

func caseAssign() {
	x := &T{n: 7}
	lifetime.set(func() { x = nil })
	n := assignRead(x)
	fmt.Println(n, x == nil)
}

func caseAggregate() {
	x := &T{n: 7}
	lifetime.set(func() { x = nil })
	n := aggregateRead(x)
	fmt.Println(n, x == nil)
}

func caseArray() {
	x := &T{n: 7}
	lifetime.set(func() { x = nil })
	n := arrayRead(x)
	fmt.Println(n, x == nil)
}

func caseSlice() {
	x := &T{n: 7}
	lifetime.set(func() { x = nil })
	n := sliceRead(x)
	fmt.Println(n, x == nil)
}

func caseMap() {
	x := &T{n: 7}
	lifetime.set(func() { x = nil })
	n := mapRead(x)
	fmt.Println(n, x == nil)
}

func caseValue() {
	s := "before"
	lifetime.set(func() { s = "changed" })
	n := valueRead(s)
	fmt.Println(n, s)
}

func caseResult() {
	x := &T{n: 7}
	lifetime.set(func() { x = nil })
	n, _ := resultRead(x)
	fmt.Println(n, x == nil)
}

func main() {
	caseDefault()
	caseDestroy()
	caseCopy()
	caseAssign()
	caseAggregate()
	caseArray()
	caseSlice()
	caseMap()
	caseValue()
	caseResult()
}

// Output:
// 7 true
// 7 true
// 7 true
// 7 true
// 7 true
// 7 true
// 7 true
// 7 true
// 6 changed
// 7 true
