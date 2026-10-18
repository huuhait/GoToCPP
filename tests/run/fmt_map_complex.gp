// phase: 05

// Maps print sorted by key (internal/fmtsort): the complex case, with %v %+v %#v and other verbs
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
	m := map[complex128]int{complex(1, 2): 1, complex(1, 1): 2, complex(0, 5): 3, complex(-1, 0): 4}
	fmt.Println(m, len(m))
	fmt.Printf("%v|%+v|%#v|%T\n", m, m, m, m)
	fmt.Printf("%d|%s|%x|%q|%5v|%-5v|\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%v", m))
}

// Output:
// map[(-1+0i):4 (0+5i):3 (1+1i):2 (1+2i):1] 4
// map[(-1+0i):4 (0+5i):3 (1+1i):2 (1+2i):1]|map[(-1+0i):4 (0+5i):3 (1+1i):2 (1+2i):1]|map[complex128]int{(-1+0i):4, (0+5i):3, (1+1i):2, (1+2i):1}|map[complex128]int
// map[%!d(complex128=(-1+0i)):4 %!d(complex128=(0+5i)):3 %!d(complex128=(1+1i)):2 %!d(complex128=(1+2i)):1]|map[%!s(complex128=(-1+0i)):%!s(int=4) %!s(complex128=(0+5i)):%!s(int=3) %!s(complex128=(1+1i)):%!s(int=2) %!s(complex128=(1+2i)):%!s(int=1)]|map[(-0x1p+00+0x0p+00i):4 (0x0p+00+0x1.4p+02i):3 (0x1p+00+0x1p+00i):2 (0x1p+00+0x1p+01i):1]|map[%!q(complex128=(-1+0i)):'\x04' %!q(complex128=(0+5i)):'\x03' %!q(complex128=(1+1i)):'\x02' %!q(complex128=(1+2i)):'\x01']|map[(   -1   +0i):    4 (    0   +5i):    3 (    1   +1i):    2 (    1   +2i):    1]|map[(-1   +0   i):4     (0    +5   i):3     (1    +1   i):2     (1    +2   i):1    ]|
// true true
