// phase: 05

// Structs that embed interfaces are comparable and can be map keys: == compares the dynamic types
// and values of the interface fields (and the other fields), a nil interface equals a nil one, and a
// struct that embeds a pointer to a struct that embeds an interface compares the pointers.
package main

import "fmt"

type Namer interface{ Name() string }

type Dog struct{ n string }

func (d Dog) Name() string { return d.n }

type Cat struct{ n string }

func (c *Cat) Name() string { return c.n }

type Pet struct {
	Namer
	age int
}

type Owner struct {
	Pet
	city string
}

type Ref struct {
	*Pet
	k int
}

func main() {
	a := Pet{Dog{"rex"}, 3}
	b := Pet{Dog{"rex"}, 3}
	c := Pet{Dog{"max"}, 3}
	cat := &Cat{"tom"}
	d := Pet{cat, 3}
	e := Pet{cat, 3}
	f := Pet{&Cat{"tom"}, 3}
	fmt.Println(a == b, a == c, d == e, d == f, a == d, Pet{} == Pet{}, Pet{}.Namer == nil)
	m := map[Pet]string{a: "a", c: "c", d: "d"}
	m[b] += "+"
	m[e] += "+"
	fmt.Println(len(m), m[a], m[Pet{Dog{"max"}, 3}], m[d], m[f] == "")
	o1 := Owner{a, "x"}
	o2 := Owner{b, "x"}
	fmt.Println(o1 == o2, o1 == Owner{c, "x"}, o1.Name(), Owner{d, "y"}.Name())
	mo := map[Owner]int{o1: 1}
	mo[o2]++
	fmt.Println(mo[o1], len(mo))
	p := &a
	r1, r2, r3 := Ref{p, 1}, Ref{p, 1}, Ref{&b, 1}
	fmt.Println(r1 == r2, r1 == r3, r1.Name(), r3.age)
	var x, y any = a, b
	fmt.Println(x == y, x == any(c))
}

// Output:
// true false true false false true true
// 3 a+ c d+ true
// true false rex tom
// 2 1
// true false rex 3
// true false
