// phase: 05

// Maps print sorted by key (internal/fmtsort): the runes case, with %v %+v %#v and other verbs
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
	m := map[rune]int{'b': 1, 'a': 2, 'é': 3, 'z': 4}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[97:2 98:1 122:4 233:3] 4
// map[97:2 98:1 122:4 233:3]|map[97:2 98:1 122:4 233:3]|map[int32]int{97:2, 98:1, 122:4, 233:3}|map[int32]int
// map[97:2 98:1 122:4 233:3]|map[%!s(int32=97):%!s(int=2) %!s(int32=98):%!s(int=1) %!s(int32=122):%!s(int=4) %!s(int32=233):%!s(int=3)]|map[61:2 62:1 7a:4 e9:3]|map['a':'\x02' 'b':'\x01' 'z':'\x04' 'é':'\x03']|map[   97:    2    98:    1   122:    4   233:    3]|map[97   :2     98   :1     122  :4     233  :3    ]|
// true true
