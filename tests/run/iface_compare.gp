// phase: 05

// == and != between interface values and concrete values (either side), between interfaces of
// different types, with nil, and in switch statements on an interface (step 5.4; the minimal ==
// of step 5.6 that structs with interface fields need).
package main

import "fmt"

type Shape interface{ Area() int }
type Namer interface{ Name() string }
type Sq int

func (s Sq) Area() int    { return int(s * s) }
func (s Sq) Name() string { return "sq" }

type Box struct{ w int }

func (b *Box) Area() int { return b.w }

func main() {
	var s Shape = Sq(3)
	var n Namer = Sq(3)
	var a any = Sq(3)
	fmt.Println(s == Sq(3), Sq(3) == s, s != Sq(4), Sq(4) != s)
	fmt.Println(a == Sq(3), Sq(3) == a, a == 3, 3 == a, a == int64(3))
	fmt.Println(s == a, a == s, n == a, a == n)
	var e1, e2 Shape
	fmt.Println(e1 == nil, nil == e1, e1 == e2, e1 == s, s == e1)
	b1, b2 := &Box{1}, &Box{1}
	var sb1, sb2, sb3 Shape = b1, b2, b1
	fmt.Println(sb1 == sb2, sb1 == sb3, sb1 == b1, b2 == sb2, sb1 == b2)
	var np *Box
	var snp Shape = np
	fmt.Println(snp == nil, snp == np, np == snp)
	switch a {
	case 3:
		fmt.Println("int 3")
	case Sq(3):
		fmt.Println("Sq 3")
	}
	switch x := any("go"); x {
	case "rust", 1:
		fmt.Println("no")
	case "go":
		fmt.Println("go")
	}
	switch s {
	case nil:
		fmt.Println("nil")
	case Sq(2), Sq(3):
		fmt.Println("2 or 3")
	}
	m := map[any]int{Sq(1): 1, 1: 2, "1": 3}
	fmt.Println(m[Sq(1)], m[1], m["1"], m[int8(1)])
}

// Output:
// true true true true
// true true false false false
// true true true true
// true true true false false
// false true true true false
// false true true
// Sq 3
// go
// 2 or 3
// 1 2 3 0
