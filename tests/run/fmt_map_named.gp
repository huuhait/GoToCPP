// phase: 05

// Maps print sorted by key (internal/fmtsort): the named case, with %v %+v %#v and other verbs
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
	m := map[Color]Level{Blue: 1, Red: 2, Green: 3}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[red:2 green:3 blue:1] 3
// map[red:2 green:3 blue:1]|map[red:2 green:3 blue:1]|map[main.Color]main.Level{0:0x2, 1:0x3, 2:0x1}|map[main.Color]main.Level
// map[0:2 1:3 2:1]|map[red:%!s(main.Level=2) green:%!s(main.Level=3) blue:%!s(main.Level=1)]|map[726564:2 677265656e:3 626c7565:1]|map["red":'\x02' "green":'\x03' "blue":'\x01']|map[  red:    2 green:    3  blue:    1]|map[red  :2     green:3     blue :1    ]|
// true true
