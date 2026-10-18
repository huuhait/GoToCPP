// phase: 05

// Maps print sorted by key (internal/fmtsort): the bools case, with %v %+v %#v and other verbs
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
	m := map[bool]string{true: "yes", false: "no"}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[false:no true:yes] 2
// map[false:no true:yes]|map[false:no true:yes]|map[bool]string{false:"no", true:"yes"}|map[bool]string
// map[%!d(bool=false):%!d(string=no) %!d(bool=true):%!d(string=yes)]|map[%!s(bool=false):no %!s(bool=true):yes]|map[%!x(bool=false):6e6f %!x(bool=true):796573]|map[%!q(bool=false):"no" %!q(bool=true):"yes"]|map[false:   no  true:  yes]|map[false:no    true :yes  ]|
// true true
