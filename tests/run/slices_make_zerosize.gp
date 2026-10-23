// phase: 09

// make of zero-size elements (struct{}, [0]int, a struct of them) takes no memory, as in Go, so
// any length up to math.MaxInt works; the checks and messages of make are Go's: makeslice
// "len out of range" or "cap out of range" with Go's size of the element, and growslice when an
// append goes past math.MaxInt.
package main

import (
	"fmt"
	"math"
)

type Unit struct{}

func try(name string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(name+":", r)
		}
	}()
	f()
}

func main() {
	s := make([]struct{}, math.MaxInt)
	fmt.Println(len(s), cap(s))
	s[math.MaxInt-1] = struct{}{}
	_ = s[len(s)-1]
	t := make([]Unit, 3, math.MaxInt)
	fmt.Println(len(t), cap(t), len(t[:math.MaxInt]))
	z := make([][0]int, math.MaxInt)
	fmt.Println(len(z), cap(z))
	var big uint64 = math.MaxInt
	fmt.Println(len(make([]struct{}, big)))
	e := make([]struct{}, 0)
	fmt.Println(e != nil, len(e))
	g := make([]struct{ a, b struct{} }, 3)
	g[2] = struct{ a, b struct{} }{}
	fmt.Println(len(g), cap(g))

	// Conversions to arrays copy no data (the slice's data is Go's zerobase, whatever its length).
	s100 := make([]struct{}, 100)
	a100 := [100]struct{}(s100)
	bu := make([]Unit, 1<<18)
	abig := [1 << 18]Unit(bu)
	pab := &abig
	p100 := (*[100]struct{})(s100)
	fmt.Println(len(a100), a100 == [100]struct{}{}, len(abig), abig[1<<18-1] == Unit{}, len(p100), &p100[3] == &s100[3], len(pab))

	n := -1
	try("make len -1", func() { _ = make([]struct{}, n) })
	try("make cap -1", func() { _ = make([]struct{}, 0, n) })
	m := 2
	try("make len > cap", func() { _ = make([]struct{}, m, 1) })
	big++
	try("make len MaxInt+1", func() { _ = make([]struct{}, big) })
	try("make cap MaxInt+1", func() { _ = make([]struct{}, 1, big) })
	try("append past MaxInt", func() { _ = append(s, struct{}{}) })
	try("append slice past MaxInt", func() { _ = append(s, s...) })
	huge := math.MaxInt / 4
	try("make []int MaxInt/4", func() { _ = make([]int, huge) })
	try("make []int cap MaxInt/4", func() { _ = make([]int, 1, huge) })
	k := 1<<44 + 1 // 16 bytes a Go string: past the 1<<48 bytes Go can allocate
	try("make []string 1<<44+1", func() { _ = make([]string, k) })
	try("make []string cap 1<<44+1", func() { _ = make([]string, 0, k) })
}

// Output:
// 9223372036854775807 9223372036854775807
// 3 9223372036854775807 9223372036854775807
// 9223372036854775807 9223372036854775807
// 9223372036854775807
// true 0
// 3 3
// 100 true 262144 true 100 true 262144
// make len -1: runtime error: makeslice: len out of range
// make cap -1: runtime error: makeslice: cap out of range
// make len > cap: runtime error: makeslice: cap out of range
// make len MaxInt+1: runtime error: makeslice: len out of range
// make cap MaxInt+1: runtime error: makeslice: cap out of range
// append past MaxInt: runtime error: growslice: len out of range
// append slice past MaxInt: runtime error: growslice: len out of range
// make []int MaxInt/4: runtime error: makeslice: len out of range
// make []int cap MaxInt/4: runtime error: makeslice: cap out of range
// make []string 1<<44+1: runtime error: makeslice: len out of range
// make []string cap 1<<44+1: runtime error: makeslice: cap out of range
