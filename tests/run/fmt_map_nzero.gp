// phase: 05

// Maps print sorted by key (internal/fmtsort): the nzero case, with %v %+v %#v and other verbs
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
	m := map[float64]int{nzero: 1}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[-0:1] 1
// map[-0:1]|map[-0:1]|map[float64]int{-0:1}|map[float64]int
// map[%!d(float64=-0):1]|map[%!s(float64=-0):%!s(int=1)]|map[-0x0p+00:1]|map[%!q(float64=-0):'\x01']|map[   -0:    1]|map[-0   :1    ]|
// true true
