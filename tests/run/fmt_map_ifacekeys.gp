// phase: 05

// Maps print sorted by key (internal/fmtsort): the ifacekeys case, with %v %+v %#v and other
// verbs applied to keys and values.
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
	m := map[any]int{3: 1, 1: 2, 2: 3}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[1:2 2:3 3:1] 3
// map[1:2 2:3 3:1]|map[1:2 2:3 3:1]|map[interface {}]int{1:2, 2:3, 3:1}|map[interface {}]int
// map[1:2 2:3 3:1]|map[%!s(int=1):%!s(int=2) %!s(int=2):%!s(int=3) %!s(int=3):%!s(int=1)]|map[1:2 2:3 3:1]|map['\x01':'\x02' '\x02':'\x03' '\x03':'\x01']|map[    1:    2     2:    3     3:    1]|map[1    :2     2    :3     3    :1    ]|
// true true
