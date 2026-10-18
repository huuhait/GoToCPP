// phase: 05

// Maps print sorted by key (internal/fmtsort): the slices case, with %v %+v %#v and other verbs
// applied to keys and values.
package main

import "fmt"

type pt struct{ x, y int }

type Color int

const (
	Red Color = iota
	Green
	Blue
)

func (c Color) String() string { return [...]string{"red", "green", "blue"}[c] }

type Level uint8

func main() {
	zero := 0.0
	inf := 1 / zero
	ninf := -1 / zero
	nan := zero / zero
	nzero := -zero
	_, _, _, _ = inf, ninf, nan, nzero
	m := map[string][]int{"b": {1, 2}, "a": nil, "c": {}}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[a:[] b:[1 2] c:[]] 3
// map[a:[] b:[1 2] c:[]]|map[a:[] b:[1 2] c:[]]|map[string][]int{"a":[]int(nil), "b":[]int{1, 2}, "c":[]int{}}|map[string][]int
// map[%!d(string=a):[] %!d(string=b):[1 2] %!d(string=c):[]]|map[a:[] b:[%!s(int=1) %!s(int=2)] c:[]]|map[61:[] 62:[1 2] 63:[]]|map["a":[] "b":['\x01' '\x02'] "c":[]]|map[    a:[]     b:[    1     2]     c:[]]|map[a    :[] b    :[1     2    ] c    :[]]|
// true true
