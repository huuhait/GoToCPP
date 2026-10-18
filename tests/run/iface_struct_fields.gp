// phase: 05

// Structs that hold interface values: copies share the box, == compares dynamically, zero
// values are nil interfaces (step 5.4).
package main

import "fmt"

type Shape interface{ Area() int }
type Sq struct{ s int }
type Re struct{ w, h int }

func (s Sq) Area() int  { return s.s * s.s }
func (r *Re) Area() int { return r.w * r.h }

type Holder struct {
	Name string
	S    Shape
	Any  any
}

func main() {
	var h Holder
	fmt.Println(h.S == nil, h.Any == nil)
	h.S = Sq{3}
	h.Any = 5
	h2 := h
	h2.S = Sq{4}
	fmt.Println(h.S.Area(), h2.S.Area())
	fmt.Println(h == h2, h == Holder{Name: "", S: Sq{3}, Any: 5})
	r := &Re{2, 3}
	a := Holder{"a", r, nil}
	b := Holder{"a", r, nil}
	c := Holder{"a", &Re{2, 3}, nil}
	fmt.Println(a == b, a == c)
	arr := [2]Holder{a, c}
	arr2 := arr
	arr2[1].Name = "z"
	fmt.Println(arr == arr2, arr[0] == arr2[0])
	hs := map[Holder]int{a: 1}
	hs[b]++
	hs[c]++
	fmt.Println(len(hs), hs[a])
	p := &h
	p.S = nil
	fmt.Println(h.S == nil)
}

// Output:
// true true
// 9 16
// false true
// true false
// false true
// 2 2
// true
