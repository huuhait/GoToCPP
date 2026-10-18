// phase: 05

// Maps print sorted by key (internal/fmtsort): the uints case, with %v %+v %#v and other verbs
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
	m := map[uint8]int{255: 1, 0: 2, 128: 3, 7: 4}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[0:2 7:4 128:3 255:1] 4
// map[0:2 7:4 128:3 255:1]|map[0:2 7:4 128:3 255:1]|map[uint8]int{0x0:2, 0x7:4, 0x80:3, 0xff:1}|map[uint8]int
// map[0:2 7:4 128:3 255:1]|map[%!s(uint8=0):%!s(int=2) %!s(uint8=7):%!s(int=4) %!s(uint8=128):%!s(int=3) %!s(uint8=255):%!s(int=1)]|map[0:2 7:4 80:3 ff:1]|map['\x00':'\x02' '\a':'\x04' '\u0080':'\x03' 'ÿ':'\x01']|map[    0:    2     7:    4   128:    3   255:    1]|map[0    :2     7    :4     128  :3     255  :1    ]|
// true true
