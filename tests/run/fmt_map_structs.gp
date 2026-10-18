// phase: 05

// Maps print sorted by key (internal/fmtsort): the structs case, with %v %+v %#v and other verbs
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
	m := map[pt]int{{2, 1}: 1, {1, 2}: 2, {1, 1}: 3, {0, 5}: 4}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[{0 5}:4 {1 1}:3 {1 2}:2 {2 1}:1] 4
// map[{0 5}:4 {1 1}:3 {1 2}:2 {2 1}:1]|map[{x:0 y:5}:4 {x:1 y:1}:3 {x:1 y:2}:2 {x:2 y:1}:1]|map[main.pt]int{main.pt{x:0, y:5}:4, main.pt{x:1, y:1}:3, main.pt{x:1, y:2}:2, main.pt{x:2, y:1}:1}|map[main.pt]int
// map[{0 5}:4 {1 1}:3 {1 2}:2 {2 1}:1]|map[{%!s(int=0) %!s(int=5)}:%!s(int=4) {%!s(int=1) %!s(int=1)}:%!s(int=3) {%!s(int=1) %!s(int=2)}:%!s(int=2) {%!s(int=2) %!s(int=1)}:%!s(int=1)]|map[{0 5}:4 {1 1}:3 {1 2}:2 {2 1}:1]|map[{'\x00' '\x05'}:'\x04' {'\x01' '\x01'}:'\x03' {'\x01' '\x02'}:'\x02' {'\x02' '\x01'}:'\x01']|map[{    0     5}:    4 {    1     1}:    3 {    1     2}:    2 {    2     1}:    1]|map[{0     5    }:4     {1     1    }:3     {1     2    }:2     {2     1    }:1    ]|
// true true
