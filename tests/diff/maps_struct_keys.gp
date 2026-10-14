// phase: 03

// m[k] = append(m[k], f()) where f changes the struct variable k: gc reads k before f runs when
// its fast map calls take the key (walk/order.go mapKeyTemp): a struct of one field (not _) that
// they take (an int, a string, a pointer), or 4 or 8 bytes of plain memory without blank fields
// or padding; and after f for the others (16 bytes, padding, a blank field, a float). Each line
// prints which key got the value.
package main

import "fmt"

type (
	two32  struct{ a, b int32 }
	two16  struct{ a, b int16 }
	oneInt struct{ v int }
	oneU64 struct{ v uint64 }
	oneStr struct{ s string }
	onePtr struct{ p *int }
	padded struct {
		a int8
		b int32
	}
	blank struct {
		a int32
		_ int32
	}
	wide   struct{ a, b int }
	oneF   struct{ f float64 }
	nested struct{ in two32 }
)

var (
	x, y int
	k1   = two32{1, 1}
	k2   = two16{1, 1}
	k3   = oneInt{1}
	k4   = oneU64{1}
	k5   = oneStr{"one"}
	k6   = onePtr{&x}
	k7   = padded{1, 1}
	k8   = blank{a: 1}
	k9   = wide{1, 1}
	k10  = oneF{1}
	k11  = nested{two32{1, 1}}
)

func change() int {
	k1, k2, k3, k4, k5, k6 = two32{5, 5}, two16{5, 5}, oneInt{5}, oneU64{5}, oneStr{"five"}, onePtr{&y}
	k7, k8, k9, k10, k11 = padded{5, 5}, blank{a: 5}, wide{5, 5}, oneF{5}, nested{two32{5, 5}}
	return 7
}

func main() {
	m1 := map[two32][]int{}
	m1[k1] = append(m1[k1], change())
	fmt.Println("two32", len(m1[two32{1, 1}]), len(m1[two32{5, 5}]))
	m2 := map[two16][]int{}
	k2 = two16{1, 1}
	m2[k2] = append(m2[k2], change())
	fmt.Println("two16", len(m2[two16{1, 1}]), len(m2[two16{5, 5}]))
	m3 := map[oneInt][]int{}
	k3 = oneInt{1}
	m3[k3] = append(m3[k3], change())
	fmt.Println("oneInt", len(m3[oneInt{1}]), len(m3[oneInt{5}]))
	m4 := map[oneU64][]int{}
	k4 = oneU64{1}
	m4[k4] = append(m4[k4], change())
	fmt.Println("oneU64", len(m4[oneU64{1}]), len(m4[oneU64{5}]))
	m5 := map[oneStr][]int{}
	k5 = oneStr{"one"}
	m5[k5] = append(m5[k5], change())
	fmt.Println("oneStr", len(m5[oneStr{"one"}]), len(m5[oneStr{"five"}]))
	m6 := map[onePtr][]int{}
	k6 = onePtr{&x}
	m6[k6] = append(m6[k6], change())
	fmt.Println("onePtr", len(m6[onePtr{&x}]), len(m6[onePtr{&y}]))
	m7 := map[padded][]int{}
	k7 = padded{1, 1}
	m7[k7] = append(m7[k7], change())
	fmt.Println("padded", len(m7[padded{1, 1}]), len(m7[padded{5, 5}]))
	m8 := map[blank][]int{}
	k8 = blank{a: 1}
	m8[k8] = append(m8[k8], change())
	fmt.Println("blank", len(m8[blank{a: 1}]), len(m8[blank{a: 5}]))
	m9 := map[wide][]int{}
	k9 = wide{1, 1}
	m9[k9] = append(m9[k9], change())
	fmt.Println("wide", len(m9[wide{1, 1}]), len(m9[wide{5, 5}]))
	m10 := map[oneF][]int{}
	k10 = oneF{1}
	m10[k10] = append(m10[k10], change())
	fmt.Println("oneF", len(m10[oneF{1}]), len(m10[oneF{5}]))
	m11 := map[nested][]int{}
	k11 = nested{two32{1, 1}}
	m11[k11] = append(m11[k11], change())
	fmt.Println("nested", len(m11[nested{two32{1, 1}}]), len(m11[nested{two32{5, 5}}]))
}
