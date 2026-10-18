// phase: 05

// Maps print sorted by key (internal/fmtsort): the ifacestr case, with %v %+v %#v and other verbs
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
	m := map[any]int{"b": 1, "a": 2, "c": 3}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[a:2 b:1 c:3] 3
// map[a:2 b:1 c:3]|map[a:2 b:1 c:3]|map[interface {}]int{"a":2, "b":1, "c":3}|map[interface {}]int
// map[%!d(string=a):2 %!d(string=b):1 %!d(string=c):3]|map[a:%!s(int=2) b:%!s(int=1) c:%!s(int=3)]|map[61:2 62:1 63:3]|map["a":'\x02' "b":'\x01' "c":'\x03']|map[    a:    2     b:    1     c:    3]|map[a    :2     b    :1     c    :3    ]|
// true true
