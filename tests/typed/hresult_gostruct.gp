// phase: 12
// opaque: same
// windows: yes

// A Go struct that has a C/C++ field is a Go type, so the elided literals of its elements next to a
// C/C++ map key have that type (check types and checks them, as for any Go struct), also as a
// pointer, a slice, an array, a map and inside another such struct.
package main

import (
	"fmt"

	"./hresult.hpp"
)

type Surf struct {
	S hresult.Surface
	N int
}

type Status struct {
	S hresult.NTSTATUS
	N int
}

type Outer struct {
	Inner Surf
	N     int
}

func main() {
	k := hresult.nt_result(1)
	surf := map[any]Surf{k: {N: 1}}
	status := map[any]Status{k: {N: 2}}
	pointer := map[any]*Surf{k: {N: 3}}
	slice := map[any][]Status{k: {{N: 4}, {N: 5}}}
	array := map[any][2]Surf{k: {{N: 6}, {N: 7}}}
	byName := map[any]map[string]Status{k: {"a": {N: 8}}}
	outer := map[any]Outer{k: {Inner: Surf{N: 9}, N: 10}}
	n := 0
	for _, v := range surf {
		n += v.N
	}
	for _, v := range status {
		n += v.N
	}
	for _, v := range pointer {
		n += v.N
	}
	for _, v := range slice {
		n += v[0].N + v[1].N
	}
	for _, v := range array {
		n += v[0].N + v[1].N
	}
	for _, v := range byName {
		n += v["a"].N
	}
	for _, v := range outer {
		n += v.Inner.N + v.N
	}
	fmt.Println(n)
}

// Output:
// 55
