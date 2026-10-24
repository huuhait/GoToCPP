// phase: 05

// A struct with C/C++ fields that Go cannot compare apart from them (a slice, map or func field, also
// inside a nested struct or an array) has no == and no hash (lower/typedecl.go: cKeyMembers): fmt prints
// it, an interface holds it, and any == any and a map[any]T key of it panic as in Go. The comparable
// shapes next to them (nested, embedded, arrays of them) compare.
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

type NV struct {
	v  std.vector[int]
	xs []int
}

type NM struct {
	name std.string
	m    map[string]int
}

type NF struct {
	name std.string
	f    func() int
}

type Inner struct {
	name std.string
	n    int
}

type Nested struct {
	in Inner
	b  bool
}

type Embedded struct {
	Inner
	k int
}

type DeepBad struct {
	in  Inner
	bad [1]NV
}

func try(what string, f func()) {
	defer func() { fmt.Println(what+":", recover()) }()
	f()
}

func main() {
	fmt.Println(NM{name: std.string("a"), m: map[string]int{"k": 1}})
	var p, q any = NV{}, NV{}
	fmt.Println(p != nil, q != nil)
	try("NV ==", func() { fmt.Println(p == q) })
	try("NF ==", func() {
		var a, b any = NF{}, NF{}
		fmt.Println(a == b)
	})
	try("NM key", func() {
		m := map[any]int{}
		m[NM{}] = 1
	})
	try("[2]NV ==", func() {
		var a, b any = [2]NV{}, [2]NV{}
		fmt.Println(a == b)
	})
	try("DeepBad ==", func() {
		var a, b any = DeepBad{}, DeepBad{}
		fmt.Println(a == b)
	})

	n1, n2 := Nested{in: Inner{name: std.string("x"), n: 1}}, Nested{in: Inner{name: std.string("x"), n: 1}}
	e1, e2 := Embedded{Inner: Inner{name: std.string("y")}, k: 2}, Embedded{Inner: Inner{name: std.string("y")}, k: 2}
	fmt.Println(n1 == n2, e1 == e2, e1.name.size(), e1 != Embedded{})
	var a1, a2 any = [2]Inner{{n: 1}, {n: 2}}, [2]Inner{{n: 1}, {n: 2}}
	var b1, b2 any = n1, Nested{}
	fmt.Println(a1 == a2, b1 == b2, b1 == any(n2))
	keys := map[any]int{n1: 1, e1: 2, [2]Inner{}: 3}
	keys[n2]++
	fmt.Println(len(keys), keys[n1], keys[Embedded{Inner: Inner{name: std.string("y")}, k: 2}], keys[[2]Inner{}])
}

// Output:
// {a map[k:1]}
// true true
// NV ==: runtime error: comparing uncomparable type main.NV
// NF ==: runtime error: comparing uncomparable type main.NF
// NM key: runtime error: hash of unhashable type main.NM
// [2]NV ==: runtime error: comparing uncomparable type [2]main.NV
// DeepBad ==: runtime error: comparing uncomparable type main.DeepBad
// true true 1 true
// true false true
// 3 2 2 3
